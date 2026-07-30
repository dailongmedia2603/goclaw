//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SkipReason enumerates non-error skip causes the executor records on the send_log.
type SkipReason string

const (
	SkipReasonCapReached     SkipReason = "cap_reached"
	SkipReasonCooldown       SkipReason = "cooldown"
	SkipReasonContentBlocked SkipReason = "content_blocked"
	SkipReasonOptOut         SkipReason = "opt_out"
	SkipReasonTooLong        SkipReason = "too_long"
	SkipReasonRecentReply    SkipReason = "customer_replied_recently"
	SkipReasonOutsideHours   SkipReason = "outside_working_hours"
)

// PolicyConfig holds the runtime knobs. Per-job overrides come from the Job
// row (DailyCap, WorkingHours); per-tenant defaults can override the global.
type PolicyConfig struct {
	DailyCap             int
	HardDailyCap         int           // upper bound the user cannot exceed even per-job (50)
	PerRecipientCooldown time.Duration // 30 days default
	MaxMessageLen        int           // 500 chars default
	Blocklist            []string      // promotional keywords; case-insensitive substring match
	OptOutKeywords       []string      // recipient phrases that disable future sends
}

// DefaultPolicyConfig matches research §6+§7.2 defaults.
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		DailyCap:             30,
		HardDailyCap:         50,
		PerRecipientCooldown: 30 * 24 * time.Hour,
		MaxMessageLen:        500,
		Blocklist: []string{
			"khuyến mãi", "khuyen mai",
			"sale",
			"giảm giá", "giam gia",
			"deal",
			"coupon",
			"voucher",
			"miễn phí", "mien phi",
			"free",
			"đăng ký", "dang ky",
			"click here",
		},
		OptOutKeywords: []string{"stop", "hủy", "huy", "không", "khong"},
	}
}

// PolicyStore is the slice of CredentialStore + SendLogStore the policy needs
// to count today's sends and find the last attempt to a recipient.
type PolicyStore interface {
	CountTodaySends(ctx context.Context, credentialID uuid.UUID, fanpageID string, since time.Time) (int, error)
	LastSendTo(ctx context.Context, credentialID uuid.UUID, recipientPSID string) (*time.Time, error)
}

// Policy enforces caps, cooldowns, and content rules. Pure logic + read-only
// queries; no DB writes.
type Policy struct {
	cfg   PolicyConfig
	store PolicyStore
	now   func() time.Time
}

// NewPolicy constructs a Policy. cfg.DailyCap is clamped to HardDailyCap;
// store may be nil for unit tests that exercise content rules only.
func NewPolicy(cfg PolicyConfig, store PolicyStore) *Policy {
	if cfg.DailyCap == 0 {
		cfg = DefaultPolicyConfig()
	}
	if cfg.HardDailyCap > 0 && cfg.DailyCap > cfg.HardDailyCap {
		cfg.DailyCap = cfg.HardDailyCap
	}
	return &Policy{cfg: cfg, store: store, now: time.Now}
}

// SetClock lets tests inject a deterministic time source.
func (p *Policy) SetClock(now func() time.Time) { p.now = now }

// AllowSend evaluates every rule and returns the first failure (as SkipReason)
// or "" + nil if the send may proceed. err is non-nil only on store failure.
func (p *Policy) AllowSend(ctx context.Context, j Job, credentialID uuid.UUID, recipient Target, message string) (SkipReason, error) {
	// Length gate (cheap; fail fast).
	if p.cfg.MaxMessageLen > 0 && len([]rune(message)) > p.cfg.MaxMessageLen {
		return SkipReasonTooLong, nil
	}
	if reason := p.contentRule(message); reason != "" {
		return reason, nil
	}

	// Daily cap (effective = min(job, hard, default)).
	cap := j.DailyCap
	if cap <= 0 {
		cap = p.cfg.DailyCap
	}
	if p.cfg.HardDailyCap > 0 && cap > p.cfg.HardDailyCap {
		cap = p.cfg.HardDailyCap
	}
	if p.store != nil && cap > 0 {
		startOfDay := startOfDayLocal(p.now(), "Asia/Ho_Chi_Minh")
		count, err := p.store.CountTodaySends(ctx, credentialID, "", startOfDay)
		if err != nil {
			return "", err
		}
		if count >= cap {
			return SkipReasonCapReached, nil
		}
	}

	// Per-recipient cooldown.
	if p.store != nil && p.cfg.PerRecipientCooldown > 0 && recipient.RecipientPSID != "" {
		last, err := p.store.LastSendTo(ctx, credentialID, recipient.RecipientPSID)
		if err != nil {
			return "", err
		}
		if last != nil && p.now().Sub(*last) < p.cfg.PerRecipientCooldown {
			return SkipReasonCooldown, nil
		}
	}

	return "", nil
}

// contentRule returns SkipReasonContentBlocked or SkipReasonOptOut for any
// banned keyword. Match is case-insensitive substring; users typing the
// keyword as part of a normal sentence WILL be blocked — this is intentional
// (HUMAN_AGENT-tag-equivalent strictness).
func (p *Policy) contentRule(message string) SkipReason {
	low := strings.ToLower(message)
	for _, kw := range p.cfg.Blocklist {
		if kw == "" {
			continue
		}
		if strings.Contains(low, strings.ToLower(kw)) {
			return SkipReasonContentBlocked
		}
	}
	// OptOut keywords are detected on incoming messages, not outgoing — but
	// we keep a guard here to avoid the bot ever responding with one of the
	// recognized opt-out words and confusing recipients.
	for _, kw := range p.cfg.OptOutKeywords {
		if kw == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(low), kw) {
			return SkipReasonOptOut
		}
	}
	return ""
}

// startOfDayLocal returns 00:00 of t in the given IANA TZ. Falls back to UTC
// if tz is invalid.
func startOfDayLocal(t time.Time, tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

// AssertCapValid sanity-checks user-supplied cap values at job creation time.
func AssertCapValid(cap int, hardCap int) error {
	if cap <= 0 {
		return errors.New("daily_cap must be > 0")
	}
	if hardCap > 0 && cap > hardCap {
		return errors.New("daily_cap exceeds hard cap")
	}
	return nil
}
