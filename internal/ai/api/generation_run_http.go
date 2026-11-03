package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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

// stream is a thin orchestration around authorization, replay and the notify
// loop; each phase is a helper so the audited writer stays reviewable.
func (h *runEventHandler) stream(w http.ResponseWriter, r *http.Request) {
	team, runID, run, after, ok := h.authorizeStream(w, r)
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
	stream, ok := h.newRunEventStream(w, r, team, runID, after, run, leaseID)
	if !ok {
		return
	}
	defer stream.unsubscribe()
	if !stream.replay(r.Context()) {
		return
	}
	stream.serve(r.Context())
}

// authorizeStream validates tenant/run/authorization/cursor before any stream
// resources are acquired. Authorization errors already write the HTTP response.
func (h *runEventHandler) authorizeStream(w http.ResponseWriter, r *http.Request) (team, runID int64, run *domain.GenerationRun, after int64, ok bool) {
	team, ok = generationTenant(w, r)
	if !ok {
		return 0, 0, nil, 0, false
	}
	runID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || runID <= 0 {
		writeGenerationError(w, http.StatusBadRequest, "validation_failed", "invalid generation run id")
		return 0, 0, nil, 0, false
	}
	run, err = h.svc.Get(r.Context(), team, runID)
	if err != nil {
		writeGenerationRunError(w, err)
		return 0, 0, nil, 0, false
	}
	if !authorizeGeneration(w, r, h.authorizer, "view", run.ProjectID) {
		return 0, 0, nil, 0, false
	}
	after, ok = eventCursor(w, r)
	if !ok {
		return 0, 0, nil, 0, false
	}
	return team, runID, run, after, true
}

type runEventStream struct {
	w             http.ResponseWriter
	flusher       http.Flusher
	handler       *runEventHandler
	team          int64
	runID         int64
	leaseID       string
	run           *domain.GenerationRun
	lastSent      int64
	notifications <-chan struct{}
	unsubscribe   func()
}

// newRunEventStream flushes SSE headers, subscribes the notifier and builds the
// frame writer. A missing notifier only means the fallback poll wakes the stream.
func (h *runEventHandler) newRunEventStream(w http.ResponseWriter, r *http.Request, team, runID, after int64, run *domain.GenerationRun, leaseID string) (*runEventStream, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeGenerationError(w, http.StatusInternalServerError, "internal", "streaming responses are not supported")
		return nil, false
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
		return nil, false
	}
	flusher.Flush()
	stream := &runEventStream{
		w: w, flusher: flusher, handler: h, team: team, runID: runID,
		leaseID: leaseID, run: run, lastSent: after, unsubscribe: func() {},
	}
	if h.notifier != nil {
		channel, cancel, subscribeErr := h.notifier.Subscribe(r.Context(), runID)
		if subscribeErr == nil && channel != nil {
			stream.notifications = channel
			if cancel != nil {
				stream.unsubscribe = cancel
			}
		}
	}
	return stream, true
}

func (s *runEventStream) writeFrame(seq int64, eventType string, snapshot *domain.GenerationRun) error {
	payload, err := json.Marshal(map[string]any{
		"seq":  seq,
		"type": eventType,
		"run":  toGenerationRunDetail(snapshot),
	})
	if err != nil {
		return err
	}
	frame := fmt.Sprintf("id: %d\nevent: run\ndata: %s\n\n", seq, payload)
	if _, err := io.Copy(s.w, strings.NewReader(frame)); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}

// replay sends buffered events after the cursor, or a snapshot when there is
// nothing to replay; it returns false when the stream must close.
func (s *runEventStream) replay(ctx context.Context) bool {
	events, err := s.handler.events.ListAfter(ctx, s.team, s.runID, s.lastSent, 200)
	if err != nil {
		return false
	}
	if len(events) > 0 {
		for _, event := range events {
			if err := s.writeFrame(event.Seq, string(event.Type), s.run); err != nil {
				return false
			}
			s.lastSent = event.Seq
		}
		return !domain.IsTerminalRunState(s.run.State)
	}
	cursor := s.lastSent
	if last, lastErr := s.handler.events.LastSeq(ctx, s.team, s.runID); lastErr == nil && last > cursor {
		cursor = last
	}
	if err := s.writeFrame(cursor, "run.snapshot", s.run); err != nil {
		return false
	}
	s.lastSent = cursor
	return !domain.IsTerminalRunState(s.run.State)
}

func (s *runEventStream) renewLease(ctx context.Context) error {
	renewCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.handler.limiter.Renew(renewCtx, s.team, s.leaseID)
}

// serve runs the notify/fallback/heartbeat/snapshot loop until terminal state,
// renewal failure or write error.
func (s *runEventStream) serve(ctx context.Context) {
	fallback := time.NewTicker(durationOr(s.handler.fallbackInterval, 20*time.Second))
	defer fallback.Stop()
	heartbeat := time.NewTicker(durationOr(s.handler.heartbeatInterval, 15*time.Second))
	defer heartbeat.Stop()
	snapshot := time.NewTicker(durationOr(s.handler.snapshotInterval, 30*time.Second))
	defer snapshot.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.notifications:
			if !s.handlePending(ctx) {
				return
			}
		case <-fallback.C:
			if !s.handlePending(ctx) {
				return
			}
		case <-heartbeat.C:
			if !s.handleHeartbeat(ctx) {
				return
			}
		case <-snapshot.C:
			if !s.handleSnapshot(ctx) {
				return
			}
		}
	}
}

func (s *runEventStream) handlePending(ctx context.Context) bool {
	_, terminal, err := s.flushPending(ctx)
	return err == nil && !terminal
}

func (s *runEventStream) handleHeartbeat(ctx context.Context) bool {
	if err := s.renewLease(ctx); err != nil {
		return false
	}
	if _, err := fmt.Fprint(s.w, ": keepalive\n\n"); err != nil {
		return false
	}
	s.flusher.Flush()
	return true
}

func (s *runEventStream) handleSnapshot(ctx context.Context) bool {
	if err := s.renewLease(ctx); err != nil {
		return false
	}
	sent, terminal, err := s.flushPending(ctx)
	if err != nil || terminal {
		return false
	}
	if sent {
		return true
	}
	latest, getErr := s.handler.svc.Get(ctx, s.team, s.runID)
	if getErr != nil {
		return false
	}
	s.run = latest
	if err := s.writeFrame(s.lastSent, "run.snapshot", s.run); err != nil {
		return false
	}
	return !domain.IsTerminalRunState(s.run.State)
}

// flushPending reads authoritative rows after the last sent cursor. sent lets
// the snapshot ticker avoid a redundant snapshot right after events.
func (s *runEventStream) flushPending(ctx context.Context) (sent bool, terminal bool, flushErr error) {
	pending, listErr := s.handler.events.ListAfter(ctx, s.team, s.runID, s.lastSent, 200)
	if listErr != nil {
		return false, false, listErr
	}
	if len(pending) == 0 {
		return false, domain.IsTerminalRunState(s.run.State), nil
	}
	latest, getErr := s.handler.svc.Get(ctx, s.team, s.runID)
	if getErr != nil {
		return false, false, getErr
	}
	s.run = latest
	for _, event := range pending {
		if writeErr := s.writeFrame(event.Seq, string(event.Type), s.run); writeErr != nil {
			return false, false, writeErr
		}
		s.lastSent = event.Seq
	}
	return true, domain.IsTerminalRunState(s.run.State), nil
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
