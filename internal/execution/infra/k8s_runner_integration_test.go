//go:build integration

// gp1-04-b 集成测试：K8s Job Runner 在 gp-runner 沙箱真实建 Job 执行并回收。
package infra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// TestK8sRunner 在 kind gp-runner 命名空间用声明式 HTTP 用例建 Job，验证真实请求、断言、证据哈希与回收。
func TestK8sRunner(t *testing.T) {
	kc := os.Getenv("KUBECONFIG")
	if kc == "" {
		home, _ := os.UserHomeDir()
		kc = filepath.Join(home, ".kube", "config")
	}
	if _, err := os.Stat(kc); err != nil {
		t.Skip("no kubeconfig, skip k8s runner")
	}
	gen, _ := id.New(1, nil)
	r, err := NewK8sRunner(gen, K8sRunnerConfig{Kubeconfig: kc, Namespace: "gp-runner", Timeout: 60 * time.Second})
	if err != nil {
		t.Fatalf("new k8s runner: %v", err)
	}
	ctx := context.Background()
	spec := []*domain.CaseSpec{{
		CaseID: 4001, CaseVersion: 1, TargetID: 1003, Env: "test",
		BaseURL: "http://gp-server.gp.svc.cluster.local:8080", RequiresSandbox: true,
		Script: map[string]any{
			"scenario": "api",
			"request": map[string]any{
				"method": "GET", "path": "/healthz",
				"headers": map[string]any{"Accept": "application/json"},
			},
			"assertions": []any{
				map[string]any{"type": "status", "expected_status": float64(200)},
				map[string]any{"type": "body_contains", "value": "ok"},
			},
		},
	}}
	results, err := r.Execute(ctx, spec)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("want 1 result got %d", len(results))
	}
	if results[0].Status != domain.CasePass {
		t.Fatalf("want pass got %s text=%q", results[0].Status, results[0].ResultText)
	}
	if results[0].Evidence == nil || results[0].Evidence.Hash == "" {
		t.Fatalf("want evidence hash, got %+v", results[0].Evidence)
	}
	if len(results[0].Evidence.Screenshots) != 0 {
		t.Fatalf("API runner must not publish synthetic screenshots: %+v", results[0].Evidence.Screenshots)
	}
	if !strings.Contains(results[0].ResultText, "assertions=passed") {
		t.Fatalf("want declarative HTTP assertions to pass, got text=%q", results[0].ResultText)
	}
	t.Logf("k8s runner ok: status=%s hash=%s", results[0].Status, results[0].Evidence.Hash)
}
