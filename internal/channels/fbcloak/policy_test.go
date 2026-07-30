//go:build !sqliteonly

package fbcloak

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakePolicyStore struct {
	count    int
	lastSend *time.Time
	err      error
}

func (f *fakePolicyStore) CountTodaySends(_ context.Context, _ uuid.UUID, _ string, _ time.Time) (int, error) {
	return f.count, f.err
}

func (f *fakePolicyStore) LastSendTo(_ context.Context, _ uuid.UUID, _ string) (*time.Time, error) {
	return f.lastSend, f.err
}

func TestPolicy_BlockedKeywords(t *testing.T) {
	p := NewPolicy(DefaultPolicyConfig(), nil)
	bad := []string{
		"Hôm nay có khuyến mãi đặc biệt",
		"Big SALE today",
		"Free voucher",
		"deal of the day",
		"đăng ký ngay",
	}
	for _, msg := range bad {
		t.Run(strings.ReplaceAll(msg, " ", "_"), func(t *testing.T) {
			r, err := p.AllowSend(t.Context(), Job{}, uuid.New(), Target{RecipientPSID: "X"}, msg)
			if err != nil {
				t.Fatal(err)
			}
			if r != SkipReasonContentBlocked {
				t.Errorf("expected content_blocked for %q, got %q", msg, r)
			}
		})
	}
}

func TestPolicy_AllowsCleanMessage(t *testing.T) {
	p := NewPolicy(DefaultPolicyConfig(), nil)
	r, err := p.AllowSend(t.Context(), Job{}, uuid.New(), Target{RecipientPSID: "X"}, "Chào anh, dạo này anh khoẻ không ạ?")
	if err != nil {
		t.Fatal(err)
	}
	if r != "" {
		t.Errorf("expected empty (allow), got %q", r)
	}
}

func TestPolicy_TooLong(t *testing.T) {
	p := NewPolicy(PolicyConfig{DailyCap: 30, MaxMessageLen: 50}, nil)
	r, _ := p.AllowSend(t.Context(), Job{}, uuid.New(), Target{}, strings.Repeat("a", 60))
	if r != SkipReasonTooLong {
		t.Errorf("expected too_long, got %q", r)
	}
}

func TestPolicy_DailyCap_Reached(t *testing.T) {
	store := &fakePolicyStore{count: 30}
	p := NewPolicy(PolicyConfig{DailyCap: 30, HardDailyCap: 50, MaxMessageLen: 500}, store)
	r, err := p.AllowSend(t.Context(), Job{DailyCap: 30}, uuid.New(), Target{RecipientPSID: "X"}, "ok")
	if err != nil {
		t.Fatal(err)
	}
	if r != SkipReasonCapReached {
		t.Errorf("expected cap_reached, got %q", r)
	}
}

func TestPolicy_DailyCap_NotReached(t *testing.T) {
	store := &fakePolicyStore{count: 5}
	p := NewPolicy(PolicyConfig{DailyCap: 30, HardDailyCap: 50, MaxMessageLen: 500}, store)
	r, _ := p.AllowSend(t.Context(), Job{DailyCap: 30}, uuid.New(), Target{RecipientPSID: "X"}, "ok")
	if r != "" {
		t.Errorf("expected allow, got %q", r)
	}
}

func TestPolicy_Cooldown_Active(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-1 * 24 * time.Hour)
	store := &fakePolicyStore{count: 0, lastSend: &yesterday}
	p := NewPolicy(PolicyConfig{DailyCap: 30, HardDailyCap: 50, MaxMessageLen: 500, PerRecipientCooldown: 30 * 24 * time.Hour}, store)
	p.SetClock(func() time.Time { return now })
	r, _ := p.AllowSend(t.Context(), Job{DailyCap: 30}, uuid.New(), Target{RecipientPSID: "X"}, "ok")
	if r != SkipReasonCooldown {
		t.Errorf("expected cooldown, got %q", r)
	}
}

func TestPolicy_Cooldown_Expired(t *testing.T) {
	now := time.Now()
	longAgo := now.Add(-60 * 24 * time.Hour)
	store := &fakePolicyStore{count: 0, lastSend: &longAgo}
	p := NewPolicy(PolicyConfig{DailyCap: 30, HardDailyCap: 50, MaxMessageLen: 500, PerRecipientCooldown: 30 * 24 * time.Hour}, store)
	p.SetClock(func() time.Time { return now })
	r, _ := p.AllowSend(t.Context(), Job{DailyCap: 30}, uuid.New(), Target{RecipientPSID: "X"}, "ok")
	if r != "" {
		t.Errorf("expected allow after cooldown expired, got %q", r)
	}
}

func TestPolicy_HardCapClamps(t *testing.T) {
	p := NewPolicy(PolicyConfig{DailyCap: 100, HardDailyCap: 50, MaxMessageLen: 500}, &fakePolicyStore{count: 60})
	// Effective cap should be 50 (hard cap), so 60 sends today → cap_reached.
	r, _ := p.AllowSend(t.Context(), Job{DailyCap: 100}, uuid.New(), Target{RecipientPSID: "X"}, "ok")
	if r != SkipReasonCapReached {
		t.Errorf("expected cap_reached after hard cap clamp, got %q", r)
	}
}

func TestAssertCapValid(t *testing.T) {
	if err := AssertCapValid(30, 50); err != nil {
		t.Errorf("30 within 50: unexpected err %v", err)
	}
	if err := AssertCapValid(0, 50); err == nil {
		t.Error("zero cap should error")
	}
	if err := AssertCapValid(60, 50); err == nil {
		t.Error("60 over 50 should error")
	}
}

func TestStartOfDayLocal(t *testing.T) {
	got := startOfDayLocal(time.Date(2026, 4, 26, 14, 30, 0, 0, time.UTC), "UTC")
	want := time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("startOfDayLocal: got %s, want %s", got, want)
	}
}
