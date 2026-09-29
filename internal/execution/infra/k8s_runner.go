// 执行器实现：K8s Job Runner（gp1-04-b）——在 gp-runner 沙箱命名空间为每条用例建 Job 执行，
// 轮询完成状态，收集日志并计算证据哈希，回收 Job 后回写执行结果。
package infra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
	"github.com/openware-io/open-green-pass/pkg/id"
)

// K8sRunnerConfig K8s Job 执行器配置。
type K8sRunnerConfig struct {
	Kubeconfig string // 空 = in-cluster
	Namespace  string // 默认 gp-runner
	Image      string // 默认 busybox
	Timeout    time.Duration
}

// K8sRunner 基于 K8s Job 的沙箱执行器（gp-runner ns）。
type K8sRunner struct {
	gen  *id.Generator
	cl   kubernetes.Interface
	cfg  K8sRunnerConfig
}

// NewK8sRunner 创建 K8s Job 执行器。
func NewK8sRunner(gen *id.Generator, cfg K8sRunnerConfig) (*K8sRunner, error) {
	if cfg.Namespace == "" {
		cfg.Namespace = "gp-runner"
	}
	if cfg.Image == "" {
		cfg.Image = "busybox:1.36"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Minute
	}
	var rc *rest.Config
	var err error
	if cfg.Kubeconfig != "" {
		rc, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else if kc := os.Getenv("KUBECONFIG"); kc != "" {
		rc, err = clientcmd.BuildConfigFromFlags("", kc)
	} else if _, e := os.Stat(filepath.Join(os.Getenv("HOME"), ".kube", "config")); e == nil {
		rc, err = clientcmd.BuildConfigFromFlags("", filepath.Join(os.Getenv("HOME"), ".kube", "config"))
	} else {
		rc, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("k8s client config: %w", err)
	}
	cl, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("k8s clientset: %w", err)
	}
	return &K8sRunner{gen: gen, cl: cl, cfg: cfg}, nil
}

// scriptSpec 从用例脚本解析的执行规格（image/command；缺省用 busybox echo 冒烟）。
type scriptSpec struct {
	Image   string   `json:"image,omitempty"`
	Command []string `json:"command,omitempty"`
}

func resolveScript(s any) scriptSpec {
	ss := scriptSpec{}
	switch m := s.(type) {
	case map[string]any:
		if im, ok := m["image"].(string); ok {
			ss.Image = im
		}
		if cmd, ok := m["command"].([]any); ok {
			for _, c := range cmd {
				if cs, ok2 := c.(string); ok2 {
					ss.Command = append(ss.Command, cs)
				}
			}
		}
	}
	return ss
}

// Execute 为每条用例在 gp-runner 建 Job 执行，轮询结果并回收，返回用例结果。
func (r *K8sRunner) Execute(ctx context.Context, spec []*domain.CaseSpec) ([]*domain.CaseResult, error) {
	results := make([]*domain.CaseResult, 0, len(spec))
	for _, s := range spec {
		res, err := r.executeOne(ctx, s)
		if err != nil {
			return results, fmt.Errorf("case %d: %w", s.CaseID, err)
		}
		results = append(results, res)
	}
	return results, nil
}

func (r *K8sRunner) executeOne(ctx context.Context, s *domain.CaseSpec) (*domain.CaseResult, error) {
	now := time.Now().UTC()
	ss := resolveScript(s.Script)
	image := ss.Image
	if image == "" {
		image = r.cfg.Image
	}
	cmd := ss.Command
	if len(cmd) == 0 {
		cmd = []string{"sh", "-c", "echo gp-runner-ok"}
	}
	jobName := fmt.Sprintf("gp-run-%d-%d", s.CaseID, time.Now().UnixNano()%1000000)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: r.cfg.Namespace},
		Spec: batchv1.JobSpec{
			BackoffLimit:            int32ptr(0),
			TTLSecondsAfterFinished: int32ptr(120),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{
						Name: "runner", Image: image, Command: cmd,
					}},
				},
			},
		},
	}
	_, err := r.cl.BatchV1().Jobs(r.cfg.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, err
	}
	defer func() {
		_ = r.cl.BatchV1().Jobs(r.cfg.Namespace).Delete(context.Background(), jobName,
			metav1.DeleteOptions{PropagationPolicy: &delProp})
	}()

	// 轮询 Job 完成状态
	status := domain.CaseFail
	var logs string
	deadline := time.Now().Add(r.cfg.Timeout)
	for {
		if time.Now().After(deadline) {
			status = domain.CaseBlocked
			break
		}
		cur, err := r.cl.BatchV1().Jobs(r.cfg.Namespace).Get(ctx, jobName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				break
			}
			return nil, err
		}
		if cur.Status.Succeeded > 0 {
			status = domain.CasePass
			logs, _ = r.podLogs(ctx, jobName)
			break
		}
		if cur.Status.Failed > 0 {
			status = domain.CaseFail
			logs, _ = r.podLogs(ctx, jobName)
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(800 * time.Millisecond):
		}
	}
	ended := time.Now().UTC()
	hash := sha256.Sum256([]byte(logs))
	ev := &domain.EvidenceRef{
		Logs: []string{fmt.Sprintf("job://%s/%s", r.cfg.Namespace, jobName)},
		Hash: hex.EncodeToString(hash[:]),
	}
	if s.ScreenshotEnabled {
		ev.Screenshots = []string{fmt.Sprintf("job://%s/%s/screenshot", r.cfg.Namespace, jobName)}
	}
	return &domain.CaseResult{
		ID: r.gen.Next(), CaseID: s.CaseID, CaseVersion: s.CaseVersion,
		Status: status, ResultText: truncateStr(logs, 400), Evidence: ev,
		AttemptSeq: 1, StartedAt: now, EndedAt: &ended,
	}, nil
}

var delProp = metav1.DeletePropagationForeground

func (r *K8sRunner) podLogs(ctx context.Context, jobName string) (string, error) {
	pods, err := r.cl.CoreV1().Pods(r.cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "job-name=" + jobName,
	})
	if err != nil || len(pods.Items) == 0 {
		return "", err
	}
	pod := pods.Items[0]
	rc := r.cl.CoreV1().Pods(r.cfg.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	raw, err := rc.DoRaw(ctx)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func int32ptr(v int32) *int32 { return &v }

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n]) + "…"
}

var _ domain.RunnerPort = (*K8sRunner)(nil)
