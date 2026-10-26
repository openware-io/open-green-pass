package infra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// BrowserRunner executes declarative browser cases through the managed
// Playwright runner. It never reports a pass unless the browser step sequence
// actually completed and its assertions held.
type BrowserRunner struct {
	gen      *id.Generator
	endpoint string
	client   *http.Client
}

type browserRequest struct {
	BaseURL string `json:"base_url"`
	Script  any    `json:"script"`
}

type browserResponse struct {
	Status      string   `json:"status"`
	Logs        []string `json:"logs"`
	Screenshots []string `json:"screenshots"`
	FinalURL    string   `json:"final_url"`
	Title       string   `json:"title"`
}

// NewBrowserRunner creates a browser runner bound to a managed Playwright
// service. An empty endpoint yields a runner that blocks cases instead of
// silently falling back to HTTP.
func NewBrowserRunner(gen *id.Generator, endpoint string) *BrowserRunner {
	return &BrowserRunner{gen: gen, endpoint: strings.TrimRight(strings.TrimSpace(endpoint), "/"), client: &http.Client{Timeout: 5 * time.Minute}}
}

func (r *BrowserRunner) Execute(ctx context.Context, specs []*domain.CaseSpec) ([]*domain.CaseResult, error) {
	results := make([]*domain.CaseResult, 0, len(specs))
	for _, spec := range specs {
		results = append(results, r.executeOne(ctx, spec))
	}
	return results, nil
}

func (r *BrowserRunner) executeOne(ctx context.Context, spec *domain.CaseSpec) *domain.CaseResult {
	now := time.Now().UTC()
	result := &domain.CaseResult{ID: r.gen.Next(), CaseID: spec.CaseID, CaseVersion: spec.CaseVersion, Evidence: &domain.EvidenceRef{}, AttemptSeq: 1, StartedAt: now}
	finish := func(status domain.CaseStatus, text string, logs []string) *domain.CaseResult {
		ended := time.Now().UTC()
		result.Status, result.ResultText, result.EndedAt = status, text, &ended
		result.Evidence.Logs = append(result.Evidence.Logs, logs...)
		return result
	}
	if r.endpoint == "" {
		return finish(domain.CaseBlocked, "browser runner is not configured", []string{"GP_BROWSER_RUNNER_URL is empty"})
	}
	root, _ := spec.Script.(map[string]any)
	baseURL := spec.BaseURL
	if configured, ok := root["base_url"].(string); ok && strings.TrimSpace(configured) != "" {
		baseURL = configured
	}
	payload, err := json.Marshal(browserRequest{BaseURL: baseURL, Script: spec.Script})
	if err != nil {
		return finish(domain.CaseFail, "encode browser case failed", []string{err.Error()})
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint+"/execute", bytes.NewReader(payload))
	if err != nil {
		return finish(domain.CaseFail, "build browser request failed", []string{err.Error()})
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := r.client.Do(request)
	if err != nil {
		return finish(domain.CaseFail, "browser runner unavailable", []string{err.Error()})
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return finish(domain.CaseFail, "browser runner rejected the case", []string{fmt.Sprintf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))})
	}
	var parsed browserResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return finish(domain.CaseFail, "invalid browser runner response", []string{err.Error()})
	}
	logs := parsed.Logs
	logs = append(logs, fmt.Sprintf("final_url=%s title=%s screenshots=%d", parsed.FinalURL, parsed.Title, len(parsed.Screenshots)))
	switch parsed.Status {
	case "pass":
		result.Evidence.Hash = evidenceHash(logs)
		result.Evidence.Screenshots = parsed.Screenshots
		return finish(domain.CasePass, "browser case passed", logs)
	case "blocked":
		return finish(domain.CaseBlocked, "browser case blocked", logs)
	default:
		return finish(domain.CaseFail, "browser case failed", logs)
	}
}

func evidenceHash(logs []string) string {
	sum := sha256.Sum256([]byte(strings.Join(logs, "\n")))
	return hex.EncodeToString(sum[:])
}

var _ domain.RunnerPort = (*BrowserRunner)(nil)
