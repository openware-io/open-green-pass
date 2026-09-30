package pool

import "testing"

func TestRegistrySelectsAvailableCapablePool(t *testing.T) {
	r := NewRegistry()
	if err := r.Upsert(Registration{ID: "browser-a", Cluster: "kind-a", Resources: []string{"browser"}, Capacity: 4, Available: 1, Enabled: true}); err != nil { t.Fatal(err) }
	if err := r.Upsert(Registration{ID: "browser-b", Cluster: "kind-b", Resources: []string{"browser", "load"}, Capacity: 8, Available: 5, Enabled: true}); err != nil { t.Fatal(err) }
	p, err := r.Select("browser"); if err != nil || p.ID != "browser-b" { t.Fatalf("pool=%+v err=%v", p, err) }
	if err := r.Reserve(p.ID, 2); err != nil { t.Fatal(err) }
	p, _ = r.Get("browser-b"); if p.Available != 3 { t.Fatalf("available=%d", p.Available) }
	if err := r.Release("browser-b", 2); err != nil { t.Fatal(err) }
}

func TestRegistryRejectsDisabledOrUnsupportedPool(t *testing.T) {
	r := NewRegistry(); _ = r.Upsert(Registration{ID: "device", Cluster: "stf", Resources: []string{"device"}, Capacity: 1, Available: 1, Enabled: false})
	if _, err := r.Select("device"); err != ErrPoolNotFound { t.Fatalf("err=%v", err) }
}
