//go:build !sqliteonly

package fbcloak

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// PlanCleanup runs once a day:
//   - AutoCancelExpired: cancel plans scheduled > now+90d (catches admin
//     errors and prevents runaway accumulation).
type PlanCleanup struct {
	Plans  PlanStore
	Logger *slog.Logger

	TickInterval time.Duration
	TTLDays      int

	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

const DefaultCleanupTick = 24 * time.Hour

func (c *PlanCleanup) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return nil
	}
	if c.TickInterval == 0 {
		c.TickInterval = DefaultCleanupTick
	}
	if c.TTLDays == 0 {
		c.TTLDays = 90
	}
	if c.Logger == nil {
		c.Logger = slog.Default()
	}
	c.stopCh = make(chan struct{})
	c.running = true
	go c.loop(ctx)
	return nil
}

func (c *PlanCleanup) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	close(c.stopCh)
	c.running = false
}

func (c *PlanCleanup) loop(ctx context.Context) {
	// Run once on start, sleeping briefly so app boot stays fast.
	timer := time.NewTimer(time.Minute)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-c.stopCh:
		return
	case <-timer.C:
	}
	c.tick(ctx)

	t := time.NewTicker(c.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-t.C:
			c.tick(ctx)
		}
	}
}

func (c *PlanCleanup) tick(ctx context.Context) {
	if c.Plans == nil {
		return
	}
	now := time.Now().UTC()
	ttl := time.Duration(c.TTLDays) * 24 * time.Hour
	n, err := c.Plans.AutoCancelExpired(ctx, now, ttl)
	if err != nil {
		c.Logger.Warn("fbcloak.plan_cleanup.auto_cancel_failed", "err", err)
		return
	}
	if n > 0 {
		c.Logger.Info("fbcloak.plan_cleanup.cancelled",
			"count", n, "ttl_days", c.TTLDays,
		)
	}
}
