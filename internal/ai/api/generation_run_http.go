package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/openware-io/open-green-pass/internal/ai/application"
	"github.com/openware-io/open-green-pass/internal/ai/domain"
	"github.com/openware-io/open-green-pass/internal/gateway/httpx"
	"github.com/openware-io/open-green-pass/internal/iam"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type generationRunHandler struct {
	svc        *application.GenerationRunService
	gen        *id.Generator
	authorizer iam.AuthorizationPort
}

// RegisterGenerationRuns exposes run routes without authorization. It is kept
// for embedded/test compositions; production must use the Authorized variant.
func RegisterGenerationRuns(mux *http.ServeMux, svc *application.GenerationRunService, gen *id.Generator) {
	registerGenerationRuns(mux, svc, gen, nil, nil, nil, nil)
}

// RegisterGenerationRunsAuthorized registers the run API with project-level G55
// authorization. events is the append-only event repository used by SSE.
func RegisterGenerationRunsAuthorized(mux *http.ServeMux, svc *application.GenerationRunService, gen *id.Generator, events domain.GenerationRunEventRepository, authorizer iam.AuthorizationPort) {
	registerGenerationRuns(mux, svc, gen, events, nil, nil, authorizer)
}

// RegisterGenerationRunsAuthorizedWithNotifier keeps the notifier-only wiring
// for callers that use the default in-process SSE limiter.
func RegisterGenerationRunsAuthorizedWithNotifier(mux *http.ServeMux, svc *application.GenerationRunService, gen *id.Generator, events domain.GenerationRunEventRepository, notifier domain.GenerationRunEventNotifier, authorizer iam.AuthorizationPort) {
	registerGenerationRuns(mux, svc, gen, events, notifier, nil, authorizer)
}

// RegisterGenerationRunsAuthorizedWithWiring is the production registration: a
// PostgreSQL NOTIFY notifier for cross-process wake-ups plus a global
// (Redis-backed) SSE stream limiter shared by all replicas.
func RegisterGenerationRunsAuthorizedWithWiring(mux *http.ServeMux, svc *application.GenerationRunService, gen *id.Generator, events domain.GenerationRunEventRepository, notifier domain.GenerationRunEventNotifier, limiter domain.GenerationSSELimiter, authorizer iam.AuthorizationPort) {
	registerGenerationRuns(mux, svc, gen, events, notifier, limiter, authorizer)
}

func registerGenerationRuns(mux *http.ServeMux, svc *application.GenerationRunService, gen *id.Generator, events domain.GenerationRunEventRepository, notifier domain.GenerationRunEventNotifier, limiter domain.GenerationSSELimiter, authorizer iam.AuthorizationPort) {
	h := &generationRunHandler{svc: svc, gen: gen, authorizer: authorizer}
	mux.HandleFunc("POST /generation/runs", h.create)
	mux.HandleFunc("GET /generation/runs", h.list)
	mux.HandleFunc("GET /generation/runs/{id}", h.get)
	mux.HandleFunc("POST /generation/runs/{id}/cancel", h.cancel)
	mux.HandleFunc("POST /generation/runs/{id}/review", h.review)
	if events != nil {
		eh := newRunEventHandler(svc, events, notifier, limiter, authorizer)
		mux.HandleFunc("GET /generation/runs/{id}/events", eh.stream)
	}
}

type generationRunUnitRequest struct {
	ServiceID  int64  `json:"service_id"`
	RepoID     int64  `json:"repo_id"`
	SourcePath string `json:"source_path"`
	RefType    string `json:"ref_type"`
	RefName    string `json:"ref_name"`
}

type generationRunCreateRequest struct {
	ProjectID      int64                      `json:"project_id"`
	ScopeType      string                     `json:"scope_type"`
	ScopeID        int64                      `json:"scope_id"`
	IdempotencyKey string                     `json:"idempotency_key"`
	PlanID         int64                      `json:"plan_id"`
	PlanHash       string                     `json:"plan_hash"`
	Units          []generationRunUnitRequest `json:"units"`
}

func (h *generationRunHandler) create(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	var req generationRunCreateRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation run request")
		return
	}
	if req.ProjectID <= 0 || req.IdempotencyKey == "" {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "project_id and idempotency_key are required")
		return
	}
	// A plan-bound create takes scope from the persisted plan; only legacy
	// requests must carry an explicit scope. External clients may therefore
	// confirm with plan_id+plan_hash+project_id only.
	if req.PlanID == 0 && !domain.ValidGenerationScopeType(req.ScopeType) {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "scope_type is required unless plan_id is provided")
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "edit", req.ProjectID) {
		return
	}
	units := make([]application.GenerationRunUnitInput, 0, len(req.Units))
	for _, unit := range req.Units {
		units = append(units, application.GenerationRunUnitInput{
			ServiceID: unit.ServiceID, RepoID: unit.RepoID, SourcePath: unit.SourcePath,
			RefType: unit.RefType, RefName: unit.RefName,
		})
	}
	run, created, err := h.svc.Create(r.Context(), application.CreateGenerationRunCommand{
		ID: h.gen.Next(), TeamID: team, ProjectID: req.ProjectID, ScopeType: req.ScopeType, ScopeID: req.ScopeID,
		ActorID: actor, IdempotencyKey: req.IdempotencyKey, PlanID: req.PlanID, PlanHash: req.PlanHash, Units: units,
	})
	if err != nil {
		writeGenerationRunError(w, err)
		return
	}
	status := http.StatusAccepted
	if !created {
		status = http.StatusOK
	}
	httpx.WriteJSON(w, status, toGenerationRunDetail(run))
}

func (h *generationRunHandler) get(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation run id")
		return
	}
	run, err := h.svc.Get(r.Context(), team, runID)
	if err != nil {
		writeGenerationRunError(w, err)
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "view", run.ProjectID) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationRunDetail(run))
}

// generationRunStates is the closed run-state set accepted by the list filter.
var generationRunStates = map[string]struct{}{
	string(domain.GenerationRunQueued): {}, string(domain.GenerationRunRunning): {},
	string(domain.GenerationRunWaitingReview): {}, string(domain.GenerationRunSucceeded): {},
	string(domain.GenerationRunPartialFailed): {}, string(domain.GenerationRunFailed): {},
	string(domain.GenerationRunCancelled): {},
}

// list serves stable cursor pagination: runs are ordered by id DESC and
// next_cursor is the last id of the page (null when there is no older page).
func (h *generationRunHandler) list(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	projectID, beforeID, state, limit, ok := parseGenerationRunListQuery(w, r)
	if !ok {
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "view", projectID) {
		return
	}
	page, err := h.svc.ListByProjectPage(r.Context(), team, projectID, beforeID, state, limit)
	if err != nil {
		slog.Error("generation run page failed", "project_id", projectID, "cursor", beforeID, "state", state, "limit", limit, "err", err)
		writeGenerationError(w, http.StatusInternalServerError, "internal", "generation runs unavailable")
		return
	}
	runs := make([]map[string]any, 0, len(page.Runs))
	for _, run := range page.Runs {
		runs = append(runs, toGenerationRunSummary(run))
	}
	response := map[string]any{"runs": runs, "next_cursor": nil}
	if page.NextCursor != nil {
		response["next_cursor"] = strconv.FormatInt(*page.NextCursor, 10)
	}
	httpx.WriteJSON(w, http.StatusOK, response)
}

// parseGenerationRunListQuery validates project/limit/cursor/state before any
// authorization or data access, writing the API error envelope on failure.
func parseGenerationRunListQuery(w http.ResponseWriter, r *http.Request) (projectID, beforeID int64, state string, limit int, ok bool) {
	projectID, err := strconv.ParseInt(r.URL.Query().Get("project_id"), 10, 64)
	if err != nil || projectID <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "project_id is required")
		return 0, 0, "", 0, false
	}
	limit = 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 100 {
			writeGenerationError(w, http.StatusBadRequest, "validation_failed", "limit must be between 1 and 100")
			return 0, 0, "", 0, false
		}
		limit = parsed
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		parsed, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || parsed <= 0 {
			writeGenerationError(w, http.StatusBadRequest, "validation_failed", "cursor must be a positive run id")
			return 0, 0, "", 0, false
		}
		beforeID = parsed
	}
	state = r.URL.Query().Get("state")
	if state != "" {
		if _, valid := generationRunStates[state]; !valid {
			writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid run state filter")
			return 0, 0, "", 0, false
		}
	}
	return projectID, beforeID, state, limit, true
}

type generationRunReviewRequest struct {
	Decision  string `json:"decision"`
	RequestID string `json:"request_id"`
}

func (h *generationRunHandler) review(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation run id")
		return
	}
	existing, err := h.svc.Find(r.Context(), team, runID)
	if err != nil {
		writeGenerationRunError(w, err)
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "edit", existing.ProjectID) {
		return
	}
	var req generationRunReviewRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation review request")
		return
	}
	run, err := h.svc.Review(r.Context(), application.ReviewGenerationRunCommand{
		TeamID: team, RunID: runID, ActorID: actor, Decision: req.Decision, RequestID: req.RequestID,
	})
	if err != nil {
		writeGenerationRunError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationRunDetail(run))
}

func (h *generationRunHandler) cancel(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	actor, ok := generationActor(w, r)
	if !ok {
		return
	}
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation run id")
		return
	}
	existing, err := h.svc.Find(r.Context(), team, runID)
	if err != nil {
		writeGenerationRunError(w, err)
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "edit", existing.ProjectID) {
		return
	}
	run, err := h.svc.Cancel(r.Context(), team, runID, actor, "run-cancel-"+strconv.FormatInt(runID, 10))
	if err != nil {
		writeGenerationRunError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toGenerationRunDetail(run))
}

// runEventHandler streams the append-only event log as SSE. A cross-process
// PostgreSQL NOTIFY hint wakes the stream; a low-frequency DB fallback covers
// lost hints, a 15s keepalive keeps proxies alive, and a 30s snapshot prevents a
// console from showing stale state when a notification is missed.
type runEventHandler struct {
	svc        *application.GenerationRunService
	events     domain.GenerationRunEventRepository
	notifier   domain.GenerationRunEventNotifier
	limiter    domain.GenerationSSELimiter
	authorizer iam.AuthorizationPort
	// Intervals are fields so tests can compress them; production uses a 20s
	// fallback poll, 15s heartbeat and 30s snapshot.
	fallbackInterval  time.Duration
	heartbeatInterval time.Duration
	snapshotInterval  time.Duration
}

func newRunEventHandler(svc *application.GenerationRunService, events domain.GenerationRunEventRepository, notifier domain.GenerationRunEventNotifier, limiter domain.GenerationSSELimiter, authorizer iam.AuthorizationPort) *runEventHandler {
	if limiter == nil {
		limiter = defaultGenerationSSELimiter()
	}
	return &runEventHandler{
		svc: svc, events: events, notifier: notifier, limiter: limiter, authorizer: authorizer,
		fallbackInterval: 20 * time.Second, heartbeatInterval: 15 * time.Second, snapshotInterval: 30 * time.Second,
	}
}

func durationOr(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}

//nolint:gocyclo,gocognit // replay/notify/fallback/heartbeat/snapshot share one audited writer.
func (h *runEventHandler) stream(w http.ResponseWriter, r *http.Request) {
	team, ok := generationTenant(w, r)
	if !ok {
		return
	}
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation run id")
		return
	}
	run, err := h.svc.Get(r.Context(), team, runID)
	if err != nil {
		writeGenerationRunError(w, err)
		return
	}
	if !authorizeGeneration(w, r, h.authorizer, "view", run.ProjectID) {
		return
	}
	after, ok := eventCursor(w, r)
	if !ok {
		return
	}
	leaseID := newGenerationSSELeaseID()
	admitted, err := h.limiter.Acquire(r.Context(), team, leaseID)
	if err != nil {
		writeGenerationError(w, http.StatusServiceUnavailable, "limiter_unavailable", "generation event limiter is unavailable")
		return
	}
	if !admitted {
		generationSSETooManyStreams(w)
		return
	}
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.limiter.Release(releaseCtx, team, leaseID)
	}()
	renewLease := func() error {
		renewCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		return h.limiter.Renew(renewCtx, team, leaseID)
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeGenerationError(w, http.StatusInternalServerError, "internal", "streaming responses are not supported")
		return
	}
	// SSE outlives the global server WriteTimeout; clear the per-response write
	// deadline so long-lived consoles are not cut every 30 seconds.
	if controller := http.NewResponseController(w); controller != nil {
		_ = controller.SetWriteDeadline(time.Time{})
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil {
		return
	}
	flusher.Flush()
	writeFrame := func(seq int64, eventType string, snapshot *domain.GenerationRun) error {
		payload, err := json.Marshal(map[string]any{
			"seq":  seq,
			"type": eventType,
			"run":  toGenerationRunDetail(snapshot),
		})
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: run\ndata: %s\n\n", seq, payload); err != nil { //nolint:gosec // G705: payload is JSON-encoded event data, not HTML
			return err
		}
		flusher.Flush()
		return nil
	}
	// Subscribe before replay so hints emitted during replay are buffered; a
	// missing notifier only means the fallback poll does the wake-up.
	var notifications <-chan struct{}
	unsubscribe := func() {}
	if h.notifier != nil {
		channel, cancel, subscribeErr := h.notifier.Subscribe(r.Context(), runID)
		if subscribeErr == nil && channel != nil {
			notifications = channel
			if cancel != nil {
				unsubscribe = cancel
			}
		}
	}
	defer unsubscribe()
	replay, err := h.events.ListAfter(r.Context(), team, runID, after, 200)
	if err != nil {
		return
	}
	lastSent := after
	if len(replay) > 0 {
		for _, event := range replay {
			if err := writeFrame(event.Seq, string(event.Type), run); err != nil {
				return
			}
			lastSent = event.Seq
		}
	} else {
		cursor := after
		if last, lastErr := h.events.LastSeq(r.Context(), team, runID); lastErr == nil && last > cursor {
			cursor = last
		}
		if err := writeFrame(cursor, "run.snapshot", run); err != nil {
			return
		}
		lastSent = cursor
	}
	if domain.IsTerminalRunState(run.State) {
		return
	}
	// flushPending reads the authoritative event rows if any hint/fallback
	// fired. It returns whether frames were sent so the snapshot ticker can skip
	// a redundant snapshot right after events.
	flushPending := func() (sent bool, terminal bool, flushErr error) {
		pending, listErr := h.events.ListAfter(r.Context(), team, runID, lastSent, 200)
		if listErr != nil {
			return false, false, listErr
		}
		if len(pending) == 0 {
			return false, domain.IsTerminalRunState(run.State), nil
		}
		latest, getErr := h.svc.Get(r.Context(), team, runID)
		if getErr != nil {
			return false, false, getErr
		}
		run = latest
		for _, event := range pending {
			if writeErr := writeFrame(event.Seq, string(event.Type), run); writeErr != nil {
				return false, false, writeErr
			}
			lastSent = event.Seq
		}
		return true, domain.IsTerminalRunState(run.State), nil
	}
	fallback := time.NewTicker(durationOr(h.fallbackInterval, 20*time.Second))
	defer fallback.Stop()
	heartbeat := time.NewTicker(durationOr(h.heartbeatInterval, 15*time.Second))
	defer heartbeat.Stop()
	snapshot := time.NewTicker(durationOr(h.snapshotInterval, 30*time.Second))
	defer snapshot.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-notifications:
			_, terminal, flushErr := flushPending()
			if flushErr != nil || terminal {
				return
			}
		case <-fallback.C:
			_, terminal, flushErr := flushPending()
			if flushErr != nil || terminal {
				return
			}
		case <-heartbeat.C:
			if err := renewLease(); err != nil {
				return
			}
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-snapshot.C:
			if err := renewLease(); err != nil {
				return
			}
			sent, terminal, flushErr := flushPending()
			if flushErr != nil || terminal {
				return
			}
			if sent {
				continue
			}
			latest, getErr := h.svc.Get(r.Context(), team, runID)
			if getErr != nil {
				return
			}
			run = latest
			if err := writeFrame(lastSent, "run.snapshot", run); err != nil {
				return
			}
			if domain.IsTerminalRunState(run.State) {
				return
			}
		}
	}
}

func eventCursor(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value := r.URL.Query().Get("after_seq")
	if value == "" {
		value = r.Header.Get("Last-Event-ID")
	}
	if value == "" {
		return 0, true
	}
	cursor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cursor < 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "after_seq must be a non-negative integer")
		return 0, false
	}
	return cursor, true
}

// toGenerationRunDetail renders the frozen aggregate shape returned by run GET,
// SSE snapshots, review and cancel.
func toGenerationRunDetail(run *domain.GenerationRun) map[string]any {
	units := make([]map[string]any, 0, len(run.Units))
	for _, unit := range run.Units {
		units = append(units, map[string]any{
			"id": unit.ID, "service_id": unit.ServiceID, "service_name": unit.ServiceName,
			"repo_id": unit.RepoID, "repo_url": unit.RepoURL,
			"ref_type": unit.RefType, "ref_name": unit.RefName, "commit_sha": unit.CommitSHA,
			"batch_id": unit.BatchID, "batch_state": unit.BatchState, "stage": unit.Stage,
			"review_required": unit.ReviewRequired, "state": unit.State, "error": unit.Error,
		})
	}
	return map[string]any{
		"id": run.ID, "team_id": run.TeamID, "project_id": run.ProjectID,
		"scope_type": run.ScopeType, "scope_id": run.ScopeID, "state": run.State,
		"total_units": run.TotalUnits, "completed_units": run.CompletedUnits,
		"failed_units": run.FailedUnits, "review_units": run.WaitingReviewUnits,
		"progress_percent": domain.ProgressPercent(run.TotalUnits, run.CompletedUnits, run.FailedUnits),
		"tokens_in":        run.TokensIn, "tokens_out": run.TokensOut, "cost_amount": run.CostAmount,
		"plan_id": run.PlanID, "created_at": run.CreatedAt, "updated_at": run.UpdatedAt,
		"units": units,
	}
}

// toGenerationRunSummary is the history-list shape; it stays additive over the
// previous response so the history page can show progress without a second GET.
func toGenerationRunSummary(run *domain.GenerationRun) map[string]any {
	units := make([]map[string]any, 0, len(run.Units))
	for _, unit := range run.Units {
		units = append(units, map[string]any{
			"id": unit.ID, "service_id": unit.ServiceID, "service_name": unit.ServiceName,
			"repo_id": unit.RepoID, "repo_url": unit.RepoURL, "batch_id": unit.BatchID,
			"batch_state": unit.BatchState, "stage": unit.Stage, "review_required": unit.ReviewRequired,
			"state": unit.State, "error": unit.Error,
			"ref_type": unit.RefType, "ref_name": unit.RefName, "commit_sha": unit.CommitSHA,
		})
	}
	return map[string]any{
		"id": run.ID, "team_id": run.TeamID, "project_id": run.ProjectID,
		"scope_type": run.ScopeType, "scope_id": run.ScopeID, "state": run.State,
		"total_units": run.TotalUnits, "completed_units": run.CompletedUnits,
		"failed_units": run.FailedUnits, "review_units": run.WaitingReviewUnits,
		"progress_percent": domain.ProgressPercent(run.TotalUnits, run.CompletedUnits, run.FailedUnits),
		"tokens_in":        run.TokensIn, "tokens_out": run.TokensOut, "cost_amount": run.CostAmount,
		"plan_id": run.PlanID, "created_at": run.CreatedAt, "updated_at": run.UpdatedAt,
		"units": units,
	}
}

// writeGenerationRunError maps domain sentinels to the frozen HTTP semantics.
func writeGenerationRunError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrGenerationRunIdempotencyConflict):
		writeGenerationError(w, http.StatusConflict, "idempotency_conflict", "idempotency key is already used for a different generation scope")
	case errors.Is(err, domain.ErrGenerationPlanStale), errors.Is(err, domain.ErrGenerationPlanExpired):
		writeGenerationError(w, http.StatusConflict, "plan_stale", "generation plan expired or no longer matches the request; run preflight again")
	case errors.Is(err, domain.ErrGenerationPlanInvalid):
		writeGenerationError(w, http.StatusConflict, "plan_invalid", "generation plan still contains invalid units")
	case errors.Is(err, domain.ErrGenerationRunNotFound), errors.Is(err, domain.ErrGenerationNotFound):
		writeGenerationError(w, http.StatusNotFound, "not_found", "generation run not found")
	case errors.Is(err, domain.ErrGenerationStateConflict), errors.Is(err, domain.ErrGenerationRunConflict):
		writeGenerationError(w, http.StatusConflict, "state_conflict", "generation state changed concurrently; refresh and retry")
	case errors.Is(err, application.ErrGenerationRunCancelIncomplete):
		writeGenerationError(w, http.StatusConflict, "cancel_incomplete", "部分生成批次取消失败，任务未标记为已取消；请稍后重试或联系管理员")
	default:
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", err.Error())
	}
}
