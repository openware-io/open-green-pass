// Package application 执行域应用服务：测试运行编排（创建/版本校验前置/执行/重跑/暂停）。
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
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
	repo               domain.RunRepository
	envSvc             *EnvService
	cases              domain.CasePort
	policy             domain.PolicyPort
	runner             domain.RunnerPort
	gen                *id.Generator
	log                *slog.Logger
	workflow           WorkflowController
	conflict           domain.ConflictPort
	quota              domain.QuotaPort
	audit              domain.AuditPort
	gate               domain.GatePort
	report             domain.ReportPort
	cost               domain.CostPort
	events             domain.RunEventBus
	pool               domain.ExecutionPoolPort
	schedule           domain.ScheduleQueuePort
	leaseRenewInterval time.Duration
	finding            FindingPort
	coverage           CoveragePort
}

// NewRunService 创建运行编排服务。
func NewRunService(repo domain.RunRepository, envSvc *EnvService, cases domain.CasePort, policy domain.PolicyPort, runner domain.RunnerPort, gen *id.Generator) *RunService {
	return &RunService{repo: repo, envSvc: envSvc, cases: cases, policy: policy, runner: runner, gen: gen, log: slog.Default(), workflow: NoopWorkflowController{}, conflict: domain.NoopConflictPort{}, quota: domain.NoopQuotaPort{}, audit: domain.NoopAuditPort{}, gate: domain.NoopGatePort{}, report: domain.NoopReportPort{}, cost: domain.NoopCostPort{}, leaseRenewInterval: 30 * time.Second}
}

// SetFindingPort injects the governance finding boundary.
func (s *RunService) SetFindingPort(port FindingPort) {
	if port != nil {
		s.finding = port
	}
}

// SetCoveragePort injects the coverage ledger boundary.
func (s *RunService) SetCoveragePort(port CoveragePort) {
	if port != nil {
		s.coverage = port
	}
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

// SetLeaseRenewInterval is intended for controlled deployment tuning and
// deterministic tests; production should keep the default below the lease TTL.
func (s *RunService) SetLeaseRenewInterval(interval time.Duration) {
	if interval > 0 {
		s.leaseRenewInterval = interval
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
func (s *RunService) HasScheduleQueue() bool { return s.schedule != nil }

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

// MarkFailed durably closes a run when a workflow activity ends without a
// completed execution result. This prevents queued/running rows from becoming
// permanently stale after a fail-closed preflight or runner error.
func (s *RunService) MarkFailed(ctx context.Context, runID int64) error {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return ErrTenantRequired
	}
	run, err := s.repo.Find(ctx, teamID, runID)
	if err != nil {
		return err
	}
	if run.State == domain.RunFailed || run.State == domain.RunDone || run.State == domain.RunCancelled {
		return nil
	}
	if err := run.Fail(); err != nil {
		return err
	}
	return s.saveRun(ctx, run)
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
	// A stop may land before the activity starts; never execute a cancelled run.
	if run.State == domain.RunCancelled {
		return run, nil, nil
	}
	quotaRequest, err := s.acquireRunQuota(ctx, teamID, run)
	if err != nil {
		return nil, nil, err
	}
	defer s.releaseRunQuota(&err, quotaRequest)

	claimToken, err := s.claimRunTarget(ctx, teamID, run)
	if err != nil {
		return nil, nil, err
	}
	defer s.releaseRunClaim(&err, claimToken)

	if err := advanceRunToRunning(run); err != nil {
		return nil, nil, err
	}
	spec, err := s.buildCaseSpecs(ctx, teamID, run)
	if err != nil {
		return nil, nil, err
	}
	poolLeases, err := s.reserveExecutionPools(ctx, spec)
	if err != nil {
		return nil, nil, err
	}
	defer s.releasePoolLeases(&err, poolLeases)

	// Keep shared resource leases alive while a real runner is executing. A
	// stale fencing generation cancels the runner instead of allowing an old
	// worker to continue using capacity after takeover.
	results, err = s.executeWithLeaseRenewal(ctx, spec, poolLeases)
	if err != nil {
		return s.handleExecuteFailure(ctx, runID, run, results, err)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return run, results, ctxErr
	}
	// Cancellation can also arrive between the runner returning and the
	// collect/gate/report transitions; reload the durable state and stop.
	if fresh, freshErr := s.repo.Find(ctx, teamID, runID); freshErr == nil && fresh.State == domain.RunCancelled {
		return fresh, results, nil
	}
	if results, err = s.saveCaseResults(ctx, teamID, runID, run, results); err != nil {
		return nil, nil, err
	}
	if err := s.finishRun(ctx, run); err != nil {
		return nil, nil, err
	}
	return run, results, nil
}

// acquireRunQuota reserves one run slot before any runner work starts.
func (s *RunService) acquireRunQuota(ctx context.Context, teamID int64, run *domain.Run) (domain.ResourceRequest, error) {
	ownerID, ok := rls.UserFrom(ctx)
	if !ok {
		// Temporal activities created before owner propagation use the immutable
		// run ID as an isolated owner scope. HTTP requests carry the real user ID.
		ownerID = run.ID
	}
	request := domain.ResourceRequest{TeamID: teamID, TargetID: run.TargetID, OwnerID: ownerID, Type: "run", Units: 1}
	if err := s.quota.Acquire(ctx, request); err != nil {
		return domain.ResourceRequest{}, err
	}
	return request, nil
}

func (s *RunService) releaseRunQuota(err *error, request domain.ResourceRequest) {
	if releaseErr := s.quota.Release(context.Background(), request); *err == nil && releaseErr != nil {
		*err = releaseErr
	}
}

// claimRunTarget takes the exclusive target claim. GP2-04 reference boundary:
// the target is exclusive by default. A shared implementation can use a finer
// resource scope without changing the application service contract.
func (s *RunService) claimRunTarget(ctx context.Context, teamID int64, run *domain.Run) (string, error) {
	claimToken, err := s.conflict.Acquire(ctx, domain.ResourceClaim{
		TeamID: teamID, TargetID: run.TargetID, ResourceType: "target", PoolID: "default", OwnerID: run.ID, Exclusive: true,
	})
	if err == nil {
		return claimToken, nil
	}
	if !errors.Is(err, domain.ErrResourceConflict) {
		return "", err
	}
	if err := s.recordResourceConflict(ctx, run, err); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%w: %v", ErrResourceConflict, err)
}

func (s *RunService) releaseRunClaim(err *error, claimToken string) {
	if claimToken == "" {
		return
	}
	if releaseErr := s.conflict.Release(context.Background(), claimToken); *err == nil && releaseErr != nil {
		*err = releaseErr
	}
}

// advanceRunToRunning implements 首次：scheduled→running；重跑（done/failed
// 终态）：重置为 running，attempt 递增；paused 通过 Resume 继续。
func advanceRunToRunning(run *domain.Run) error {
	switch run.State {
	case domain.RunDone, domain.RunFailed:
		run.State = domain.RunRunning
	case domain.RunPaused:
		return run.Resume()
	default:
		return run.Schedule()
	}
	return nil
}

// buildCaseSpecs 装配用例执行规格（勾选范围 + 服务级截图开关策略）。
func (s *RunService) buildCaseSpecs(ctx context.Context, teamID int64, run *domain.Run) ([]*domain.CaseSpec, error) {
	shot, err := s.policy.Get(ctx, teamID, run.TargetID, run.ScenarioID)
	if err != nil {
		return nil, err
	}
	spec := make([]*domain.CaseSpec, 0, len(run.SelectedCases))
	for _, caseID := range run.SelectedCases {
		caseSpec, err := s.buildCaseSpec(ctx, teamID, run, shot, caseID)
		if err != nil {
			return nil, err
		}
		spec = append(spec, caseSpec)
	}
	return spec, nil
}

func (s *RunService) buildCaseSpec(ctx context.Context, teamID int64, run *domain.Run, shot *domain.ScreenshotPolicy, caseID int64) (*domain.CaseSpec, error) {
	version, script, err := s.fetchCaseScript(ctx, teamID, run.TargetID, caseID)
	if err != nil {
		return nil, err
	}
	requiresSandbox := hasDeclarativeHTTPRequest(script)
	baseURL := ""
	if requiresSandbox {
		if s.envSvc == nil {
			return nil, fmt.Errorf("environment runtime service is required to execute generated API cases")
		}
		if baseURL, err = s.envSvc.ExecutionBaseURL(ctx, run.TargetID, run.Env); err != nil {
			return nil, err
		}
	}
	return &domain.CaseSpec{
		CaseID: caseID, CaseVersion: version, TargetID: run.TargetID, Env: run.Env, BaseURL: baseURL, RequiresSandbox: requiresSandbox,
		Scenario: scenarioFromScript(script), Script: script, ScreenshotEnabled: shot.ScreenshotEnabled,
	}, nil
}

func (s *RunService) fetchCaseScript(ctx context.Context, teamID, targetID, caseID int64) (int, any, error) {
	if scoped, ok := s.cases.(domain.TargetScopedCasePort); ok {
		return scoped.FetchScriptForTarget(ctx, teamID, targetID, caseID)
	}
	return s.cases.FetchScript(ctx, teamID, caseID)
}

func (s *RunService) releasePoolLeases(err *error, leases []domain.ExecutionPoolLease) {
	for i := len(leases) - 1; i >= 0; i-- {
		if releaseErr := s.pool.Release(context.Background(), leases[i]); *err == nil && releaseErr != nil {
			*err = releaseErr
		}
	}
}

func (s *RunService) handleExecuteFailure(ctx context.Context, runID int64, run *domain.Run, results []*domain.CaseResult, err error) (*domain.Run, []*domain.CaseResult, error) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		// User cancelled while the runner was executing. The run row is
		// already cancelled; do not append gate/report side effects.
		return run, results, ctxErr
	}
	s.log.Error("execute cases failed", "run_id", runID, "error", err)
	return nil, nil, err
}

// saveCaseResults 落库：同 run 同 case 重跑 attempt_seq 递增。
func (s *RunService) saveCaseResults(ctx context.Context, teamID, runID int64, run *domain.Run, results []*domain.CaseResult) ([]*domain.CaseResult, error) {
	for _, c := range results {
		if err := s.saveCaseResult(ctx, teamID, runID, run, c); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (s *RunService) saveCaseResult(ctx context.Context, teamID, runID int64, run *domain.Run, c *domain.CaseResult) error {
	attempt, err := s.repo.MaxAttempt(ctx, teamID, runID, c.CaseID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	ended := now
	c.TeamID = teamID
	c.RunID = runID
	c.AttemptSeq = attempt + 1
	c.StartedAt = now
	c.EndedAt = &ended
	if err := s.repo.SaveCaseResult(ctx, c); err != nil {
		s.log.Error("save case result failed", "run_id", runID, "case_id", c.CaseID, "error", err)
		return err
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
		return fmt.Errorf("record execution cost: %w", err)
	}
	return nil
}

// finishRun advances the run through its terminal collect/gate/report states.
// Gate and report are trusted-domain side effects. A non-pass gate is a quality
// outcome, not an execution infrastructure failure: the Run still reaches done
// and the generated report carries the authoritative decision.
func (s *RunService) finishRun(ctx context.Context, run *domain.Run) error {
	if err := s.advanceRunState(ctx, run, run.StartCollect); err != nil {
		return err
	}
	if err := s.advanceRunState(ctx, run, run.StartGate); err != nil {
		return err
	}
	if _, err := s.gate.Evaluate(ctx, run.ID); err != nil {
		return fmt.Errorf("evaluate gate: %w", err)
	}
	if err := s.advanceRunState(ctx, run, run.StartReport); err != nil {
		return err
	}
	if err := s.report.Generate(ctx, run.ID); err != nil {
		return fmt.Errorf("generate report: %w", err)
	}
	return s.advanceRunState(ctx, run, run.Finish)
}

func (s *RunService) advanceRunState(ctx context.Context, run *domain.Run, transition func() error) error {
	if err := transition(); err != nil {
		return err
	}
	return s.saveRun(ctx, run)
}

func hasDeclarativeHTTPRequest(script any) bool {
	root, ok := script.(map[string]any)
	if !ok {
		return false
	}
	_, ok = root["request"].(map[string]any)
	if ok {
		return true
	}
	// The first generated case shape is still executable for compatibility.
	path, _ := root["path"].(string)
	method, _ := root["method"].(string)
	return strings.TrimSpace(path) != "" && strings.TrimSpace(method) != ""
}

func (s *RunService) executeWithLeaseRenewal(ctx context.Context, specs []*domain.CaseSpec, leases []domain.ExecutionPoolLease) ([]*domain.CaseResult, error) {
	if len(leases) == 0 || s.pool == nil {
		return s.runner.Execute(ctx, specs)
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	renewErr := make(chan error, 1)
	renewCtx, stopRenew := context.WithCancel(context.Background())
	defer stopRenew()
	go func() {
		interval := s.leaseRenewInterval
		if interval <= 0 {
			interval = 30 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				for i := range leases {
					lease, err := s.pool.Renew(renewCtx, leases[i])
					if err != nil {
						select {
						case renewErr <- fmt.Errorf("renew execution pool lease %s: %w", leases[i].Token, err):
						default:
						}
						cancel()
						return
					}
					leases[i] = lease
				}
			}
		}
	}()
	results, err := s.runner.Execute(runCtx, specs)
	select {
	case leaseErr := <-renewErr:
		return nil, leaseErr
	default:
	}
	if err != nil {
		return nil, err
	}
	return results, nil
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

// CancelRun 停止运行：先把 run 置为终态 cancelled 并持久化（用户意图优先），
// 再请求 Temporal 取消 workflow；workflow 取消会传播到运行中的 Runner，
// HTTP/K8s runner 据此中止当前请求/删除 Job。重复停止是幂等的。
func (s *RunService) CancelRun(ctx context.Context, runID int64, reason string) (*domain.Run, error) {
	teamID, ok := rls.TenantFrom(ctx)
	if !ok {
		return nil, ErrTenantRequired
	}
	run, err := s.repo.Find(ctx, teamID, runID)
	if err != nil {
		return nil, err
	}
	if run.State == domain.RunDone || run.State == domain.RunFailed || run.State == domain.RunCancelled {
		return run, nil
	}
	if err := run.Cancel(); err != nil {
		if errors.Is(err, domain.ErrRunFinished) {
			return run, nil
		}
		return nil, err
	}
	now := time.Now().UTC()
	run.CancelledAt = &now
	run.EndedAt = &now
	run.CancelReason = strings.TrimSpace(reason)
	if len(run.CancelReason) > 512 {
		run.CancelReason = run.CancelReason[:512]
	}
	if err := s.saveRun(ctx, run); err != nil {
		return nil, err
	}
	if err := s.workflow.Cancel(ctx, runID, teamID); err != nil {
		// The run is already durably cancelled; a missing/closed workflow is not
		// allowed to turn a successful stop into an error.
		s.log.Warn("cancel run workflow failed; run remains cancelled", "run_id", runID, "err", err)
	}
	return run, nil
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
	results, err := s.repo.CaseResults(ctx, teamID, runID)
	if err != nil {
		s.log.Error("list case results failed", "run_id", runID, "error", err)
	}
	return results, err
}
