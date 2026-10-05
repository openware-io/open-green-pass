package infra

import (
	"context"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

func TestK8sRunnerJobIsTokenlessAndNonRoot(t *testing.T) {
	gen, err := id.New(1, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewSimpleClientset()
	var created *batchv1.Job
	client.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		created = action.(k8stesting.CreateAction).GetObject().(*batchv1.Job).DeepCopy()
		return false, nil, nil
	})
	r := &K8sRunner{gen: gen, cl: client, cfg: K8sRunnerConfig{Namespace: "gp-runner", Image: "curlimages/curl:8.10.1", ServiceAccountName: "gp-runner-job", Timeout: time.Minute}}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	spec := &domain.CaseSpec{
		CaseID:            1,
		CaseVersion:       1,
		ScreenshotEnabled: true,
		Script:            map[string]any{"command": []any{"sh", "-c", "true"}},
	}
	_, _ = r.Execute(ctx, []*domain.CaseSpec{spec})
	if created == nil {
		t.Fatal("runner did not create a job")
	}
	assertSecureRunnerJob(t, created)
}

func assertSecureRunnerJob(t *testing.T, job *batchv1.Job) {
	t.Helper()
	pod := job.Spec.Template.Spec
	if pod.ServiceAccountName != "gp-runner-job" || pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken {
		t.Fatalf("runner identity is not tokenless: sa=%q automount=%v", pod.ServiceAccountName, pod.AutomountServiceAccountToken)
	}
	if pod.SecurityContext == nil || pod.SecurityContext.RunAsNonRoot == nil || !*pod.SecurityContext.RunAsNonRoot || pod.SecurityContext.RunAsUser == nil || *pod.SecurityContext.RunAsUser != 100 {
		t.Fatalf("pod security context=%+v", pod.SecurityContext)
	}
	c := pod.Containers[0]
	if c.SecurityContext == nil || c.SecurityContext.AllowPrivilegeEscalation == nil || *c.SecurityContext.AllowPrivilegeEscalation || !dropsAll(c.SecurityContext.Capabilities) {
		t.Fatalf("container security context=%+v", c.SecurityContext)
	}
}

func dropsAll(caps *corev1.Capabilities) bool {
	if caps == nil {
		return false
	}
	for _, cap := range caps.Drop {
		if cap == "ALL" {
			return true
		}
	}
	return false
}
