package pool

import (
	"testing"
	"time"
)

func TestRegistrySelectsAvailableCapablePool(t *testing.T) {
	r := NewRegistry()
	if err := r.Upsert(Registration{ID: "browser-a", Cluster: "kind-a", Resources: []string{"browser"}, Capacity: 4, Available: 1, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.Upsert(Registration{ID: "browser-b", Cluster: "kind-b", Resources: []string{"browser", "load"}, Capacity: 8, Available: 5, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	p, err := r.Select("browser")
	if err != nil || p.ID != "browser-b" {
		t.Fatalf("pool=%+v err=%v", p, err)
	}
	if err := r.Reserve(p.ID, 2); err != nil {
		t.Fatal(err)
	}
	p, _ = r.Get("browser-b")
	if p.Available != 3 {
		t.Fatalf("available=%d", p.Available)
	}
	if err := r.Release("browser-b", 2); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryRejectsDisabledOrUnsupportedPool(t *testing.T) {
	r := NewRegistry()
	_ = r.Upsert(Registration{ID: "device", Cluster: "stf", Resources: []string{"device"}, Capacity: 1, Available: 1, Enabled: false})
	if _, err := r.Select("device"); err != ErrPoolNotFound {
		t.Fatalf("err=%v", err)
	}
}

func TestRegistrySelectLiveHonorsHeartbeatTTL(t *testing.T) {
	r := NewRegistry()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	if err := r.Upsert(Registration{ID: "fresh", Cluster: "kind", Resources: []string{"browser"}, Capacity: 2, Available: 1, Enabled: true, LastHeartbeat: now.Add(-5 * time.Second)}); err != nil {
		t.Fatal(err)
	}
	if err := r.Upsert(Registration{ID: "stale", Cluster: "kind", Resources: []string{"browser"}, Capacity: 2, Available: 2, Enabled: true, LastHeartbeat: now.Add(-2 * time.Minute)}); err != nil {
		t.Fatal(err)
	}
	p, err := r.SelectLive("browser", now, time.Minute)
	if err != nil || p.ID != "fresh" {
		t.Fatalf("pool=%+v err=%v", p, err)
	}
	if err := r.Heartbeat("stale", now.Add(-10*time.Second)); err != nil {
		t.Fatal(err)
	}
	p, err = r.SelectLive("browser", now, time.Minute)
	if err != nil || p.ID != "stale" {
		t.Fatalf("after heartbeat pool=%+v err=%v", p, err)
	}
}

func TestRegistryHeartbeatRejectsUnknownOrZeroTime(t *testing.T) {
	r := NewRegistry()
	if err := r.Heartbeat("missing", time.Now()); err != ErrPoolNotFound {
		t.Fatalf("err=%v, want not found", err)
	}
	if err := r.Upsert(Registration{ID: "pool", Cluster: "kind", Resources: []string{"browser"}, Capacity: 1, Available: 1, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := r.Heartbeat("pool", time.Time{}); err != ErrInvalidHeartbeat {
		t.Fatalf("err=%v, want invalid heartbeat", err)
	}
}
