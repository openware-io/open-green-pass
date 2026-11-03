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

func (r *HTTPRunner) executeOne(ctx context.Context, spec *domain.CaseSpec) *domain.CaseResult {
	now := time.Now().UTC()
	result := &domain.CaseResult{
		ID: r.gen.Next(), CaseID: spec.CaseID, CaseVersion: spec.CaseVersion,
		StartedAt: now, EndedAt: &now, Evidence: &domain.EvidenceRef{},
	}
	root, ok := spec.Script.(map[string]any)
	if !ok {
		return failCaseResult(result, "case script is not a JSON object")
	}
	if result, blocked := blockedCaseResult(result, root); blocked {
		return result
	}
	request, err := declarativeRequest(root)
	if err != nil {
		return failCaseResult(result, err.Error())
	}
	method, err := requestMethod(request)
	if err != nil {
		return failCaseResult(result, err.Error())
	}
	target, err := requestTarget(spec.BaseURL, request)
	if err != nil {
		return failCaseResult(result, err.Error())
	}
	body, err := requestBody(request)
	if err != nil {
		return failCaseResult(result, err.Error())
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return failCaseResult(result, "build request failed")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := applyRequestHeaders(req, request["headers"]); err != nil {
		return failCaseResult(result, err.Error())
	}
	if r.bearerToken != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+r.bearerToken)
	}
	return r.executeRequest(req, root, result)
}

func failCaseResult(result *domain.CaseResult, message string) *domain.CaseResult {
	result.Status = domain.CaseFail
	result.ResultText = message
	result.Evidence.Logs = []string{message}
	ended := time.Now().UTC()
	result.EndedAt = &ended
	return result
}

// blockedCaseResult returns a blocked result when the case script declares a
// non-empty blocked_reason (e.g. an unconfirmed environment baseline).
func blockedCaseResult(result *domain.CaseResult, root map[string]any) (*domain.CaseResult, bool) {
	reason, blocked := root["blocked_reason"].(string)
	if !blocked || strings.TrimSpace(reason) == "" {
		return result, false
	}
	result.Status = domain.CaseBlocked
	result.ResultText = reason
	result.Evidence.Logs = []string{reason}
	ended := time.Now().UTC()
	result.EndedAt = &ended
	return result, true
}

func declarativeRequest(root map[string]any) (map[string]any, error) {
	request, ok := root["request"].(map[string]any)
	if ok {
		return request, nil
	}
	// Backward compatibility for the first generated case format:
	// {"path": "...", "method": "GET", "expected_status": 200}.
	legacyPath, _ := root["path"].(string)
	legacyMethod, _ := root["method"].(string)
	if strings.TrimSpace(legacyPath) == "" || strings.TrimSpace(legacyMethod) == "" {
		return nil, fmt.Errorf("case has no declarative request")
	}
	return normalizeLegacyRequest(root), nil
}

func requestMethod(request map[string]any) (string, error) {
	method, _ := request["method"].(string)
	method = strings.ToUpper(strings.TrimSpace(method))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return method, nil
	default:
		return "", fmt.Errorf("unsupported HTTP method")
	}
}

func requestTarget(baseURL string, request map[string]any) (string, error) {
	path, _ := request["path"].(string)
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "://") {
		return "", fmt.Errorf("request path must be relative")
	}
	target := strings.TrimRight(baseURL, "/") + path
	if _, err := url.ParseRequestURI(target); err != nil {
		return "", fmt.Errorf("invalid request URL")
	}
	return target, nil
}

func requestBody(request map[string]any) (io.Reader, error) {
	rawBody, exists := request["body"]
	if !exists {
		return nil, nil
	}
	encoded, err := json.Marshal(rawBody)
	if err != nil || len(encoded) > 16*1024 {
		return nil, fmt.Errorf("request body invalid or too large")
	}
	return bytes.NewReader(encoded), nil
}

func applyRequestHeaders(req *http.Request, raw any) error {
	headers, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	for name, value := range headers {
		text, ok := value.(string)
		if !ok || strings.ContainsAny(name, "\r\n") || strings.ContainsAny(text, "\r\n") {
			return fmt.Errorf("invalid request header")
		}
		if strings.HasPrefix(text, "env:") {
			resolved, err := resolveHeaderEnvReference(text)
			if err != nil {
				return err
			}
			text = resolved
		}
		req.Header.Set(name, text)
	}
	return nil
}

func resolveHeaderEnvReference(value string) (string, error) {
	envName := strings.TrimPrefix(value, "env:")
	if !strings.HasPrefix(envName, "GP_CASE_") {
		return "", fmt.Errorf("header env reference must use GP_CASE_ prefix")
	}
	text := os.Getenv(envName)
	if text == "" {
		return "", fmt.Errorf("header env reference is empty")
	}
	return text, nil
}

func (r *HTTPRunner) executeRequest(req *http.Request, root map[string]any, result *domain.CaseResult) *domain.CaseResult {
	response, err := r.client.Do(req)
	if err != nil {
		return failCaseResult(result, "request failed: "+err.Error())
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	result.ResultText = fmt.Sprintf("http_status=%d body=%s", response.StatusCode, truncate(string(payload), 512))
	hash := sha256.Sum256(payload)
	result.Evidence.Hash = "sha256:" + hex.EncodeToString(hash[:])
	result.Evidence.Logs = []string{fmt.Sprintf("http_status=%d", response.StatusCode)}
	if evaluateAssertions(root["assertions"], response.StatusCode, payload) {
		result.Status = domain.CasePass
	} else {
		result.Status = domain.CaseFail
	}
	ended := time.Now().UTC()
	result.EndedAt = &ended
	return result
}

func evaluateAssertions(raw any, statusCode int, payload []byte) bool {
	passed := true
	assertions, _ := raw.([]any)
	for _, rawAssertion := range assertions {
		assertion, ok := rawAssertion.(map[string]any)
		if !ok {
			continue
		}
		switch assertion["type"] {
		case "status":
			expected, ok := numberToInt(assertion["expected_status"])
			if !ok || statusCode != expected {
				passed = false
			}
		case "body_contains":
			value, _ := assertion["value"].(string)
			if value != "" && !strings.Contains(string(payload), value) {
				passed = false
			}
		}
	}
	return passed
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
