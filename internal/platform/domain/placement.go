// Package domain contains provider-independent tenant placement contracts.
package domain

import (
	"context"
	"errors"
	"fmt"
)

type PlacementType string

const (
	PlacementShared   PlacementType = "shared"
	PlacementSchema   PlacementType = "schema"
	PlacementDatabase PlacementType = "database"
	PlacementCluster  PlacementType = "cluster"
)

type PlacementState string

const (
	PlacementPreparing PlacementState = "preparing"
	PlacementMigrating PlacementState = "migrating"
	PlacementVerifying PlacementState = "verifying"
	PlacementCutover   PlacementState = "cutover"
	PlacementStable    PlacementState = "stable"
	PlacementRollback  PlacementState = "rollback"
)

// TenantRoute is deliberately physical-location agnostic. Reference is an
// opaque secret/connection reference, never a DSN, schema or endpoint.
type TenantRoute struct {
	TeamID       int64
	Placement    PlacementType
	RouteVersion int64
	Reference    string
}

func (r TenantRoute) Validate() error {
	if r.TeamID <= 0 || r.RouteVersion <= 0 || r.Reference == "" {
		return errors.New("tenant route requires team, positive version and opaque reference")
	}
	switch r.Placement {
	case PlacementShared, PlacementSchema, PlacementDatabase, PlacementCluster:
		return nil
	default:
		return fmt.Errorf("unsupported placement type %q", r.Placement)
	}
}

type Placement struct {
	TeamID      int64
	Current     TenantRoute
	State       PlacementState
	MigrationID string
	Target      *TenantRoute
}

func (p Placement) Validate() error {
	if err := p.Current.Validate(); err != nil {
		return err
	}
	if p.State == "" {
		return errors.New("placement state required")
	}
	if p.State != PlacementStable && p.State != PlacementRollback && p.Target == nil {
		return errors.New("transition state requires target route")
	}
	if p.Target != nil && p.Target.TeamID != p.TeamID {
		return errors.New("target route team mismatch")
	}
	return nil
}

// Transition enforces the one-way migration protocol. Rollback is allowed
// from every in-flight phase and returns to the current stable route.
func (p *Placement) Transition(next PlacementState) error {
	if p == nil {
		return errors.New("nil placement")
	}
	if err := p.Validate(); err != nil {
		return err
	}
	allowed := map[PlacementState]map[PlacementState]bool{
		PlacementStable:    {PlacementPreparing: true},
		PlacementPreparing: {PlacementMigrating: true, PlacementRollback: true},
		PlacementMigrating: {PlacementVerifying: true, PlacementRollback: true},
		PlacementVerifying: {PlacementCutover: true, PlacementRollback: true},
		PlacementCutover:   {PlacementStable: true, PlacementRollback: true},
		PlacementRollback:  {},
	}
	if !allowed[p.State][next] {
		return fmt.Errorf("invalid placement transition %s -> %s", p.State, next)
	}
	if next == PlacementStable {
		if p.Target == nil {
			return errors.New("stable transition requires target route")
		}
		p.Current = *p.Target
		p.Target = nil
	}
	p.State = next
	return nil
}

// TenantRouteResolver is the only route lookup boundary used by applications.
type TenantRouteResolver interface {
	Resolve(context.Context, int64) (TenantRoute, error)
}

// ResolveFailClosed rejects invalid or missing routes; callers must not fall
// back to a client-supplied endpoint or shared connection on resolver errors.
func ResolveFailClosed(ctx context.Context, resolver TenantRouteResolver, teamID int64) (TenantRoute, error) {
	if resolver == nil || teamID <= 0 {
		return TenantRoute{}, errors.New("tenant route unavailable")
	}
	route, err := resolver.Resolve(ctx, teamID)
	if err != nil {
		return TenantRoute{}, fmt.Errorf("resolve tenant route: %w", err)
	}
	if err := route.Validate(); err != nil {
		return TenantRoute{}, fmt.Errorf("invalid tenant route: %w", err)
	}
	return route, nil
}

type ConsistencyReport struct {
	SourceRows, TargetRows int64
	SourceHash, TargetHash string
	AuditContinuous        bool
}

func (r ConsistencyReport) Valid() error {
	if r.SourceRows != r.TargetRows {
		return fmt.Errorf("row count mismatch: source=%d target=%d", r.SourceRows, r.TargetRows)
	}
	if r.SourceHash == "" || r.SourceHash != r.TargetHash {
		return errors.New("data hash mismatch")
	}
	if !r.AuditContinuous {
		return errors.New("audit chain is not continuous")
	}
	return nil
}
