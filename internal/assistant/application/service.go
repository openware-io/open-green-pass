// Package application implements the bounded, user-visible assistant flow.
package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/pkg/id"
)

var (
	ErrNotFound         = errors.New("assistant: resource not found")
	ErrInvalidInput     = errors.New("assistant: invalid input")
	ErrInvalidAction    = errors.New("assistant: unsupported UI action")
	ErrAuditUnavailable = errors.New("assistant: audit unavailable")
	ErrModelUnavailable = errors.New("assistant: model unavailable")
)

type Session struct {
	ID        int64     `json:"id"`
	Subject   string    `json:"-"`
	Page      string    `json:"page"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Turn struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Role      string    `json:"role"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type Artifact struct {
	ID             int64     `json:"id"`
	Kind           string    `json:"kind"`
	Priority       string    `json:"priority"`
	Title          string    `json:"title"`
	Summary        string    `json:"summary"`
	Recommendation string    `json:"recommendation"`
	Evidence       []string  `json:"evidence"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// UIAction is deliberately a manifest, never a selector, script, or arbitrary URL.
type UIAction struct {
	ID     int64             `json:"id"`
	Kind   string            `json:"kind"`
	Route  string            `json:"route,omitempty"`
	Target string            `json:"target,omitempty"`
	Values map[string]string `json:"values,omitempty"`
	Label  string            `json:"label"`
}

type ActionRecord struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Repository interface {
	CreateSession(context.Context, Session, string) error
	Session(context.Context, int64) (Session, error)
	AddTurns(context.Context, []Turn) error
	Turns(context.Context, int64) ([]Turn, error)
	CreateArtifact(context.Context, Artifact) error
	Artifacts(context.Context) ([]Artifact, error)
	CreateAction(context.Context, int64, UIAction, string) (ActionRecord, error)
	Actions(context.Context, int64) ([]ActionRecord, error)
	ActionBelongs(context.Context, int64, int64) (bool, error)
}

type AuditPort interface {
	Append(context.Context, int64, string, string, int64, map[string]any) error
}

type ModelResponder interface {
	Respond(context.Context, string, string) (answer string, model string, err error)
}

type AuditFunc func(context.Context, int64, string, string, int64, map[string]any) error

func (f AuditFunc) Append(ctx context.Context, actor int64, op, asset string, assetID int64, payload map[string]any) error {
	return f(ctx, actor, op, asset, assetID, payload)
}

type Service struct {
	repo      Repository
	audit     AuditPort
	gen       *id.Generator
	responder ModelResponder
}

func NewService(repo Repository, audit AuditPort, gen *id.Generator) *Service {
	return &Service{repo: repo, audit: audit, gen: gen}
}

func (s *Service) SetModelResponder(responder ModelResponder) { s.responder = responder }

func (s *Service) CreateSession(ctx context.Context, actor int64, subject, page string) (Session, error) {
	page = normalizePage(page)
	if page == "" || strings.TrimSpace(subject) == "" {
		return Session{}, ErrInvalidInput
	}
	now := time.Now().UTC()
	session := Session{ID: s.gen.Next(), Page: page, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateSession(ctx, session, subject); err != nil {
		return Session{}, err
	}
	if err := s.auditEvent(ctx, actor, "assistant.session.created", "assistant_session", session.ID, map[string]any{"page": page}); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) AddTurn(ctx context.Context, actor int64, subject string, sessionID int64, text string) (Turn, Turn, *Artifact, error) {
	session, err := s.ownedSession(ctx, sessionID, subject)
	if err != nil {
		return Turn{}, Turn{}, nil, err
	}
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 4000 {
		return Turn{}, Turn{}, nil, ErrInvalidInput
	}
	var answerText string
	var model string
	var artifact *Artifact
	if s.responder != nil {
		answerText, model, err = s.responder.Respond(ctx, session.Page, text)
		if err != nil {
			return Turn{}, Turn{}, nil, s.failedModelTurn(ctx, actor, sessionID, text, err)
		}
		if strings.TrimSpace(answerText) == "" {
			return Turn{}, Turn{}, nil, s.failedModelTurn(ctx, actor, sessionID, text, errors.New("assistant: model returned empty answer"))
		}
	} else {
		answerText, artifact = deterministicResponse(session.Page, text, s.gen.Next())
		model = "deterministic-fallback"
	}
	now := time.Now().UTC()
	user := Turn{ID: s.gen.Next(), SessionID: sessionID, Role: "user", Text: text, CreatedAt: now}
	assistant := Turn{ID: s.gen.Next(), SessionID: sessionID, Role: "assistant", Text: answerText, CreatedAt: now}
	if err := s.repo.AddTurns(ctx, []Turn{user, assistant}); err != nil {
		return Turn{}, Turn{}, nil, err
	}
	if err := s.auditEvent(ctx, actor, "assistant.turn.created", "assistant_session", sessionID, map[string]any{"role": "user", "text_sha256": digest(text)}); err != nil {
		return Turn{}, Turn{}, nil, err
	}
	if err := s.auditEvent(ctx, actor, "assistant.turn.created", "assistant_session", sessionID, map[string]any{"role": "assistant", "text_sha256": digest(answerText), "model": model}); err != nil {
		return Turn{}, Turn{}, nil, err
	}
	if artifact != nil {
		if err := s.repo.CreateArtifact(ctx, *artifact); err != nil {
			return Turn{}, Turn{}, nil, err
		}
		if err := s.auditEvent(ctx, 0, "assistant.artifact.created", "assistant_artifact", artifact.ID, map[string]any{"kind": artifact.Kind, "priority": artifact.Priority, "source": "assistant_analysis"}); err != nil {
			return Turn{}, Turn{}, nil, err
		}
	}
	return user, assistant, artifact, nil
}

func (s *Service) failedModelTurn(ctx context.Context, actor, sessionID int64, text string, cause error) error {
	if err := s.auditEvent(ctx, actor, "assistant.turn.failed", "assistant_session", sessionID, map[string]any{
		"text_sha256": digest(text),
		"reason":      "model_unavailable",
	}); err != nil {
		return fmt.Errorf("%w: failed request audit could not be written", ErrAuditUnavailable)
	}
	return fmt.Errorf("%w: %v", ErrModelUnavailable, cause)
}

func (s *Service) Turns(ctx context.Context, subject string, sessionID int64) ([]Turn, error) {
	if _, err := s.ownedSession(ctx, sessionID, subject); err != nil {
		return nil, err
	}
	return s.repo.Turns(ctx, sessionID)
}
func (s *Service) Artifacts(ctx context.Context) ([]Artifact, error) { return s.repo.Artifacts(ctx) }
func (s *Service) Actions(ctx context.Context, subject string, sessionID int64) ([]ActionRecord, error) {
	if _, err := s.ownedSession(ctx, sessionID, subject); err != nil {
		return nil, err
	}
	return s.repo.Actions(ctx, sessionID)
}

// StartGuidance returns only actions from the hard-coded allowlist. The browser performs
// them visibly; no command is a submit, network call, JavaScript, or selector.
func (s *Service) StartGuidance(ctx context.Context, actor int64, subject string, sessionID int64) ([]UIAction, error) {
	session, err := s.ownedSession(ctx, sessionID, subject)
	if err != nil {
		return nil, err
	}
	actions := guidanceForPage(session.Page, s.gen)
	for _, action := range actions {
		if !allowed(action) {
			return nil, ErrInvalidAction
		}
		if _, err := s.repo.CreateAction(ctx, sessionID, action, "issued"); err != nil {
			return nil, err
		}
		if err := s.auditEvent(ctx, actor, "ui."+action.Kind, "assistant_action", action.ID, map[string]any{"route": action.Route, "target": action.Target, "phase": "issued"}); err != nil {
			return nil, err
		}
	}
	return actions, nil
}

func (s *Service) CompleteAction(ctx context.Context, actor int64, subject string, sessionID, actionID int64, status string) (ActionRecord, error) {
	if _, err := s.ownedSession(ctx, sessionID, subject); err != nil {
		return ActionRecord{}, err
	}
	if status != "completed" && status != "cancelled" {
		return ActionRecord{}, ErrInvalidInput
	}
	belongs, err := s.repo.ActionBelongs(ctx, sessionID, actionID)
	if err != nil {
		return ActionRecord{}, err
	}
	if !belongs {
		return ActionRecord{}, ErrNotFound
	}
	// The row is append-only: completion is represented as a new record.
	record, err := s.repo.CreateAction(ctx, sessionID, UIAction{ID: s.gen.Next(), Kind: "guide", Target: "action:" + int64Text(actionID)}, status)
	if err != nil {
		return ActionRecord{}, err
	}
	if err := s.auditEvent(ctx, actor, "ui.guide."+status, "assistant_action", actionID, map[string]any{"session_id": sessionID}); err != nil {
		return ActionRecord{}, err
	}
	return record, nil
}

func (s *Service) ownedSession(ctx context.Context, id int64, subject string) (Session, error) {
	v, err := s.repo.Session(ctx, id)
	if err != nil {
		return Session{}, err
	}
	if v.Subject != subject {
		return Session{}, ErrNotFound
	}
	return v, nil
}
func (s *Service) auditEvent(ctx context.Context, actor int64, op, asset string, assetID int64, payload map[string]any) error {
	if s.audit == nil {
		return ErrAuditUnavailable
	}
	return s.audit.Append(ctx, actor, op, asset, assetID, payload)
}
func normalizePage(page string) string {
	switch page {
	case "/gate", "/cases", "/exec", "/history", "/generation", "/trace", "/target", "/scenarios", "/assistant":
		return page
	}
	return ""
}
func digest(v string) string   { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func int64Text(n int64) string { return strconv.FormatInt(n, 10) }

func deterministicResponse(page, question string, artifactID int64) (string, *Artifact) {
	if page == "/gate" {
		return "我已把当前问题限定在质量门禁上下文。请先核对门禁规则、最近一次执行结果和证据引用；需要处理时点击“带我操作”，我只会打开页面和预填复测范围，最终提交仍由你确认。", &Artifact{ID: artifactID, Kind: "risk", Priority: "high", Title: "质量门禁需要核对", Summary: "当前对话请求涉及质量门禁；请先核对规则和原始执行证据。", Recommendation: "查看门禁依据后，按需创建复测草稿。", Evidence: []string{"页面上下文：质量门禁", "对话摘要已哈希审计"}, Status: "open", CreatedAt: time.Now().UTC()}
	}
	return "我会仅使用当前页面可见、且你有权限访问的事实协助判断。需要进入流程时，请点击“带我操作”；我会给出可见的定位或预填，不会替你提交。", nil
}
func guidanceForPage(page string, gen *id.Generator) []UIAction {
	if page == "/gate" {
		return []UIAction{{ID: gen.Next(), Kind: "navigate", Route: "/gate", Target: "gate.open-rule", Label: "查看门禁规则与证据"}, {ID: gen.Next(), Kind: "navigate", Route: "/cases", Target: "cases.prefill-retest-scope", Label: "前往用例管理并预填复测范围"}, {ID: gen.Next(), Kind: "prefill", Route: "/exec", Target: "exec.open-retest-confirm", Values: map[string]string{"environment": "staging", "scenario": "contract-retest"}, Label: "预填复测草稿（未提交）"}}
	}
	return []UIAction{{ID: gen.Next(), Kind: "navigate", Route: page, Target: "page.current", Label: "回到当前页面"}}
}
func allowed(a UIAction) bool {
	if a.Kind != "navigate" && a.Kind != "focus" && a.Kind != "prefill" && a.Kind != "open_evidence" {
		return false
	}
	return normalizePage(a.Route) != "" && !strings.ContainsAny(a.Route, "?#")
}
