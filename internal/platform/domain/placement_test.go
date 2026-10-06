package domain

import (
	"context"
	"testing"
)

func route(team int64, ref string, version int64) TenantRoute {
	return TenantRoute{TeamID: team, Placement: PlacementShared, Reference: ref, RouteVersion: version}
}

func TestPlacementStateMachineAndRollback(t *testing.T) {
	p := Placement{TeamID: 7, Current: route(7, "shared", 1), State: PlacementStable, Target: &TenantRoute{TeamID: 7, Placement: PlacementSchema, Reference: "opaque-target", RouteVersion: 2}, MigrationID: "m-1"}
	for _, state := range []PlacementState{PlacementPreparing, PlacementMigrating, PlacementVerifying, PlacementCutover, PlacementStable} {
		if err := p.Transition(state); err != nil {
			t.Fatalf("transition to %s: %v", state, err)
		}
	}
	if p.Current.Placement != PlacementSchema || p.Target != nil {
		t.Fatalf("cutover did not promote target: %+v", p)
	}
	if err := p.Transition(PlacementPreparing); err != nil {
		t.Fatal(err)
	}
	p.Target = &TenantRoute{TeamID: 7, Placement: PlacementCluster, Reference: "next", RouteVersion: 3}
	if err := p.Transition(PlacementRollback); err != nil {
		t.Fatal(err)
	}
	if p.Current.RouteVersion != 2 {
		t.Fatal("rollback changed current route")
	}
}

func TestPlacementRejectsInvalidTransitionAndRoute(t *testing.T) {
	p := Placement{TeamID: 1, Current: route(1, "x", 1), State: PlacementStable}
	if err := p.Transition(PlacementVerifying); err == nil {
		t.Fatal("expected invalid transition")
	}
	if _, err := ResolveFailClosed(context.Background(), nil, 1); err == nil {
		t.Fatal("nil resolver must fail closed")
	}
	bad := staticResolver{route: TenantRoute{TeamID: 1, Reference: "x", RouteVersion: 1}}
	if _, err := ResolveFailClosed(context.Background(), bad, 1); err == nil {
		t.Fatal("invalid route must fail closed")
	}
}

type staticResolver struct {
	route TenantRoute
	err   error
}

func (s staticResolver) Resolve(context.Context, int64) (TenantRoute, error) { return s.route, s.err }

func TestConsistencyReport(t *testing.T) {
	if err := (ConsistencyReport{SourceRows: 2, TargetRows: 2, SourceHash: "h", TargetHash: "h", AuditContinuous: true}).Valid(); err != nil {
		t.Fatal(err)
	}
	for _, report := range []ConsistencyReport{{SourceRows: 1, TargetRows: 2}, {SourceHash: "a", TargetHash: "b"}, {SourceHash: "a", TargetHash: "a"}} {
		if err := report.Valid(); err == nil {
			t.Fatal("expected consistency failure")
		}
	}
}
