package infra

import (
	"strings"
	"testing"
)

func validGeneratedCase() generatedCase {
	return generatedCase{
		Title: "健康检查接口返回成功", Kind: "api", Preconditions: []string{"服务已启动"},
		Steps: []string{"发送 GET /health 请求"}, Expected: "响应状态码为 HTTP 200", SourceLocator: "openapi.yaml", Scenario: "api",
		Request:    generatedRequest{Method: "GET", Path: "/health", Headers: map[string]string{"accept": "application/json"}},
		Assertions: []generatedAssert{{Type: "status", ExpectedStatus: 200}, {Type: "body_contains", Value: "ok"}},
	}
}

func TestValidateGeneratedCaseRejectsNonChineseReviewDescriptions(t *testing.T) {
	candidate := validGeneratedCase()
	candidate.Title = "health endpoint returns success"
	if err := validateGeneratedCase(candidate); err == nil || !strings.Contains(err.Error(), "Simplified Chinese") {
		t.Fatalf("expected non-Chinese title rejection, got %v", err)
	}
}

func TestDecodeGeneratedCasesRejectsExecutableShellFields(t *testing.T) {
	_, err := decodeGeneratedCases(`{"cases":[{"title":"x","kind":"api","preconditions":[],"steps":["GET /health"],"expected":"200","source_locator":"openapi.yaml","scenario":"api","request":{"method":"GET","path":"/health","headers":{},"body":null},"assertions":[{"type":"status","expected_status":200}],"command":"echo pwned"}]}`)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown executable field to be rejected, got %v", err)
	}
}

func TestValidateGeneratedCaseAllowsNegativeStatusButRejectsRemoteHost(t *testing.T) {
	candidate := validGeneratedCase()
	candidate.Assertions[0].ExpectedStatus = 404
	if err := validateGeneratedCase(candidate); err != nil {
		t.Fatalf("negative HTTP assertions should be executable: %v", err)
	}
	candidate.Request.Path = "https://attacker.example/collect"
	if err := validateGeneratedCase(candidate); err == nil {
		t.Fatal("expected absolute URL to be rejected")
	}
}

func TestDecodeGeneratedCasesRejectsTrailingJSON(t *testing.T) {
	_, err := decodeGeneratedCases(`{"cases":[]} {"cases":[]}`)
	if err == nil {
		t.Fatal("expected trailing JSON value to be rejected")
	}
}
