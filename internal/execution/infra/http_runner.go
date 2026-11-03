// Package infra 执行器：受控 HTTP Runner（无 K8s Job 依赖，便于本地/kind 真实执行）。
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
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// HTTPRunner executes declarative API cases with the standard library.
type HTTPRunner struct {
	gen         *id.Generator
	client      *http.Client
	bearerToken string
}

// NewHTTPRunner creates a bounded HTTP runner. bearerToken is optional and is
// injected only into requests that do not set Authorization explicitly.
func NewHTTPRunner(gen *id.Generator, bearerToken string) *HTTPRunner {
	return &HTTPRunner{
		gen:         gen,
		client:      &http.Client{Timeout: 60 * time.Second},
		bearerToken: strings.TrimSpace(bearerToken),
	}
}

// Execute runs each declarative case once and records status/evidence.
func (r *HTTPRunner) Execute(ctx context.Context, specs []*domain.CaseSpec) ([]*domain.CaseResult, error) {
	results := make([]*domain.CaseResult, 0, len(specs))
	for _, spec := range specs {
		if err := ctx.Err(); err != nil {
			return results, err
		}
		result := r.executeOne(ctx, spec)
		results = append(results, result)
		if err := ctx.Err(); err != nil {
			return results, err
		}
	}
	return results, nil
}

//nolint:gocyclo // legacy complexity; refactor tracked in lint backlog
func (r *HTTPRunner) executeOne(ctx context.Context, spec *domain.CaseSpec) *domain.CaseResult {
	now := time.Now().UTC()
	result := &domain.CaseResult{
		ID: r.gen.Next(), CaseID: spec.CaseID, CaseVersion: spec.CaseVersion,
		StartedAt: now, EndedAt: &now, Evidence: &domain.EvidenceRef{},
	}
	fail := func(message string) *domain.CaseResult {
		result.Status = domain.CaseFail
		result.ResultText = message
		result.Evidence.Logs = []string{message}
		ended := time.Now().UTC()
		result.EndedAt = &ended
		return result
	}
	root, ok := spec.Script.(map[string]any)
	if !ok {
		return fail("case script is not a JSON object")
	}
	if reason, blocked := root["blocked_reason"].(string); blocked && strings.TrimSpace(reason) != "" {
		result.Status = domain.CaseBlocked
		result.ResultText = reason
		result.Evidence.Logs = []string{reason}
		ended := time.Now().UTC()
		result.EndedAt = &ended
		return result
	}
	request, ok := root["request"].(map[string]any)
	if !ok {
		// Backward compatibility for the first generated case format:
		// {"path": "...", "method": "GET", "expected_status": 200}.
		legacyPath, _ := root["path"].(string)
		legacyMethod, _ := root["method"].(string)
		if strings.TrimSpace(legacyPath) == "" || strings.TrimSpace(legacyMethod) == "" {
			return fail("case has no declarative request")
		}
		request = normalizeLegacyRequest(root)
	}
	method, _ := request["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return fail("unsupported HTTP method")
	}
	path, _ := request["path"].(string)
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "://") {
		return fail("request path must be relative")
	}
	target := strings.TrimRight(spec.BaseURL, "/") + path
	if _, err := url.ParseRequestURI(target); err != nil {
		return fail("invalid request URL")
	}
	var body io.Reader
	if rawBody, exists := request["body"]; exists {
		encoded, err := json.Marshal(rawBody)
		if err != nil || len(encoded) > 16*1024 {
			return fail("request body invalid or too large")
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return fail("build request failed")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if headers, ok := request["headers"].(map[string]any); ok {
		for name, value := range headers {
			text, ok := value.(string)
			if !ok || strings.ContainsAny(name, "\r\n") || strings.ContainsAny(text, "\r\n") {
				return fail("invalid request header")
			}
			if strings.HasPrefix(text, "env:") {
				envName := strings.TrimPrefix(text, "env:")
				if !strings.HasPrefix(envName, "GP_CASE_") {
					return fail("header env reference must use GP_CASE_ prefix")
				}
				text = os.Getenv(envName)
				if text == "" {
					return fail("header env reference is empty")
				}
			}
			req.Header.Set(name, text)
		}
	}
	if r.bearerToken != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+r.bearerToken)
	}
	response, err := r.client.Do(req)
	if err != nil {
		return fail("request failed: " + err.Error())
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	result.ResultText = fmt.Sprintf("http_status=%d body=%s", response.StatusCode, truncate(string(payload), 512))
	hash := sha256.Sum256(payload)
	result.Evidence.Hash = "sha256:" + hex.EncodeToString(hash[:])
	result.Evidence.Logs = []string{fmt.Sprintf("http_status=%d", response.StatusCode)}
	passed := true
	assertions, _ := root["assertions"].([]any)
	for _, raw := range assertions {
		assertion, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch assertion["type"] {
		case "status":
			expected, ok := numberToInt(assertion["expected_status"])
			if !ok || response.StatusCode != expected {
				passed = false
			}
		case "body_contains":
			value, _ := assertion["value"].(string)
			if value != "" && !strings.Contains(string(payload), value) {
				passed = false
			}
		}
	}
	if passed {
		result.Status = domain.CasePass
	} else {
		result.Status = domain.CaseFail
	}
	ended := time.Now().UTC()
	result.EndedAt = &ended
	return result
}

func truncate(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	return value[:maxLen] + "..."
}

func numberToInt(value any) (int, bool) {
	switch n := value.(type) {
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case int:
		return n, true
	case int64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}

var _ domain.RunnerPort = (*HTTPRunner)(nil)

// normalizeLegacyRequest converts the first generated case shape into the
// declarative shape used by the runner. It intentionally does not resolve
// placeholder IDs or invent request bodies.
func normalizeLegacyRequest(root map[string]any) map[string]any {
	path, _ := root["path"].(string)
	method, _ := root["method"].(string)
	request := map[string]any{"path": path, "method": method}
	if _, hasAssertions := root["assertions"]; !hasAssertions {
		if expected, valid := numberToInt(root["expected_status"]); valid {
			root["assertions"] = []any{map[string]any{"type": "status", "expected_status": expected}}
		}
	}
	return request
}
