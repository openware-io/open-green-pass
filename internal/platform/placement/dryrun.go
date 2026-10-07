// Package placement contains provider-independent placement planning.
package placement

import (
	"context"
	"errors"
	"fmt"

	platform "github.com/openware-io/open-green-pass/internal/platform/domain"
)

var ErrDryRunRejected = errors.New("tenant placement dry-run rejected")

// ConsistencyChecker supplies read-only source/target checks. It must not
// mutate data or perform a cutover.
type ConsistencyChecker interface {
	Check(context.Context, platform.TenantRoute, platform.TenantRoute) (platform.ConsistencyReport, error)
}

type Plan struct {
	TeamID      int64
	From        platform.TenantRoute
	To          platform.TenantRoute
	Consistency platform.ConsistencyReport
	WillCutover bool
}

// DryRun validates a placement change and, when a checker is supplied,
// verifies source/target consistency without changing the active route.
func DryRun(ctx context.Context, current, target platform.TenantRoute, checker ConsistencyChecker) (Plan, error) {
	if err := current.Validate(); err != nil {
		return Plan{}, fmt.Errorf("%w: invalid current route: %v", ErrDryRunRejected, err)
	}
	if err := target.Validate(); err != nil {
		return Plan{}, fmt.Errorf("%w: invalid target route: %v", ErrDryRunRejected, err)
	}
	if current.TeamID != target.TeamID {
		return Plan{}, fmt.Errorf("%w: route team mismatch", ErrDryRunRejected)
	}
	if target.RouteVersion <= current.RouteVersion {
		return Plan{}, fmt.Errorf("%w: target route version must increase", ErrDryRunRejected)
	}
	plan := Plan{TeamID: current.TeamID, From: current, To: target, WillCutover: false}
	if checker == nil {
		return plan, nil
	}
	report, err := checker.Check(ctx, current, target)
	if err != nil {
		return Plan{}, fmt.Errorf("%w: consistency check: %v", ErrDryRunRejected, err)
	}
	if err := report.Valid(); err != nil {
		return Plan{}, fmt.Errorf("%w: consistency report: %v", ErrDryRunRejected, err)
	}
	plan.Consistency = report
	return plan, nil
}
