// Package metrics defines the stable, low-cardinality metric contract.
package metrics

import (
	"errors"
	"strings"
)

var ErrSensitiveLabel = errors.New("metrics: sensitive or unbounded label")

// Names are intentionally domain-level; adapters map them to Prometheus or OTEL.
const (
	HTTPRequests      = "gp_http_requests_total"
	HTTPDuration      = "gp_http_request_duration_seconds"
	RunTransitions    = "gp_run_transitions_total"
	QueueWait         = "gp_queue_wait_seconds"
	WorkerActivities  = "gp_worker_activities_total"
	PoolCapacity      = "gp_pool_capacity"
	DependencyErrors  = "gp_dependency_errors_total"
)

var allowedLabels = map[string]struct{}{
	"method": {}, "route": {}, "status": {}, "component": {}, "dependency": {},
		"state": {}, "resource_type": {}, "result": {},
}

// ValidateLabels rejects IDs and arbitrary user supplied values from metric labels.
func ValidateLabels(labels map[string]string) error {
	for key, value := range labels {
		if _, ok := allowedLabels[key]; !ok || value == "" || strings.ContainsAny(value, "\r\n") {
			return ErrSensitiveLabel
		}
	}
	return nil
}

func AllowedLabels() map[string]struct{} {
	copy := make(map[string]struct{}, len(allowedLabels))
	for key := range allowedLabels {
		copy[key] = struct{}{}
	}
	return copy
}
