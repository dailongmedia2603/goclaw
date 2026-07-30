package facebookmessenger

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_DefaultFloor(t *testing.T) {
	rl := NewOutboundRateLimiter(0) // invalid → floor 20
	// 20 allows back-to-back without wait under a fixed clock.
	now := time.Now()
	rl.SetClock(func() time.Time { return now })
	for i := 0; i < 20; i++ {
		if !rl.Allow() {
			t.Fatalf("slot %d should be allowed", i)
		}
	}
	if rl.Allow() {
		t.Error("21st call should be blocked")
	}
}

func TestRateLimiter_WindowSlides(t *testing.T) {
	rl := NewOutboundRateLimiter(3)
	t0 := time.Unix(1_700_000_000, 0)
	var current time.Time
	var mu sync.Mutex
	setNow := func(ts time.Time) { mu.Lock(); current = ts; mu.Unlock() }
	rl.SetClock(func() time.Time { mu.Lock(); defer mu.Unlock(); return current })

	setNow(t0)
	for i := 0; i < 3; i++ {
		if !rl.Allow() {
			t.Fatalf("slot %d: should allow", i)
		}
	}
	if rl.Allow() {
		t.Fatal("4th should block")
	}

	// Slide window beyond 60s — old events should be evicted.
	setNow(t0.Add(61 * time.Second))
	if !rl.Allow() {
		t.Error("after window slide, new slot should be allowed")
	}
}

func TestRateLimiter_Pending(t *testing.T) {
	rl := NewOutboundRateLimiter(5)
	t0 := time.Now()
	rl.SetClock(func() time.Time { return t0 })
	for i := 0; i < 3; i++ {
		rl.Allow()
	}
	if p := rl.Pending(); p != 3 {
		t.Errorf("Pending=%d want=3", p)
	}
}

func TestRateLimiter_WaitRespectsContext(t *testing.T) {
	rl := NewOutboundRateLimiter(1)
	t0 := time.Now()
	rl.SetClock(func() time.Time { return t0 }) // frozen clock — never slides
	// Fill the slot.
	if !rl.Allow() {
		t.Fatal("initial Allow failed")
	}
	// Now Wait — must block until ctx cancel since clock is frozen.
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("Wait should fail with frozen clock when limiter is full")
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Errorf("Wait returned too quickly: %v", elapsed)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewOutboundRateLimiter(100)
	var wg sync.WaitGroup
	allowed := make(chan struct{}, 200)
	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow() {
				allowed <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(allowed)
	count := 0
	for range allowed {
		count++
	}
	if count > 100 {
		t.Errorf("over-allocated: %d > 100", count)
	}
	if count == 0 {
		t.Error("no slots allocated — likely a bug")
	}
}

func TestRateLimiter_SetClockNilRestoresDefault(t *testing.T) {
	rl := NewOutboundRateLimiter(5)
	rl.SetClock(func() time.Time { return time.Unix(0, 0) })
	rl.SetClock(nil)
	// Can still allow — covers the nil-safe path.
	if !rl.Allow() {
		t.Error("Allow should succeed with default clock")
	}
}
