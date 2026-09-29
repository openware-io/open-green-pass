// Package observability 提供结构化日志（log/slog）与可观测性接口占位（指标/追踪后续接入）。
package observability

import (
	"log/slog"
	"os"
)

// NewLogger 构造结构化日志器，level 对应 GP_ENV：dev=Debug、prod=Info。
func NewLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "dev" || env == "test" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
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
