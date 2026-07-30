//go:build !sqliteonly

package fbproactive

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
)

type fakeResolver struct {
	last time.Time
	err  error
}

func (f *fakeResolver) LastInboundAt(_ context.Context, _ uuid.UUID, _, _ string) (time.Time, error) {
	return f.last, f.err
}

type fakeGraph struct {
	called  int
	gotTag  FBProactiveTag
	gotMsg  string
	failErr error
}

func (f *fakeGraph) SendViaGraph(_ context.Context, _ uuid.UUID, _, _, msg string, tag FBProactiveTag) error {
	f.called++
	f.gotTag = tag
	f.gotMsg = msg
	return f.failErr
}

type fakeCloak struct {
	called     int
	gotDryRun  bool
	returnID   string
	returnErr  error
}

func (f *fakeCloak) SendProactive(_ context.Context, _ uuid.UUID, _, _, _ string, dryRun bool) (string, error) {
	f.called++
	f.gotDryRun = dryRun
	return f.returnID, f.returnErr
}

func newRouter(last time.Time) (*FBProactiveRouter, *fakeGraph, *fakeCloak) {
	g := &fakeGraph{}
	c := &fakeCloak{returnID: "send-log-uuid"}
	r := &FBProactiveRouter{
		Resolver: &fakeResolver{last: last},
		Graph:    g,
		Cloak:    c,
		Now:      func() time.Time { return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC) },
	}
	return r, g, c
}

func TestRouter_24h_Response(t *testing.T) {
	r, g, c := newRouter(time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)) // 12h ago
	res, err := r.SendProactive(context.Background(), uuid.New(), "page-1", "psid-1", "hi", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Channel != FBProactiveChannelAPI || res.Tag != FBProactiveTagResponse {
		t.Errorf("got %+v, want api/response", res)
	}
	if g.called != 1 || g.gotTag != FBProactiveTagResponse {
		t.Errorf("graph called=%d tag=%s", g.called, g.gotTag)
	}
	if c.called != 0 {
		t.Errorf("cloak should not be called")
	}
}

func TestRouter_3d_HumanAgent(t *testing.T) {
	r, g, c := newRouter(time.Date(2026, 4, 23, 12, 0, 0, 0, time.UTC)) // 3d ago
	res, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "msg", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Tag != FBProactiveTagHumanAgent {
		t.Errorf("got tag=%s, want human_agent", res.Tag)
	}
	if g.called != 1 || c.called != 0 {
		t.Errorf("graph %d cloak %d", g.called, c.called)
	}
}

func TestRouter_30d_Cloak(t *testing.T) {
	r, g, c := newRouter(time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)) // ~30d
	res, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Channel != FBProactiveChannelCloak {
		t.Errorf("got %+v, want cloak", res)
	}
	if res.SendLogID == "" {
		t.Errorf("expected send log id")
	}
	if g.called != 0 || c.called != 1 {
		t.Errorf("graph %d cloak %d", g.called, c.called)
	}
}

func TestRouter_6m_Cloak(t *testing.T) {
	// Just inside 6 months → still cloak.
	r, _, c := newRouter(time.Date(2025, 10, 30, 12, 0, 0, 0, time.UTC)) // ~178d
	res, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.Channel != FBProactiveChannelCloak {
		t.Errorf("got %+v, want cloak (within 6m)", res)
	}
	if c.called != 1 {
		t.Errorf("cloak called=%d", c.called)
	}
}

func TestRouter_OverYear_OutOfWindow(t *testing.T) {
	r, g, c := newRouter(time.Date(2024, 4, 26, 12, 0, 0, 0, time.UTC)) // ~2y
	_, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if !errors.Is(err, fbcloak.ErrOutOfWindow) {
		t.Errorf("got %v, want ErrOutOfWindow", err)
	}
	if g.called != 0 || c.called != 0 {
		t.Errorf("backends should not be called")
	}
}

func TestRouter_NoHistory(t *testing.T) {
	r := &FBProactiveRouter{
		Resolver: &fakeResolver{}, // zero time → fail
		Graph:    &fakeGraph{},
		Cloak:    &fakeCloak{},
		Now:      func() time.Time { return time.Now() },
	}
	_, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if !errors.Is(err, fbcloak.ErrNoConversationHistory) {
		t.Errorf("got %v, want ErrNoConversationHistory", err)
	}
}

func TestRouter_ResolverError(t *testing.T) {
	wantErr := errors.New("db down")
	r := &FBProactiveRouter{
		Resolver: &fakeResolver{err: wantErr},
		Graph:    &fakeGraph{},
		Cloak:    &fakeCloak{},
		Now:      func() time.Time { return time.Now() },
	}
	_, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if err == nil || !errors.Is(err, wantErr) {
		t.Errorf("got %v, want wraps %v", err, wantErr)
	}
}

func TestRouter_GraphUnconfigured(t *testing.T) {
	r := &FBProactiveRouter{
		Resolver: &fakeResolver{last: time.Now()},
		Graph:    nil, // no graph sender wired
		Cloak:    &fakeCloak{},
		Now:      time.Now,
	}
	_, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if !errors.Is(err, fbcloak.ErrGraphSenderUnconfigured) {
		t.Errorf("got %v, want ErrGraphSenderUnconfigured", err)
	}
}

func TestRouter_GraphErrorWrapped(t *testing.T) {
	wantErr := errors.New("rate limited")
	r := &FBProactiveRouter{
		Resolver: &fakeResolver{last: time.Now().Add(-1 * time.Hour)},
		Graph:    &fakeGraph{failErr: wantErr},
		Cloak:    &fakeCloak{},
		Now:      time.Now,
	}
	_, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want wraps %v", err, wantErr)
	}
}

func TestRouter_RequiredDeps(t *testing.T) {
	r := &FBProactiveRouter{}
	_, err := r.SendProactive(context.Background(), uuid.New(), "p", "psid", "m", false)
	if err == nil {
		t.Fatal("expected error for missing deps")
	}
}

func TestRouter_RequiredArgs(t *testing.T) {
	r, _, _ := newRouter(time.Now())
	cases := []struct {
		tenantID uuid.UUID
		fanpage  string
		psid     string
	}{
		{uuid.Nil, "p", "psid"},
		{uuid.New(), "", "psid"},
		{uuid.New(), "p", ""},
	}
	for _, c := range cases {
		_, err := r.SendProactive(context.Background(), c.tenantID, c.fanpage, c.psid, "m", false)
		if err == nil {
			t.Errorf("expected error for tenant=%v fanpage=%q psid=%q", c.tenantID, c.fanpage, c.psid)
		}
	}
}
