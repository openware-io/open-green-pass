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

// TestK8sRunner 在 kind gp-runner 命名空间建 busybox Job，验证 pass + 证据哈希 + 回收。
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
		Script: map[string]any{"image": "curlimages/curl:8.10.1", "command": []any{"sh", "-c", "curl --fail --silent http://gateway.open-im-local.svc.cluster.local:3002/actuator/health && echo gp-runner-ok"}},
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
	if !strings.Contains(results[0].ResultText, "gp-runner-ok") {
		t.Fatalf("want log captured got text=%q", results[0].ResultText)
	}
	t.Logf("k8s runner ok: status=%s hash=%s", results[0].Status, results[0].Evidence.Hash)
}
