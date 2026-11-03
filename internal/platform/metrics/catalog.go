// Package metrics defines the stable, low-cardinality metric contract.
package metrics

import (
	"errors"
	"regexp"
	"strings"
)

var ErrSensitiveLabel = errors.New("metrics: sensitive or unbounded label")

// Names are intentionally domain-level; adapters map them to Prometheus or OTEL.
const (
	HTTPRequests     = "gp_http_requests_total"
	HTTPDuration     = "gp_http_request_duration_seconds"
	RunTransitions   = "gp_run_transitions_total"
	QueueWait        = "gp_queue_wait_seconds"
	WorkerActivities = "gp_worker_activities_total"
	PoolCapacity     = "gp_pool_capacity"
	DependencyErrors = "gp_dependency_errors_total"

	// Generation plan GC is a background sweep: counters are intentionally
	// low-cardinality (no tenant/team labels) so a large tenant registry cannot
	// explode the metric series.
	GenerationPlanGCExpired  = "gp_generation_plan_gc_expired_total"
	GenerationPlanGCFailed   = "gp_generation_plan_gc_failed_total"
	GenerationPlanGCSweeps   = "gp_generation_plan_gc_sweeps_total"
	GenerationPlanGCDuration = "gp_generation_plan_gc_duration_seconds"
)

var allowedLabels = map[string]struct{}{
	"method": {}, "route": {}, "status": {}, "component": {}, "dependency": {},
	"state": {}, "resource_type": {}, "result": {},
}

var dynamicPath = regexp.MustCompile(`/[0-9a-fA-F-]{3,}`)

// ValidateLabels rejects IDs and arbitrary user supplied values from metric labels.
func ValidateLabels(labels map[string]string) error {
	for key, value := range labels {
		if _, ok := allowedLabels[key]; !ok || value == "" || strings.ContainsAny(value, "\r\n") || (key == "route" && dynamicPath.MatchString(value)) {
			return ErrSensitiveLabel
		}
	}
	return nil
}

func AllowedLabels() map[string]struct{} {
	clone := make(map[string]struct{}, len(allowedLabels))
	for key := range allowedLabels {
		clone[key] = struct{}{}
	}
	return clone
}
