package id

import (
	"sync"
	"testing"
	"time"

	"github.com/openware-io/open-green-pass/pkg/clock"
)

func TestNextUniqueness(t *testing.T) {
	g, err := New(1, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for i := 0; i < 10000; i++ {
		v := g.Next()
		if seen[v] {
			t.Fatalf("重复 ID: %d", v)
		}
		seen[v] = true
	}
}

func TestNextConcurrent(t *testing.T) {
	g, err := New(2, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	const workers = 8
	const per = 2000
	seen := make(chan int64, workers*per)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				seen <- g.Next()
			}
		}()
	}
	wg.Wait()
	close(seen)
	set := map[int64]bool{}
	for v := range seen {
		if set[v] {
			t.Fatal("并发下出现重复 ID")
		}
		set[v] = true
	}
	if len(set) != workers*per {
		t.Fatalf("ID 数量 %d, want %d", len(set), workers*per)
	}
}

func TestInvalidNode(t *testing.T) {
	if _, err := New(8, clock.System()); err == nil {
		t.Fatal("node=8 应报错")
	}
}

func TestMonotonicWithinMs(t *testing.T) {
	base := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	m := clock.NewMock(base)
	g, err := New(3, m)
	if err != nil {
		t.Fatal(err)
	}
	prev := g.Next()
	// 同一毫秒内多次取应递增（seq），但固定时钟下上限为 seqMask+1（4096），取 1000 即可。
	for i := 0; i < 1000; i++ {
		cur := g.Next()
		if cur <= prev {
			t.Fatalf("非单调: %d <= %d", cur, prev)
		}
		prev = cur
	}
}

func TestNextRemainsJavaScriptSafe(t *testing.T) {
	g, err := New(1, clock.System())
	if err != nil {
		t.Fatal(err)
	}
	const maxSafeInteger = int64(1<<53 - 1)
	for i := 0; i < 10_000; i++ {
		if got := g.Next(); got <= 0 || got > maxSafeInteger {
			t.Fatalf("id %d is not safe for a JavaScript Number", got)
		}
	}
}

func TestClockRollback(t *testing.T) {
	base := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	m := clock.NewMock(base)
	g, err := New(5, m)
	if err != nil {
		t.Fatal(err)
	}
	_ = g.Next()
	// 时钟回拨 5ms：仍应产生单调递增的 ID（沿用 last）
	m.Set(base.Add(-5 * time.Millisecond))
	prev := g.Next()
	last := g.Next()
	if last <= prev {
		t.Fatalf("回拨后 ID 不单调: %d <= %d", last, prev)
	}
}
