package infra

import (
	"context"
	"strings"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// HybridRunner routes each case to the adapter that can actually execute its
// execution surface. API cases stay on HTTP; browser cases go to Playwright.
type HybridRunner struct {
	http    *HTTPRunner
	browser *BrowserRunner
}

// NewHybridRunner wires both adapters. A missing browser endpoint is allowed:
// browser cases are blocked per case instead of being silently executed as
// HTTP.
func NewHybridRunner(gen *id.Generator, bearerToken, browserEndpoint string) *HybridRunner {
	return &HybridRunner{
		http:    NewHTTPRunner(gen, bearerToken),
		browser: NewBrowserRunner(gen, browserEndpoint),
	}
}

func (r *HybridRunner) Execute(ctx context.Context, specs []*domain.CaseSpec) ([]*domain.CaseResult, error) {
	results := make([]*domain.CaseResult, 0, len(specs))
	for _, spec := range specs {
		if browserScenario(spec) {
			next, err := r.browser.Execute(ctx, []*domain.CaseSpec{spec})
			if err != nil {
				return results, err
			}
			results = append(results, next...)
			continue
		}
		next, err := r.http.Execute(ctx, []*domain.CaseSpec{spec})
		if err != nil {
			return results, err
		}
		results = append(results, next...)
	}
	return results, nil
}

func browserScenario(spec *domain.CaseSpec) bool {
	scenario := strings.ToLower(strings.TrimSpace(spec.Scenario))
	if strings.HasPrefix(scenario, "web") || scenario == "browser" {
		return true
	}
	if root, ok := spec.Script.(map[string]any); ok {
		if surface, _ := root["execution_surface"].(string); strings.EqualFold(surface, "browser") {
			return true
		}
		if _, ok := root["steps"].([]any); ok {
			return true
		}
	}
	return false
}

var _ domain.RunnerPort = (*HybridRunner)(nil)
