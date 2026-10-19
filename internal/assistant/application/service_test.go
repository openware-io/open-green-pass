package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/pkg/clock"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type testRepository struct {
	session Session
	turns   []Turn
	writes  int
}

func (r *testRepository) CreateSession(context.Context, Session, string) error { return nil }
func (r *testRepository) Session(context.Context, int64) (Session, error)      { return r.session, nil }
func (r *testRepository) AddTurns(_ context.Context, turns []Turn) error {
	r.writes++
	r.turns = append(r.turns, turns...)
	return nil
}
func (r *testRepository) Turns(context.Context, int64) ([]Turn, error)   { return r.turns, nil }
func (r *testRepository) CreateArtifact(context.Context, Artifact) error { return nil }
func (r *testRepository) Artifacts(context.Context) ([]Artifact, error)  { return nil, nil }
func (r *testRepository) CreateAction(context.Context, int64, UIAction, string) (ActionRecord, error) {
	return ActionRecord{}, nil
}
func (r *testRepository) Actions(context.Context, int64) ([]ActionRecord, error) { return nil, nil }
func (r *testRepository) ActionBelongs(context.Context, int64, int64) (bool, error) {
	return true, nil
}

type responderFunc func(context.Context, string, string) (string, string, error)

func (f responderFunc) Respond(ctx context.Context, page, question string) (string, string, error) {
	return f(ctx, page, question)
}

func newTestService(t *testing.T, repo *testRepository, audit AuditPort, responder ModelResponder) *Service {
	t.Helper()
	generator, err := id.New(1, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(repo, audit, generator)
	svc.SetModelResponder(responder)
	return svc
}

func TestAddTurnModelFailureAuditsAttemptWithoutPersistingTurns(t *testing.T) {
	const question = "请检查当前页面风险"
	repo := &testRepository{session: Session{ID: 5, Subject: "42", Page: "/target"}}
	var operations []string
	var payloads []map[string]any
	audit := AuditFunc(func(_ context.Context, _ int64, operation, _ string, _ int64, payload map[string]any) error {
		operations = append(operations, operation)
		payloads = append(payloads, payload)
		return nil
	})
	svc := newTestService(t, repo, audit, responderFunc(func(context.Context, string, string) (string, string, error) {
		return "", "provider:model", errors.New("upstream failed")
	}))

	_, _, _, err := svc.AddTurn(context.Background(), 42, "42", 5, question)
	if !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("error = %v, want ErrModelUnavailable", err)
	}
	if repo.writes != 0 || len(repo.turns) != 0 {
		t.Fatalf("failed request persisted turns: writes=%d turns=%d", repo.writes, len(repo.turns))
	}
	if len(operations) != 1 || operations[0] != "assistant.turn.failed" {
		t.Fatalf("audit operations = %v, want one assistant.turn.failed event", operations)
	}
	if _, exists := payloads[0]["text"]; exists {
		t.Fatal("failed request audit must not contain the question text")
	}
	if payloads[0]["text_sha256"] == "" || payloads[0]["reason"] != "model_unavailable" {
		t.Fatalf("unexpected failed request audit payload: %v", payloads[0])
	}
}

func TestAddTurnPersistsUserAndAssistantTogetherAfterModelSuccess(t *testing.T) {
	repo := &testRepository{session: Session{ID: 5, Subject: "42", Page: "/target"}}
	var operations []string
	audit := AuditFunc(func(_ context.Context, _ int64, operation, _ string, _ int64, _ map[string]any) error {
		operations = append(operations, operation)
		return nil
	})
	svc := newTestService(t, repo, audit, responderFunc(func(context.Context, string, string) (string, string, error) {
		return "基于证据的回答", "provider:model", nil
	}))

	user, assistant, _, err := svc.AddTurn(context.Background(), 42, "42", 5, "请检查当前页面风险")
	if err != nil {
		t.Fatal(err)
	}
	if repo.writes != 1 || len(repo.turns) != 2 || user.Role != "user" || assistant.Role != "assistant" {
		t.Fatalf("turns were not persisted as one pair: writes=%d turns=%+v", repo.writes, repo.turns)
	}
	if !repo.turns[0].CreatedAt.Equal(repo.turns[1].CreatedAt) || repo.turns[0].CreatedAt.After(time.Now().UTC()) {
		t.Fatalf("unexpected timestamps for turn pair: %+v", repo.turns)
	}
	if len(operations) != 2 || operations[0] != "assistant.turn.created" || operations[1] != "assistant.turn.created" {
		t.Fatalf("audit operations = %v, want user and assistant turn events", operations)
	}
}
