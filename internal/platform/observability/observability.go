// Package observability 提供结构化日志（log/slog）与可观测性接口占位（指标/追踪后续接入）。
package observability

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/openware-io/open-green-pass/internal/platform/metrics"
)

// NewLogger 构造结构化日志器，level 对应 GP_ENV：dev=Debug、prod=Info。
func NewLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "dev" || env == "test" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: redactAttr,
	}))
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if sensitiveLogKey(attr.Key) {
		return slog.String(attr.Key, "[REDACTED]")
	}
	return attr
}

// MetricRecorder 指标记录接口占位（后续接 Prometheus/OTel）。
type MetricRecorder interface {
	// Incr 递增一个计数器（name 携带标签）。
	Incr(name string, n int64)
	// Observe 记录一个直方图观测值。
	Observe(name string, v float64)
}

// NoopRecorder 空实现，避免未接入时的 nil 恐慌。
type NoopRecorder struct{}

// Incr 空操作。
func (NoopRecorder) Incr(string, int64) {}

// Observe 空操作。
func (NoopRecorder) Observe(string, float64) {}

// Recorder is a process-local, Prometheus-compatible recorder. It is a
// deliberately small adapter for the GP4-06A skeleton: values are ephemeral
// and are not a capacity/SLO measurement. Labels are validated before they
// enter the store so tenant, user, run and case identifiers cannot become
// unbounded metric series.
type Recorder struct {
	mu         sync.RWMutex
	counters   map[string]float64
	histograms map[string]*histogram
}

type histogram struct {
	count uint64
	sum   float64
}

func NewRecorder() *Recorder {
	return &Recorder{counters: make(map[string]float64), histograms: make(map[string]*histogram)}
}

func (r *Recorder) Incr(name string, n int64)      { r.IncrLabels(name, nil, n) }
func (r *Recorder) Observe(name string, v float64) { r.ObserveLabels(name, nil, v) }

// IncrLabels records a counter with bounded labels. Invalid labels are
// ignored: telemetry must never make a real request fail.
func (r *Recorder) IncrLabels(name string, labels map[string]string, n int64) {
	if r == nil || n == 0 || metrics.ValidateLabels(labels) != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counters[series(name, labels)] += float64(n)
}

// ObserveLabels records a count and sum pair for a low-cardinality histogram.
func (r *Recorder) ObserveLabels(name string, labels map[string]string, v float64) {
	if r == nil || metrics.ValidateLabels(labels) != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := series(name, labels)
	h := r.histograms[k]
	if h == nil {
		h = &histogram{}
		r.histograms[k] = h
	}
	h.count++
	h.sum += v
}

func series(name string, labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	for _, k := range keys {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	return b.String()
}

func parseSeries(s string) (string, map[string]string) {
	parts := strings.Split(s, "|")
	labels := make(map[string]string)
	for _, p := range parts[1:] {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			labels[kv[0]] = kv[1]
		}
	}
	return parts[0], labels
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `%s="%s"`, k, strings.ReplaceAll(strings.ReplaceAll(labels[k], `\`, `\\`), `"`, `\"`))
	}
	b.WriteByte('}')
	return b.String()
}

// Handler returns Prometheus text exposition. The output is a snapshot and
// intentionally contains no service-specific SLO thresholds.
func (r *Recorder) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		r.mu.RLock()
		defer r.mu.RUnlock()
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		keys := make([]string, 0, len(r.counters))
		for k := range r.counters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			n, l := parseSeries(k)
			fmt.Fprintf(w, "%s%s %g\n", n, formatLabels(l), r.counters[k])
		}
		hkeys := make([]string, 0, len(r.histograms))
		for k := range r.histograms {
			hkeys = append(hkeys, k)
		}
		sort.Strings(hkeys)
		for _, k := range hkeys {
			n, l := parseSeries(k)
			h := r.histograms[k]
			fmt.Fprintf(w, "%s_count%s %d\n%s_sum%s %g\n", n, formatLabels(l), h.count, n, formatLabels(l), h.sum)
		}
	})
}
