package personal

import (
	"context"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels/zalo/personal/protocol"
	"github.com/nextlevelbuilder/goclaw/internal/config"
)

// A group the account merely belongs to (warmed by ListGroups on Start) must
// not bypass the "pairing" group policy.
func TestKnownGroup_DoesNotBypassPairing(t *testing.T) {
	msgBus := bus.New()
	ch, err := New(config.ZaloPersonalConfig{GroupPolicy: "pairing"}, msgBus, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch.markKnownGroup("g1")

	if ch.checkGroupPolicy(context.Background(), "u1", "g1") {
		t.Fatal("known but unpaired group passed the pairing policy")
	}
	if ch.IsGroupApproved("g1") {
		t.Fatal("markKnownGroup must not mark the group as pairing-approved")
	}

	ch.handleGroupMessage(protocol.NewGroupMessage("bot", protocol.TGroupMessage{
		TMessage: protocol.TMessage{MsgID: "m1", UIDFrom: "u1", IDTo: "g1", Content: textContent("hi")},
	}))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if got, ok := msgBus.ConsumeInbound(ctx); ok {
		t.Fatalf("unexpected inbound for unpaired group: %+v", got)
	}
}

// allow_from may list group IDs (UI: "Zalo user IDs or group IDs"): under the
// "allowlist" group policy a listed group is open to every member, while other
// groups stay closed unless the sender is listed.
func TestGroupAllowlist_AcceptsGroupIDs(t *testing.T) {
	ch := newTestChannel(t, config.ZaloPersonalConfig{
		GroupPolicy: "allowlist",
		AllowFrom:   config.FlexibleStringSlice{"g1", "u-vip"},
	})
	ctx := context.Background()

	if !ch.checkGroupPolicy(ctx, "anyone", "g1") {
		t.Fatal("member of allowlisted group g1 was rejected")
	}
	if ch.checkGroupPolicy(ctx, "anyone", "g2") {
		t.Fatal("non-allowlisted group g2 was accepted")
	}
	if !ch.checkGroupPolicy(ctx, "u-vip", "g2") {
		t.Fatal("allowlisted user was rejected in another group")
	}

	prefixed := newTestChannel(t, config.ZaloPersonalConfig{
		GroupPolicy: "allowlist",
		AllowFrom:   config.FlexibleStringSlice{"group:g3"},
	})
	if !prefixed.checkGroupPolicy(ctx, "anyone", "g3") {
		t.Fatal(`"group:g3" entry did not allow group g3`)
	}
}

func TestResolveThreadType(t *testing.T) {
	ch := newTestChannel(t, config.ZaloPersonalConfig{})

	if got := ch.resolveThreadType("u1", nil); got != protocol.ThreadTypeUser {
		t.Fatalf("unknown id: got %v, want user", got)
	}

	ch.markKnownGroup("g1")
	if got := ch.resolveThreadType("g1", nil); got != protocol.ThreadTypeGroup {
		t.Fatalf("known group: got %v, want group", got)
	}

	if got := ch.resolveThreadType("g2", map[string]string{"group_id": "g2"}); got != protocol.ThreadTypeGroup {
		t.Fatalf("group_id metadata: got %v, want group", got)
	}
	if got := ch.resolveThreadType("g2", nil); got != protocol.ThreadTypeGroup {
		t.Fatal("group_id metadata should be remembered for later sends")
	}
	if ch.IsGroupApproved("g2") {
		t.Fatal("an outbound send must not mark the group as pairing-approved")
	}

	ch.MarkGroupApproved("g3")
	if got := ch.resolveThreadType("g3", nil); got != protocol.ThreadTypeGroup {
		t.Fatalf("approved group: got %v, want group", got)
	}
}
