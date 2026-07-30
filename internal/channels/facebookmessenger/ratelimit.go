package facebookmessenger

import (
	"context"
	"sync"
	"time"
)

// OutboundRateLimiter enforces a per-account outbound message rate using a
// sliding-window counter. Keeps Meta anti-automation heuristics happy by
// preventing burst sends.
//
// Not meant to be accurate to microsecond — goal is "no more than N msgs/minute".
type OutboundRateLimiter struct {
	mu         sync.Mutex
	maxPerMin  int
	windowSize time.Duration
	events     []time.Time

	// Overridable clock for tests. Default uses time.Now.
	nowFn func() time.Time
}

// NewOutboundRateLimiter constructs a limiter. maxPerMin <= 0 falls back to 20.
func NewOutboundRateLimiter(maxPerMin int) *OutboundRateLimiter {
	if maxPerMin <= 0 {
		maxPerMin = 20
	}
	return &OutboundRateLimiter{
		maxPerMin:  maxPerMin,
		windowSize: time.Minute,
		events:     make([]time.Time, 0, maxPerMin),
		nowFn:      time.Now,
	}
}

// Allow consumes a slot if available. Non-blocking.
func (rl *OutboundRateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.nowFn()
	cutoff := now.Add(-rl.windowSize)

	// Evict old entries (kept roughly sorted, but be defensive).
	filtered := rl.events[:0]
	for _, ts := range rl.events {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	rl.events = filtered

	if len(rl.events) >= rl.maxPerMin {
		return false
	}
	rl.events = append(rl.events, now)
	return true
}

// Wait blocks until a slot is available, or ctx is cancelled.
// Uses exponential backoff capped at 3 seconds between polls.
func (rl *OutboundRateLimiter) Wait(ctx context.Context) error {
	backoff := 100 * time.Millisecond
	for {
		if rl.Allow() {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 3*time.Second {
			backoff *= 2
		}
	}
}

// Pending returns the number of events in the current window.
// Useful for metrics + debugging.
func (rl *OutboundRateLimiter) Pending() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := rl.nowFn().Add(-rl.windowSize)
	n := 0
	for _, ts := range rl.events {
		if ts.After(cutoff) {
			n++
		}
	}
	return n
}

// SetClock is exported for tests to inject a deterministic clock.
func (rl *OutboundRateLimiter) SetClock(nowFn func() time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if nowFn == nil {
		rl.nowFn = time.Now
	} else {
		rl.nowFn = nowFn
	}
}
