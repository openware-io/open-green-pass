package infra

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
)

type generatedCase struct {
	Title         string            `json:"title"`
	Kind          string            `json:"kind"`
	Preconditions []string          `json:"preconditions"`
	Steps         []string          `json:"steps"`
	Expected      string            `json:"expected"`
	SourceLocator string            `json:"source_locator"`
	Scenario      string            `json:"scenario"`
	Request       generatedRequest  `json:"request"`
	Assertions    []generatedAssert `json:"assertions"`
}

type generatedRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    any               `json:"body"`
}

type generatedAssert struct {
	Type           string `json:"type"`
	ExpectedStatus int    `json:"expected_status,omitempty"`
	Value          string `json:"value,omitempty"`
}

type generatedCaseEnvelope struct {
	Cases []generatedCase `json:"cases"`
}

// generatedCasesInstructions is deliberately Chinese because this deployment's
// configured system language is Simplified Chinese.  The generated artifact is
// shown to reviewers, so its human-readable fields must not silently follow
// the model's default language (HTTP methods, paths and JSON keys stay intact).
const generatedCasesInstructions = `请只返回一个 JSON 对象，且只包含 "cases" 数组；不要输出其他字段或说明。生成 3 到 8 条不同的用例。若提供的源码不足以支持 3 条可执行 HTTP 用例，返回 {"cases":[]}，不得编造端点或行为。
Every case must have precisely these keys and types:
- title: 简短、非空的中文标题。
- kind: the literal string "api" (not the service kind).
- scenario: the literal string "api".
- preconditions: 描述准备条件的简短中文字符串数组；没有条件时使用 []。
- steps: 非空的简短中文步骤数组，不得是对象或命令。可保留 HTTP 方法、路径和 JSON 字段等必要技术标识，例如“发送 GET /health 请求”。
- expected: 描述断言的非空中文字符串。
- source_locator: the exact, case-sensitive path of ONE supplied source file. Do not add line numbers, anchors, a URL, or a prefix.
- request: an object with method, path, headers, body. Allowed methods: GET, POST, PUT, PATCH, DELETE. path must start with one slash and be relative to the operator-configured environment host. Never include a scheme, host, credentials, cookies, tokens, shell syntax, or unresolved path placeholders. headers must be an object containing only Accept, Content-Type, or X-API-Version with string values; use {} when none is needed. body must be JSON or null.
- assertions: an array containing exactly one {"type":"status","expected_status":integer} and optionally {"type":"body_contains","value":"short literal"}. Use no other assertion fields or types.
只使用已提供源码能够佐证的路由、方法、载荷和预期行为。优先选择不依赖密钥或预置标识的数据；不得宣称用例已经执行。示例仅展示 JSON 形状，除非源码支持，否则不得生成 /example：{"cases":[{"title":"健康检查接口返回成功","kind":"api","scenario":"api","preconditions":[],"steps":["发送 GET /health 请求"],"expected":"响应状态码为 HTTP 200","source_locator":"exact/path/from/supplied/files","request":{"method":"GET","path":"/health","headers":{"Accept":"application/json"},"body":null},"assertions":[{"type":"status","expected_status":200}]}]}`

var safeHTTPHeader = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{0,63}$`)

func decodeGeneratedCases(raw string) ([]generatedCase, error) {
	value := strings.TrimSpace(raw)
	if strings.HasPrefix(value, "```") {
		value = strings.TrimPrefix(value, "```")
		if newline := strings.IndexByte(value, '\n'); newline >= 0 {
			value = value[newline+1:]
		}
		value = strings.TrimSuffix(strings.TrimSpace(value), "```")
	}
	var envelope generatedCaseEnvelope
	decoder := json.NewDecoder(bytes.NewBufferString(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("model returned invalid JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("model response must contain exactly one JSON value")
	}
	if envelope.Cases == nil {
		return nil, fmt.Errorf("model response must contain cases array")
	}
	return envelope.Cases, nil
}

//nolint:gocyclo // legacy complexity; refactor tracked in lint backlog
func validateGeneratedCase(candidate generatedCase) error {
	if strings.TrimSpace(candidate.Title) == "" || len([]rune(candidate.Title)) > 200 {
		return fmt.Errorf("title is required and must be at most 200 characters")
	}
	if !containsHan(candidate.Title) {
		return fmt.Errorf("title must use the configured Simplified Chinese description language")
	}
	if strings.TrimSpace(candidate.SourceLocator) == "" {
		return fmt.Errorf("source_locator is required")
	}
	if candidate.Kind != "api" {
		return fmt.Errorf("only api test cases are supported in this generation slice")
	}
	if candidate.Scenario != "api" {
		return fmt.Errorf("only api scenario is executable in the first generation slice")
	}
	method := strings.ToUpper(strings.TrimSpace(candidate.Request.Method))
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return fmt.Errorf("request method is not allowed")
	}
	if err := validateRelativeAPIPath(candidate.Request.Path); err != nil {
		return err
	}
	if len(candidate.Request.Headers) > 8 {
		return fmt.Errorf("too many request headers")
	}
	for key, value := range candidate.Request.Headers {
		if !safeHTTPHeader.MatchString(key) || (!strings.EqualFold(key, "Accept") && !strings.EqualFold(key, "Content-Type") && !strings.EqualFold(key, "X-API-Version")) {
			return fmt.Errorf("request header %q is not allowed", key)
		}
		if len(value) > 512 || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("request header %q is invalid", key)
		}
	}
	if candidate.Request.Body != nil {
		raw, err := json.Marshal(candidate.Request.Body)
		if err != nil || len(raw) > 16*1024 {
			return fmt.Errorf("request body is too large or invalid")
		}
	}
	statusCount := 0
	for _, assertion := range candidate.Assertions {
		switch assertion.Type {
		case "status":
			statusCount++
			if assertion.ExpectedStatus < 100 || assertion.ExpectedStatus > 599 {
				return fmt.Errorf("status assertion must be between 100 and 599")
			}
		case "body_contains":
			if strings.TrimSpace(assertion.Value) == "" || len(assertion.Value) > 256 {
				return fmt.Errorf("body_contains assertion is invalid")
			}
		default:
			return fmt.Errorf("assertion type %q is not allowed", assertion.Type)
		}
	}
	if statusCount != 1 {
		return fmt.Errorf("exactly one status assertion is required")
	}
	if len(candidate.Steps) == 0 || len(candidate.Steps) > 20 || strings.TrimSpace(candidate.Expected) == "" {
		return fmt.Errorf("steps and expected result are required")
	}
	if !containsHan(candidate.Expected) {
		return fmt.Errorf("expected result must use the configured Simplified Chinese description language")
	}
	if len(candidate.Preconditions) > 20 || len(candidate.Expected) > 2000 {
		return fmt.Errorf("preconditions or expected result exceed the configured size limit")
	}
	for _, step := range candidate.Steps {
		if strings.TrimSpace(step) == "" || len(step) > 1000 {
			return fmt.Errorf("steps must be non-empty and at most 1000 characters")
		}
		if !containsHan(step) {
			return fmt.Errorf("steps must use the configured Simplified Chinese description language")
		}
	}
	for _, precondition := range candidate.Preconditions {
		if strings.TrimSpace(precondition) == "" || len(precondition) > 1000 {
			return fmt.Errorf("preconditions must be non-empty and at most 1000 characters")
		}
		if !containsHan(precondition) {
			return fmt.Errorf("preconditions must use the configured Simplified Chinese description language")
		}
	}
	return nil
}

func containsHan(value string) bool {
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

func validateRelativeAPIPath(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" || len(value) > 2048 || !strings.HasPrefix(value, "/") || strings.ContainsAny(value, "\\\r\n\x00#;$`") || strings.Contains(value, "//") || strings.Contains(value, "..") {
		return fmt.Errorf("request path must be a safe repository-relative API path")
	}
	u, err := url.Parse(value)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil {
		return fmt.Errorf("request path cannot contain a host or credentials")
	}
	return nil
}

func generatedCaseMap(candidate generatedCase, batchID int64, provider, model, sourcePath string) map[string]any {
	request := map[string]any{
		"method":  strings.ToUpper(candidate.Request.Method),
		"path":    candidate.Request.Path,
		"headers": candidate.Request.Headers,
		"body":    candidate.Request.Body,
	}
	assertions := make([]map[string]any, 0, len(candidate.Assertions))
	for _, assertion := range candidate.Assertions {
		row := map[string]any{"type": assertion.Type}
		if assertion.ExpectedStatus != 0 {
			row["expected_status"] = assertion.ExpectedStatus
		}
		if assertion.Value != "" {
			row["value"] = assertion.Value
		}
		assertions = append(assertions, row)
	}
	return map[string]any{
		"title": candidate.Title, "kind": candidate.Kind, "preconditions": candidate.Preconditions,
		"steps": candidate.Steps, "expected": candidate.Expected, "source_locator": candidate.SourceLocator,
		"scenario": "api", "request": request, "assertions": assertions,
		"batch_id": batchID, "provider": provider, "model": model, "source_path": sourcePath,
	}
}

func seedDigest(spec map[string]any) string {
	content, _ := json.Marshal(spec)
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// generatedCaseKey is stable across iterations. It captures the request intent
// while leaving presentation and expected behavior mutable, so a corrected
// assertion becomes an update instead of an archive-plus-add.
func generatedCaseKey(spec map[string]any) string {
	request, _ := spec["request"].(map[string]any)
	identity := map[string]any{
		"scenario": spec["scenario"], "method": request["method"], "path": request["path"],
		"headers": request["headers"], "body": request["body"],
	}
	content, _ := json.Marshal(identity)
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}
