// Package runner defines the execution-adapter boundary for heterogeneous
// scenarios. Adapters only translate a case into a reproducible execution
// plan; the caller decides where (Kubernetes, browser pool, device farm) it
// is executed.
package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

const (
	ScenarioAPI          = "api"
	ScenarioWebE2E       = "web_e2e"
	ScenarioWebVisual    = "web_visual"
	ScenarioWebPerf      = "web_performance"
	ScenarioMobileE2E    = "mobile_e2e"
	ScenarioMobileCompat = "mobile_compat"
	ScenarioWeakNetwork  = "weak_network"
	ScenarioPerformance  = "performance"
)

// Plan is the portable execution contract passed to a concrete resource pool.
type Plan struct {
	Scenario    string
	Resource    string
	Image       string
	Command     []string
	Environment []string
}

// Adapter translates one scenario into a plan.
type Adapter interface {
	Name() string
	Plan(*domain.CaseSpec) (Plan, error)
}

// Registry routes scenarios to adapters. It is immutable after construction
// so a worker can safely share it across concurrent runs.
type Registry struct{ adapters map[string]Adapter }

func NewRegistry(adapters ...Adapter) *Registry {
	r := &Registry{adapters: make(map[string]Adapter, len(adapters))}
	for _, a := range adapters {
		if a != nil {
			r.adapters[a.Name()] = a
		}
	}
	return r
}

func DefaultRegistry() *Registry {
	r := NewRegistry(APIAdapter{}, PlaywrightAdapter{}, STFAdapter{}, LoadAdapter{})
	// The same pool adapter serves these closely related scenario variants;
	// their script remains the source of the concrete test command.
	r.adapters[ScenarioWebVisual] = r.adapters[ScenarioWebE2E]
	r.adapters[ScenarioWebPerf] = r.adapters[ScenarioWebE2E]
	r.adapters[ScenarioMobileCompat] = r.adapters[ScenarioMobileE2E]
	r.adapters[ScenarioWeakNetwork] = r.adapters[ScenarioMobileE2E]
	// Aliases used by existing case seeds and the frontend prototype.
	r.adapters["web"] = r.adapters[ScenarioWebE2E]
	r.adapters["visual"] = r.adapters[ScenarioWebVisual]
	r.adapters["mobile"] = r.adapters[ScenarioMobileE2E]
	r.adapters["perf"] = r.adapters[ScenarioPerformance]
	r.adapters["load"] = r.adapters[ScenarioPerformance]
	r.adapters["weaknet"] = r.adapters[ScenarioWeakNetwork]
	return r
}

func (r *Registry) Plan(s *domain.CaseSpec) (Plan, error) {
	if s == nil {
		return Plan{}, errors.New("case spec is nil")
	}
	name := strings.ToLower(strings.TrimSpace(s.Scenario))
	name = strings.ReplaceAll(name, "-", "_")
	if name == "" {
		name = ScenarioAPI
	}
	a, ok := r.adapters[name]
	if !ok {
		return Plan{}, fmt.Errorf("unsupported execution scenario %q", name)
	}
	p, err := a.Plan(s)
	if err != nil {
		return Plan{}, err
	}
	// Preserve the requested variant (visual/perf/compat/weak-network) even
	// when it shares a physical pool adapter with its base scenario.
	p.Scenario = name
	return p, nil
}

// CommandExecutor is injected in tests and by resource-pool integrations.
type CommandExecutor interface {
	Execute(context.Context, Plan) (string, error)
}

// LocalCommandExecutor is a development fallback. Production deployments
// should use a sandbox executor instead of running commands in the server pod.
type LocalCommandExecutor struct{}

func (LocalCommandExecutor) Execute(ctx context.Context, p Plan) (string, error) {
	if len(p.Command) == 0 {
		return "", errors.New("execution plan has no command")
	}
	cmd := exec.CommandContext(ctx, p.Command[0], p.Command[1:]...)
	cmd.Env = append(os.Environ(), p.Environment...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w", p.Command[0], err)
	}
	return string(out), nil
}

// ScenarioRunner turns adapter plans into the existing domain RunnerPort.
type ScenarioRunner struct {
	gen      *id.Generator
	registry *Registry
	exec     CommandExecutor
}

func NewScenarioRunner(gen *id.Generator, registry *Registry, executor CommandExecutor) *ScenarioRunner {
	if registry == nil {
		registry = DefaultRegistry()
	}
	if executor == nil {
		executor = LocalCommandExecutor{}
	}
	return &ScenarioRunner{gen: gen, registry: registry, exec: executor}
}

func (r *ScenarioRunner) Execute(ctx context.Context, specs []*domain.CaseSpec) ([]*domain.CaseResult, error) {
	results := make([]*domain.CaseResult, 0, len(specs))
	for _, s := range specs {
		started := time.Now().UTC()
		plan, err := r.registry.Plan(s)
		if err != nil {
			return results, err
		}
		text, runErr := r.exec.Execute(ctx, plan)
		ended := time.Now().UTC()
		status := domain.CasePass
		if runErr != nil {
			status = domain.CaseFail
		}
		h := sha256.Sum256([]byte(text))
		ev := &domain.EvidenceRef{Logs: []string{fmt.Sprintf("runner://%s/case-%d", plan.Resource, s.CaseID)}, Hash: hex.EncodeToString(h[:])}
		if s.ScreenshotEnabled {
			ev.Screenshots = []string{fmt.Sprintf("runner://%s/case-%d/screenshot", plan.Resource, s.CaseID)}
		}
		res := &domain.CaseResult{ID: r.gen.Next(), CaseID: s.CaseID, CaseVersion: s.CaseVersion, Status: status, ResultText: text, Evidence: ev, AttemptSeq: 1, StartedAt: started, EndedAt: &ended}
		results = append(results, res)
		if runErr != nil {
			return results, runErr
		}
	}
	return results, nil
}

func scriptMap(s any) map[string]any {
	if m, ok := s.(map[string]any); ok {
		return m
	}
	return nil
}

func scriptString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
func scriptCommand(m map[string]any) []string {
	if v, ok := m["command"].([]string); ok {
		return append([]string(nil), v...)
	}
	if v, ok := m["command"].([]any); ok {
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	if raw, ok := m["command"].(string); ok && raw != "" {
		return []string{"sh", "-c", raw}
	}
	return nil
}

type APIAdapter struct{}

func (APIAdapter) Name() string { return ScenarioAPI }
func (APIAdapter) Plan(s *domain.CaseSpec) (Plan, error) {
	m := scriptMap(s.Script)
	cmd := scriptCommand(m)
	if len(cmd) == 0 {
		cmd = []string{"sh", "-c", "echo greenpass-api-ok"}
	}
	image := scriptString(m, "image")
	if image == "" {
		image = "curlimages/curl:8.10.1"
	}
	return Plan{Scenario: ScenarioAPI, Resource: "k8s-sandbox", Image: image, Command: cmd}, nil
}

type PlaywrightAdapter struct{}

func (PlaywrightAdapter) Name() string { return ScenarioWebE2E }
func (PlaywrightAdapter) Plan(s *domain.CaseSpec) (Plan, error) {
	m := scriptMap(s.Script)
	cmd := scriptCommand(m)
	if len(cmd) == 0 {
		cmd = []string{"npx", "playwright", "test"}
	}
	image := scriptString(m, "image")
	if image == "" {
		image = "mcr.microsoft.com/playwright:v1.49.1-noble"
	}
	return Plan{Scenario: ScenarioWebE2E, Resource: "browser", Image: image, Command: cmd}, nil
}

type STFAdapter struct{}

func (STFAdapter) Name() string { return ScenarioMobileE2E }
func (STFAdapter) Plan(s *domain.CaseSpec) (Plan, error) {
	m := scriptMap(s.Script)
	cmd := scriptCommand(m)
	if len(cmd) == 0 {
		cmd = []string{"sh", "-c", "echo greenpass-mobile-ok"}
	}
	image := scriptString(m, "image")
	if image == "" {
		image = "appium/appium:2.5.1"
	}
	return Plan{Scenario: ScenarioMobileE2E, Resource: "device", Image: image, Command: cmd}, nil
}

type LoadAdapter struct{}

func (LoadAdapter) Name() string { return ScenarioPerformance }
func (LoadAdapter) Plan(s *domain.CaseSpec) (Plan, error) {
	m := scriptMap(s.Script)
	tool := scriptString(m, "tool")
	if tool == "" {
		tool = "k6"
	}
	cmd := scriptCommand(m)
	if len(cmd) == 0 {
		cmd = []string{tool, "run", "/workspace/script.js"}
	}
	image := scriptString(m, "image")
	if image == "" {
		image = "grafana/k6:0.54.0"
	}
	return Plan{Scenario: ScenarioPerformance, Resource: "load", Image: image, Command: cmd}, nil
}

// Keep JSON script values useful to callers while retaining a strict adapter API.
func ParseScript(raw []byte) (any, error) { var v any; err := json.Unmarshal(raw, &v); return v, err }

var _ domain.RunnerPort = (*ScenarioRunner)(nil)
