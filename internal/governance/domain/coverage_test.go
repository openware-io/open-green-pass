package domain

import (
	"testing"
	"time"
)

func TestBuildCoveragePlanUserModule(t *testing.T) {
	endpoints := []Endpoint{
		{Method: "GET", Path: "/api/v1/admin/users", Module: "user"},
		{Method: "GET", Path: "/api/v1/admin/users/{id}", Module: "user"},
		{Method: "PUT", Path: "/api/v1/admin/users/{id}/status", Module: "user"},
		{Method: "DELETE", Path: "/api/v1/admin/users/{id}", Module: "user"},
	}
	cells := BuildCoveragePlan(1, endpoints)
	if len(cells) != 24 {
		t.Fatalf("cells=%d want 24", len(cells))
	}
	seen := map[string]bool{}
	for _, c := range cells {
		if !c.Required {
			t.Fatalf("cell must be required: %+v", c)
		}
		key := c.EndpointHash + c.Dimension + c.EquivalenceClass
		if seen[key] {
			t.Fatalf("duplicate cell %s", key)
		}
		seen[key] = true
	}
}

func TestEndpointHashStable(t *testing.T) {
	a := EndpointHash("post", "/api/v1/x/")
	b := EndpointHash("POST", "/api/v1/x")
	if a != b {
		t.Fatalf("hash should normalize: %s != %s", a, b)
	}
}

func TestArchetype(t *testing.T) {
	cases := map[string]struct {
		ep   Endpoint
		want string
	}{
		"list":   {Endpoint{Method: "GET", Path: "/admin/users"}, "list"},
		"detail": {Endpoint{Method: "GET", Path: "/admin/users/{id}"}, "detail"},
		"upload": {Endpoint{Method: "POST", Path: "/media/upload"}, "upload"},
		"state":  {Endpoint{Method: "POST", Path: "/releases/{id}/submit"}, "state"},
		"delete": {Endpoint{Method: "DELETE", Path: "/admin/users/{id}"}, "delete"},
		"auth":   {Endpoint{Method: "POST", Path: "/auth/login"}, "auth"},
	}
	for name, tc := range cases {
		if got := Archetype(tc.ep); got != tc.want {
			t.Fatalf("%s: got %s want %s", name, got, tc.want)
		}
	}
}

func TestCoverageCellWaivedAt(t *testing.T) {
	now := time.Now().UTC()
	waiverID := int64(9)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)
	if (CoverageCell{Status: "planned"}).WaivedAt(now) {
		t.Fatal("planned cell must not be waived")
	}
	if (CoverageCell{Status: "waived"}).WaivedAt(now) == false {
		t.Fatal("waived status without waiver must count as waived")
	}
	if (CoverageCell{Status: "waived", WaiverID: &waiverID, WaiverExpiresAt: &future}).WaivedAt(now) == false {
		t.Fatal("future waiver must count as waived")
	}
	if (CoverageCell{Status: "waived", WaiverID: &waiverID, WaiverExpiresAt: &past}).WaivedAt(now) {
		t.Fatal("expired waiver must not count as waived")
	}
}
