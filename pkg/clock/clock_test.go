package clock

import (
	"testing"
	"time"
)

func TestSystem(t *testing.T) {
	if System().Now().IsZero() {
		t.Fatal("System().Now() 不应为零值")
	}
}

func TestMock(t *testing.T) {
	fixed := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	m := NewMock(fixed)
	if !m.Now().Equal(fixed) {
		t.Fatalf("Now()=%v, want %v", m.Now(), fixed)
	}
	after := fixed.Add(time.Hour)
	m.Set(after)
	if !m.Now().Equal(after) {
		t.Fatalf("Set 后 Now()=%v, want %v", m.Now(), after)
	}
}
