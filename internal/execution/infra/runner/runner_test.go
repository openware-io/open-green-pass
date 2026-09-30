package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

type recordingExecutor struct {
	plans []Plan
	err   error
}

func (e *recordingExecutor) Execute(_ context.Context, p Plan) (string, error) {
	e.plans = append(e.plans, p)
	return "runner output", e.err
}

func testGen(t *testing.T) *id.Generator {
	t.Helper()
	g, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestRegistryRoutesHeterogeneousScenarios(t *testing.T) {
	r := DefaultRegistry()
	cases := []struct{ scenario, resource, command string }{
		{ScenarioAPI, "k8s-sandbox", "sh"},
		{ScenarioWebE2E, "browser", "npx"},
		{ScenarioMobileE2E, "device", "sh"},
		{ScenarioPerformance, "load", "k6"},
	}
	for _, tc := range cases {
		p, err := r.Plan(&domain.CaseSpec{Scenario: tc.scenario})
		if err != nil {
			t.Fatalf("%s: %v", tc.scenario, err)
		}
		if p.Resource != tc.resource || p.Command[0] != tc.command {
			t.Fatalf("%s: got resource=%s command=%v", tc.scenario, p.Resource, p.Command)
		}
	}
}

func TestRegistryRejectsUnknownScenario(t *testing.T) {
	_, err := DefaultRegistry().Plan(&domain.CaseSpec{Scenario: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unsupported execution scenario") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScenarioRunnerProducesEvidenceAndFailure(t *testing.T) {
	exec := &recordingExecutor{err: errors.New("boom")}
	r := NewScenarioRunner(testGen(t), DefaultRegistry(), exec)
	res, err := r.Execute(context.Background(), []*domain.CaseSpec{{CaseID: 7, Scenario: ScenarioWebE2E, ScreenshotEnabled: true}})
	if err == nil || len(res) != 1 {
		t.Fatalf("expected one failed result, got results=%d err=%v", len(res), err)
	}
	if res[0].Status != domain.CaseFail || res[0].Evidence.Hash == "" || len(res[0].Evidence.Screenshots) != 1 {
		t.Fatalf("bad result: %+v", res[0])
	}
	if len(exec.plans) != 1 || exec.plans[0].Resource != "browser" {
		t.Fatalf("bad plan: %+v", exec.plans)
	}
}

func TestParseScript(t *testing.T) {
	v, err := ParseScript([]byte(`{"scenario":"performance","command":"k6 run script.js"}`))
	if err != nil || v == nil {
		t.Fatalf("parse script: %v", err)
	}
}
