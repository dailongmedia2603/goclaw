package fbbackfill

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// bucEntry is one rate-limit object for a single BUC type (e.g. "pages").
// Values are percentages of the hourly quota used.
type bucEntry struct {
	Type                        string `json:"type,omitempty"`
	CallCount                   int    `json:"call_count"`
	TotalCPUTime                int    `json:"total_cputime"`
	TotalTime                   int    `json:"total_time"`
	EstimatedTimeToRegainAccess int    `json:"estimated_time_to_regain_access"` // minutes
}

// peak returns the largest of the three percentage metrics.
func (e bucEntry) peak() int {
	m := e.CallCount
	if e.TotalCPUTime > m {
		m = e.TotalCPUTime
	}
	if e.TotalTime > m {
		m = e.TotalTime
	}
	return m
}

// bucTracker watches the X-Business-Use-Case-Usage header across calls
// and advises the client on how long to pause between requests.
type bucTracker struct {
	mu               sync.Mutex
	lastPeak         int
	lastRegainMins   int
	lastSeen         time.Time
}

// ParseHeader decodes a single X-Business-Use-Case-Usage header value.
// The header shape is {"<id>":[{...entries...}]}. We take the worst peak
// across all entries for all IDs.
func (b *bucTracker) ParseHeader(raw string) {
	if raw == "" {
		return
	}
	var decoded map[string][]bucEntry
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		slog.Debug("fb_backfill.buc.parse_failed", "err", err, "raw_len", len(raw))
		return
	}
	worst := 0
	worstRegain := 0
	for _, entries := range decoded {
		for _, e := range entries {
			if p := e.peak(); p > worst {
				worst = p
				worstRegain = e.EstimatedTimeToRegainAccess
			}
		}
	}
	b.mu.Lock()
	b.lastPeak = worst
	b.lastRegainMins = worstRegain
	b.lastSeen = time.Now()
	b.mu.Unlock()
	if worst >= 90 {
		slog.Warn("fb_backfill.client.rate_limit", "buc_pct", worst, "regain_mins", worstRegain)
	}
}

// ShouldPauseFor returns how long to sleep before the next call based on
// the most recent BUC reading. Returns 0 if under the pacing threshold.
func (b *bucTracker) ShouldPauseFor() time.Duration {
	b.mu.Lock()
	peak := b.lastPeak
	b.mu.Unlock()
	switch {
	case peak >= 90:
		return 60 * time.Second
	case peak >= 70:
		return 10 * time.Second
	case peak >= 50:
		return 2 * time.Second
	default:
		return 0
	}
}

// IsSaturated reports whether the tracker has seen a reading at or near
// the quota limit, meaning the caller should pause the job and auto-resume
// after ResumeAfter().
func (b *bucTracker) IsSaturated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastPeak >= 100
}

// ResumeAfter returns how long to wait before auto-resuming a saturated
// job. Falls back to 1 hour if the estimate is zero or missing.
func (b *bucTracker) ResumeAfter() time.Duration {
	b.mu.Lock()
	mins := b.lastRegainMins
	b.mu.Unlock()
	if mins <= 0 {
		return time.Hour
	}
	return time.Duration(mins) * time.Minute
}

// PeakPercent is the most recent peak BUC reading, for logging/metrics.
func (b *bucTracker) PeakPercent() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lastPeak
}
