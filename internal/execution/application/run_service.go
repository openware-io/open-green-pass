// Package application 执行域应用服务：测试运行编排（创建/版本校验前置/执行/重跑/暂停）。
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/internal/platform/rls"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// ErrResourceConflict is retained as the application-facing sentinel for
// existing HTTP callers; its identity is the execution-domain error.
var ErrResourceConflict = domain.ErrResourceConflict

// RunService 运行编排应用服务实现。
type RunService struct {
	repo     domain.RunRepository
	envSvc   *EnvService
	cases    domain.CasePort
	policy   domain.PolicyPort
	runner   domain.RunnerPort
	gen      *id.Generator
	log      *slog.Logger
	workflow WorkflowController
	conflict domain.ConflictPort
	quota    domain.QuotaPort
	audit    domain.AuditPort
	gate     domain.GatePort
	report   domain.ReportPort
	cost     domain.CostPort
	events   domain.RunEventBus
	pool     domain.ExecutionPoolPort
	schedule domain.ScheduleQueuePort
}

// NewRunService 创建运行编排服务。
func NewRunService(repo domain.RunRepository, envSvc *EnvService, cases domain.CasePort, policy domain.PolicyPort, runner domain.RunnerPort, gen *id.Generator) *RunService {
	return &RunService{repo: repo, envSvc: envSvc, cases: cases, policy: policy, runner: runner, gen: gen, log: slog.Default(), workflow: NoopWorkflowController{}, conflict: domain.NoopConflictPort{}, quota: domain.NoopQuotaPort{}, audit: domain.NoopAuditPort{}, gate: domain.NoopGatePort{}, report: domain.NoopReportPort{}, cost: domain.NoopCostPort{}}
}

// SetQuotaPort injects the shared scheduling quota implementation.
func (s *RunService) SetQuotaPort(port domain.QuotaPort) {
	if port != nil {
		s.quota = port
	}
}

func (s *RunService) SetWorkflowController(controller WorkflowController) {
	if controller != nil {
		s.workflow = controller
	}
}

// SetConflictPort injects the scheduling conflict implementation. The default
// is a no-op for P1 compatibility; production schedulers should inject a
// shared implementation (the in-process registry is only a reference).
func (s *RunService) SetConflictPort(port domain.ConflictPort) {
	if port != nil {
		s.conflict = port
	}
}

// SetAuditPort injects the trusted-domain audit boundary.
func (s *RunService) SetAuditPort(port domain.AuditPort) {
	if port != nil {
		s.audit = port
	}
}

// SetGatePort wires the trusted quality decision into the Run completion path.
func (s *RunService) SetGatePort(port domain.GatePort) {
	if port != nil {
		s.gate = port
	}
}

// SetReportPort wires immutable report generation into the Run completion path.
func (s *RunService) SetReportPort(port domain.ReportPort) {
	if port != nil {
		s.report = port
	}
}

// SetCostPort wires factual execution measurements into trusted accounting.
func (s *RunService) SetCostPort(port domain.CostPort) {
	if port != nil {
		s.cost = port
	}
}

// SetRunEventBus enables cross-process wakeups for run SSE streams.
func (s *RunService) SetRunEventBus(bus domain.RunEventBus) { s.events = bus }

func (s *RunService) SetExecutionPool(port domain.ExecutionPoolPort) { s.pool = port }

// SetScheduleQueue enables durable asynchronous dispatch for newly created runs.
func (s *RunService) SetScheduleQueue(queue domain.ScheduleQueuePort) { s.schedule = queue }

func (s *RunService) SubscribeRunEvents(ctx context.Context, teamID, runID int64) (domain.RunEventSubscription, error) {
	if s.events == nil {
		return nil, nil
	}
	return s.events.Subscribe(ctx, teamID, runID)
}

func (s *RunService) saveRun(ctx context.Context, run *domain.Run) error {
	if err := s.repo.Save(ctx, run); err != nil {
		return err
	}
	if s.events != nil {
		if err := s.events.Publish(ctx, run.TeamID, run.ID); err != nil {
			s.log.Warn("publish run event failed; polling fallback remains active", "run_id", run.ID, "err", err)
		}
	}
	return nil
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
	if err := s.saveRun(ctx, run); err != nil {
		return nil, err
	}
	if s.schedule != nil {
		if err := EnqueueRun(ctx, s.schedule, run); err != nil {
			return nil, fmt.Errorf("enqueue run %d: %w", run.ID, err)
		}
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
	if err := s.saveRun(ctx, run); err != nil {
		s.log.Error("run save failed after version check", "run_id", runID, "err", err)
		return nil, err
	}
	return run, nil
}

// ExecuteRun 执行当次用例（scheduled→running→collect→gate→report→done）；重跑 attempt_seq 递增。
func (s *RunService) ExecuteRun(ctx context.Context, runID int64) (run *domain.Run, results []*domain.CaseResult, err error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, nil, ErrTenantRequired
	}
	run, err = s.repo.Find(ctx, teamID, runID)
	if err != nil {
		return nil, nil, err
	}
	ownerID, ok := rls.UserFrom(ctx)
	if !ok {
		// Temporal activities created before owner propagation use the immutable
		// run ID as an isolated owner scope. HTTP requests carry the real user ID.
		ownerID = run.ID
	}
	quotaRequest := domain.ResourceRequest{TeamID: teamID, TargetID: run.TargetID, OwnerID: ownerID, Type: "run", Units: 1}
	if err := s.quota.Acquire(ctx, quotaRequest); err != nil {
		return nil, nil, err
	}
	defer func() {
		if releaseErr := s.quota.Release(context.Background(), quotaRequest); err == nil && releaseErr != nil {
			err = releaseErr
		}
	}()
	// GP2-04 reference boundary: the target is exclusive by default. A shared
	// implementation can use a finer resource scope without changing the
	// application service contract.
	claimToken, err := s.conflict.Acquire(ctx, domain.ResourceClaim{
		TeamID: teamID, TargetID: run.TargetID, ResourceType: "target", PoolID: "default", OwnerID: run.ID, Exclusive: true,
	})
	if err != nil {
		if !errors.Is(err, domain.ErrResourceConflict) {
			return nil, nil, err
		}
		if err := s.recordResourceConflict(ctx, run, err); err != nil {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("%w: %v", ErrResourceConflict, err)
	}
	defer func() {
		if claimToken == "" {
			return
		}
		if releaseErr := s.conflict.Release(context.Background(), claimToken); err == nil && releaseErr != nil {
			err = releaseErr
		}
	}()
	// 首次：scheduled→running；重跑（done/failed 终态）：重置为 running，attempt 递增
	if run.State == domain.RunDone || run.State == domain.RunFailed {
		run.State = domain.RunRunning
	} else if run.State == domain.RunPaused {
		if err := run.Resume(); err != nil {
			return nil, nil, err
		}
	} else {
		if err := run.Schedule(); err != nil {
			return nil, nil, err
		}
	}

	// 装配用例执行规格（勾选范围 + 服务级截图开关策略）
	shot, err := s.policy.Get(ctx, teamID, run.TargetID, run.ScenarioID)
	if err != nil {
		return nil, nil, err
	}
	spec := make([]*domain.CaseSpec, 0, len(run.SelectedCases))
	for _, caseID := range run.SelectedCases {
		version, script, err := s.cases.FetchScript(ctx, teamID, caseID)
		if err != nil {
			return nil, nil, err
		}
		spec = append(spec, &domain.CaseSpec{
			CaseID: caseID, CaseVersion: version, TargetID: run.TargetID, Env: run.Env,
			Scenario: scenarioFromScript(script), Script: script, ScreenshotEnabled: shot.ScreenshotEnabled,
		})
	}

	poolLeases, err := s.reserveExecutionPools(ctx, spec)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		for i := len(poolLeases) - 1; i >= 0; i-- {
			if releaseErr := s.pool.Release(context.Background(), poolLeases[i]); err == nil && releaseErr != nil {
				err = releaseErr
			}
		}
	}()

	// 执行（P1 MockRunner；K8s Job Runner 接管真实执行）
	results, err = s.runner.Execute(ctx, spec)
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
		if err := s.cost.Record(ctx, domain.CostRecord{
			RequestID: executionCostRequestID(runID, c.CaseID, c.AttemptSeq),
			TargetID:  run.TargetID,
			RunID:     runID,
			CaseID:    c.CaseID,
			Attempt:   c.AttemptSeq,
			Model:     "k8s-api-runner",
			TokensIn:  c.AITokensIn,
			TokensOut: c.AITokensOut,
		}); err != nil {
			return nil, nil, fmt.Errorf("record execution cost: %w", err)
		}
	}

	// Gate and report are trusted-domain side effects. A non-pass gate is a
	// quality outcome, not an execution infrastructure failure: the Run still
	// reaches done and the generated report carries the authoritative decision.
	if err := run.StartCollect(); err != nil {
		return nil, nil, err
	}
	if err := s.saveRun(ctx, run); err != nil {
		return nil, nil, err
	}
	if err := run.StartGate(); err != nil {
		return nil, nil, err
	}
	if err := s.saveRun(ctx, run); err != nil {
		return nil, nil, err
	}
	if _, err := s.gate.Evaluate(ctx, runID); err != nil {
		return nil, nil, fmt.Errorf("evaluate gate: %w", err)
	}
	if err := run.StartReport(); err != nil {
		return nil, nil, err
	}
	if err := s.saveRun(ctx, run); err != nil {
		return nil, nil, err
	}
	if err := s.report.Generate(ctx, runID); err != nil {
		return nil, nil, fmt.Errorf("generate report: %w", err)
	}
	if err := run.Finish(); err != nil {
		return nil, nil, err
	}
	if err := s.saveRun(ctx, run); err != nil {
		return nil, nil, err
	}
	return run, results, nil
}

func (s *RunService) reserveExecutionPools(ctx context.Context, specs []*domain.CaseSpec) ([]domain.ExecutionPoolLease, error) {
	if s.pool == nil {
		return nil, nil
	}
	requests := make(map[string]int)
	for _, spec := range specs {
		resource := spec.Scenario
		if resource == "" {
			resource = "api"
		}
		requests[resource]++
	}
	leases := make([]domain.ExecutionPoolLease, 0, len(requests))
	resources := make([]string, 0, len(requests))
	for resource := range requests {
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	for _, resource := range resources {
		units := requests[resource]
		lease, reserveErr := s.pool.Reserve(ctx, domain.ExecutionPoolRequest{Resource: resource, Units: units})
		if reserveErr != nil {
			for i := len(leases) - 1; i >= 0; i-- {
				_ = s.pool.Release(context.Background(), leases[i])
			}
			return nil, fmt.Errorf("reserve execution pool for %s: %w", resource, reserveErr)
		}
		leases = append(leases, lease)
	}
	return leases, nil
}

func (s *RunService) recordResourceConflict(ctx context.Context, run *domain.Run, cause error) error {
	return s.audit.Append(ctx, domain.ExecutionAuditEvent{
		Op:      "execution.resource_conflict",
		Asset:   "run",
		AssetID: run.ID,
		Payload: map[string]any{
			"target_id":     run.TargetID,
			"resource_type": "target",
			"pool_id":       "default",
			"cause":         cause.Error(),
		},
	})
}

func scenarioFromScript(script any) string {
	if m, ok := script.(map[string]any); ok {
		if v, ok := m["scenario"].(string); ok && v != "" {
			return v
		}
		if v, ok := m["kind"].(string); ok && v != "" {
			return v
		}
	}
	return "api"
}

func executionCostRequestID(runID, caseID int64, attempt int) string {
	return fmt.Sprintf("run:%d:case:%d:attempt:%d", runID, caseID, attempt)
}

// PauseRun 暂停运行。
func (s *RunService) PauseRun(ctx context.Context, runID int64) (*domain.Run, error) {
	return s.transitionWithSignal(ctx, runID, (*domain.Run).Pause, "pause")
}

// ResumeRun 恢复运行。
func (s *RunService) ResumeRun(ctx context.Context, runID int64) (*domain.Run, error) {
	return s.transitionWithSignal(ctx, runID, (*domain.Run).Resume, "resume")
}

func (s *RunService) transitionWithSignal(ctx context.Context, runID int64, fn func(*domain.Run) error, signal string) (*domain.Run, error) {
	run, err := s.transition(ctx, runID, fn)
	if err != nil {
		return nil, err
	}
	teamID, _ := rls.TenantFrom(ctx)
	if err := s.workflow.Signal(ctx, runID, teamID, signal); err != nil {
		return nil, err
	}
	return run, nil
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
	if err := s.saveRun(ctx, run); err != nil {
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

// ListRuns returns tenant-scoped runs for history views.
func (s *RunService) ListRuns(ctx context.Context, targetID *int64, state string, limit int) ([]*domain.Run, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.List(ctx, teamID, targetID, state, limit)
}

// CaseResults 查询运行内用例结果。
func (s *RunService) CaseResults(ctx context.Context, runID int64) ([]*domain.CaseResult, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	return s.repo.CaseResults(ctx, teamID, runID)
}
