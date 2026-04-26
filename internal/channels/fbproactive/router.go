//go:build !sqliteonly

// Package fbproactive owns the dual-mode router that callers use to send
// a proactive message to a Facebook user. It chooses between two
// backends based on `last_inbound_at`:
//
//   ≤24h  →  Graph API messaging_type=RESPONSE
//   ≤7d   →  Graph API MESSAGE_TAG / HUMAN_AGENT
//   ≤6m   →  fbcloak browser automation
//   else  →  ErrOutOfWindow
//
// The router lives in its own sub-package (not internal/channels) to
// avoid an import cycle: internal/channels/fbcloak depends on the parent
// `channels` types, so the router cannot import fbcloak from there.
package fbproactive

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
)

// FBProactiveChannel reports which backend served a successful send. The
// caller surfaces this to the UI for transparency ("sent via API" vs
// "sent via Cloak").
type FBProactiveChannel string

const (
	FBProactiveChannelAPI   FBProactiveChannel = "api"
	FBProactiveChannelCloak FBProactiveChannel = "cloak"
)

// FBProactiveTag enumerates the messaging_type/tag the API path used.
// Empty when channel=cloak.
type FBProactiveTag string

const (
	FBProactiveTagResponse   FBProactiveTag = "response"
	FBProactiveTagHumanAgent FBProactiveTag = "human_agent"
	FBProactiveTagNone       FBProactiveTag = ""
)

// FBProactiveResult is the router's response — channel + tag for caller
// visibility, plus an optional sendLogID when the cloak path was used so
// the UI can pull the screenshot.
type FBProactiveResult struct {
	Channel   FBProactiveChannel `json:"channel"`
	Tag       FBProactiveTag     `json:"tag"`
	SendLogID string             `json:"sendLogId,omitempty"`
}

// LastInboundResolver returns when the recipient last messaged the
// fanpage. Implementations query existing message storage (fbm episodic
// summaries / messages table). Return (zero time, ErrNoConversationHistory)
// when no record exists — the router treats that as a hard fail.
type LastInboundResolver interface {
	LastInboundAt(ctx context.Context, tenantID uuid.UUID, fanpageID, recipientPSID string) (time.Time, error)
}

// GraphSender abstracts the fbm Graph API SendViaGraph call. Tag is one
// of `FBProactiveTagResponse` (≤24h) or `FBProactiveTagHumanAgent`. Until
// the fbm channel exposes this, callers leave the field nil and the
// router returns ErrGraphSenderUnconfigured for the API path.
type GraphSender interface {
	SendViaGraph(ctx context.Context, tenantID uuid.UUID, fanpageID, recipientPSID, message string, tag FBProactiveTag) error
}

// CloakSender abstracts the fbcloak.Service entry point. Implementations
// in production wrap fbcloak.Service.SendProactive (added below). dryRun
// flows through to the executor — true skips the actual type/click.
type CloakSender interface {
	SendProactive(ctx context.Context, tenantID uuid.UUID, fanpageID, recipientPSID, message string, dryRun bool) (sendLogID string, err error)
}

// FBProactiveRouter is the unified entry. All four collaborators are
// required for full functionality; nil GraphSender disables only the
// ≤7d path and leaves >7d (cloak) working.
type FBProactiveRouter struct {
	Resolver LastInboundResolver
	Graph    GraphSender // optional — nil → ErrGraphSenderUnconfigured for API path
	Cloak    CloakSender
	Now      func() time.Time // overridable in tests; defaults to time.Now
}

// Window thresholds are exported so tests assert against the same values
// the router uses.
const (
	WindowResponse   = 24 * time.Hour
	WindowHumanAgent = 7 * 24 * time.Hour
	WindowCloakMax   = 6 * 30 * 24 * time.Hour // ~6 months
)

// SendProactive runs the decision tree. Caller receives a clean error
// (ErrNoConversationHistory / ErrOutOfWindow / ErrGraphSenderUnconfigured)
// or the underlying backend error wrapped. Tenant scope flows through —
// every backend receives the same tenantID and is responsible for its
// own SQL guard. dryRun applies to the cloak path only; the Graph API
// path has no equivalent (the API is a real send by definition).
func (r *FBProactiveRouter) SendProactive(ctx context.Context, tenantID uuid.UUID, fanpageID, recipientPSID, message string, dryRun bool) (FBProactiveResult, error) {
	if r.Resolver == nil || r.Cloak == nil {
		return FBProactiveResult{}, errors.New("fb_proactive_router: missing required deps (Resolver/Cloak)")
	}
	if tenantID == uuid.Nil {
		return FBProactiveResult{}, errors.New("fb_proactive_router: tenantID required")
	}
	if fanpageID == "" || recipientPSID == "" {
		return FBProactiveResult{}, errors.New("fb_proactive_router: fanpageID and recipientPSID required")
	}

	last, err := r.Resolver.LastInboundAt(ctx, tenantID, fanpageID, recipientPSID)
	if err != nil {
		return FBProactiveResult{}, fmt.Errorf("resolve last_inbound_at: %w", err)
	}
	if last.IsZero() {
		return FBProactiveResult{}, fbcloak.ErrNoConversationHistory
	}

	now := r.now()
	delta := now.Sub(last)
	switch {
	case delta < 0:
		// Clock skew or future-dated record — treat as ≤24h to be safe.
		return r.sendGraph(ctx, tenantID, fanpageID, recipientPSID, message, FBProactiveTagResponse)
	case delta <= WindowResponse:
		return r.sendGraph(ctx, tenantID, fanpageID, recipientPSID, message, FBProactiveTagResponse)
	case delta <= WindowHumanAgent:
		return r.sendGraph(ctx, tenantID, fanpageID, recipientPSID, message, FBProactiveTagHumanAgent)
	case delta <= WindowCloakMax:
		return r.sendCloak(ctx, tenantID, fanpageID, recipientPSID, message, dryRun)
	default:
		return FBProactiveResult{}, fbcloak.ErrOutOfWindow
	}
}

func (r *FBProactiveRouter) sendGraph(ctx context.Context, tenantID uuid.UUID, fanpageID, psid, msg string, tag FBProactiveTag) (FBProactiveResult, error) {
	if r.Graph == nil {
		return FBProactiveResult{}, fbcloak.ErrGraphSenderUnconfigured
	}
	if err := r.Graph.SendViaGraph(ctx, tenantID, fanpageID, psid, msg, tag); err != nil {
		return FBProactiveResult{}, fmt.Errorf("graph api: %w", err)
	}
	return FBProactiveResult{Channel: FBProactiveChannelAPI, Tag: tag}, nil
}

func (r *FBProactiveRouter) sendCloak(ctx context.Context, tenantID uuid.UUID, fanpageID, psid, msg string, dryRun bool) (FBProactiveResult, error) {
	id, err := r.Cloak.SendProactive(ctx, tenantID, fanpageID, psid, msg, dryRun)
	if err != nil {
		return FBProactiveResult{}, fmt.Errorf("fbcloak: %w", err)
	}
	return FBProactiveResult{Channel: FBProactiveChannelCloak, Tag: FBProactiveTagNone, SendLogID: id}, nil
}

func (r *FBProactiveRouter) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}
