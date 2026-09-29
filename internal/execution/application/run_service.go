// Package application 执行域应用服务：测试运行编排（创建/版本校验前置/执行/重跑/暂停）。
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// RunService 运行编排应用服务实现。
type RunService struct {
	repo   domain.RunRepository
	envSvc *EnvService
	cases  domain.CasePort
	runner domain.RunnerPort
	gen    *id.Generator
	log    *slog.Logger
}

// NewRunService 创建运行编排服务。
func NewRunService(repo domain.RunRepository, envSvc *EnvService, cases domain.CasePort, runner domain.RunnerPort, gen *id.Generator) *RunService {
	return &RunService{repo: repo, envSvc: envSvc, cases: cases, runner: runner, gen: gen, log: slog.Default()}
}

// CreateRunRequest 创建运行请求。
type CreateRunRequest struct {
	ScenarioID    int64   `json:"scenario_id"`
	TargetID      int64   `json:"target_id"`
	Env           string  `json:"env"`
	TargetVersion string  `json:"target_version"`
	TargetBranch  string  `json:"target_branch"`
	RunMode       string  `json:"run_mode"`
	SelectedCases []int64 `json:"selected_case_ids"` // 空=全量
}

// CreateRun 创建运行（queued）：勾选空→解析为被测对象全量用例；同时解析当次用例范围。
func (s *RunService) CreateRun(ctx context.Context, req CreateRunRequest) (*domain.Run, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	selected := req.SelectedCases
	if len(selected) == 0 {
		ids, err := s.cases.ListIDsByTarget(ctx, teamID, req.TargetID)
		if err != nil {
			return nil, err
		}
		selected = ids
	}
	mode := req.RunMode
	if mode == "" {
		mode = "manual"
	}
	run := domain.NewRun(s.gen.Next(), teamID, req.ScenarioID, req.TargetID,
		req.Env, req.TargetVersion, req.TargetBranch, mode, selected)
	if err := s.repo.Save(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

// StartVersionCheck 触发版本校验（queued→version_check→scheduled/failed，"测了没白测"前置）。
func (s *RunService) StartVersionCheck(ctx context.Context, runID int64) (*domain.Run, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	run, err := s.repo.Find(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	if err := run.StartVersionCheck(); err != nil {
		return nil, err
	}
	check, err := s.envSvc.CheckVersion(ctx, run.TargetID, run.Env, run.TargetVersion)
	if err != nil {
		s.log.Error("run version check failed", "run_id", runID, "err", err)
		return nil, err
	}
	okRes := check.Result != domain.EnvMismatch // mismatch 阻断；match/unknown 放行
	if err := run.MarkVersionChecked(okRes, check.EnvVersion, &check.ID); err != nil {
		s.log.Error("run mark version checked failed", "run_id", runID, "err", err)
		return nil, err
	}
	if err := s.repo.Save(ctx, run); err != nil {
		s.log.Error("run save failed after version check", "run_id", runID, "err", err)
		return nil, err
	}
	return run, nil
}

// ExecuteRun 执行当次用例（scheduled→running→collect→gate→report→done）；重跑 attempt_seq 递增。
func (s *RunService) ExecuteRun(ctx context.Context, runID int64) (*domain.Run, []*domain.CaseResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, nil, ErrTenantRequired
	}
	run, err := s.repo.Find(ctx, teamID, runID)
	if err != nil {
		return nil, nil, err
	}
	// 首次：scheduled→running；重跑（done/failed 终态）：重置为 running，attempt 递增
	if run.State == domain.RunDone || run.State == domain.RunFailed {
		run.State = domain.RunRunning
	} else {
		if err := run.Schedule(); err != nil {
			return nil, nil, err
		}
	}

	// 装配用例执行规格（勾选范围）
	spec := make([]*domain.CaseSpec, 0, len(run.SelectedCases))
	for _, caseID := range run.SelectedCases {
		version, script, err := s.cases.FetchScript(ctx, teamID, caseID)
		if err != nil {
			return nil, nil, err
		}
		spec = append(spec, &domain.CaseSpec{
			CaseID: caseID, CaseVersion: version, TargetID: run.TargetID, Env: run.Env, Script: script,
		})
	}

	// 执行（P1 MockRunner；K8s Job Runner 接管真实执行）
	results, err := s.runner.Execute(ctx, spec)
	if err != nil {
		return nil, nil, err
	}

	// 落库：同 run 同 case 重跑 attempt_seq 递增
	for _, c := range results {
		attempt, err := s.repo.MaxAttempt(ctx, teamID, runID, c.CaseID)
		if err != nil {
			return nil, nil, err
		}
		now := time.Now().UTC()
		ended := now
		c.TeamID = teamID
		c.RunID = runID
		c.AttemptSeq = attempt + 1
		c.StartedAt = now
		c.EndedAt = &ended
		if err := s.repo.SaveCaseResult(ctx, c); err != nil {
			return nil, nil, err
		}
	}

	// 推进状态机至 done（gate/report 由 GP1-05/07 接入）
	if err := run.StartCollect(); err != nil {
		return nil, nil, err
	}
	if err := run.StartGate(); err != nil {
		return nil, nil, err
	}
	if err := run.StartReport(); err != nil {
		return nil, nil, err
	}
	if err := run.Finish(); err != nil {
		return nil, nil, err
	}
	if err := s.repo.Save(ctx, run); err != nil {
		return nil, nil, err
	}
	return run, results, nil
}

// PauseRun 暂停运行。
func (s *RunService) PauseRun(ctx context.Context, runID int64) (*domain.Run, error) {
	return s.transition(ctx, runID, (*domain.Run).Pause)
}

// ResumeRun 恢复运行。
func (s *RunService) ResumeRun(ctx context.Context, runID int64) (*domain.Run, error) {
	return s.transition(ctx, runID, (*domain.Run).Resume)
}

func (s *RunService) transition(ctx context.Context, runID int64, fn func(*domain.Run) error) (*domain.Run, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	run, err := s.repo.Find(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	if err := fn(run); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

// GetRun 查询运行。
func (s *RunService) GetRun(ctx context.Context, runID int64) (*domain.Run, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.Find(ctx, teamID, runID)
}

// CaseResults 查询运行内用例结果。
func (s *RunService) CaseResults(ctx context.Context, runID int64) ([]*domain.CaseResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.CaseResults(ctx, teamID, runID)
}
