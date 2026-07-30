//go:build !sqliteonly

package methods

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
	"github.com/nextlevelbuilder/goclaw/internal/channels/fbproactive"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/edition"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// FBCloakPhase4Methods owns the disclaimer + dual-mode router RPC. Kept
// in its own struct so wiring code can opt-in independently — the
// Phase-4 features stay dark on servers that haven't migrated yet.
type FBCloakPhase4Methods struct {
	service *fbcloak.Service
	router  *fbproactive.FBProactiveRouter // optional — nil disables fbcloak.send-proactive
	cfg     *config.Config
}

// NewFBCloakPhase4Methods constructs the handler bundle. The router may
// be nil when the operator hasn't wired Graph API + LastInbound resolver
// yet; in that case fbcloak.send-proactive returns ErrUnavailable.
func NewFBCloakPhase4Methods(service *fbcloak.Service, router *fbproactive.FBProactiveRouter, cfg *config.Config) *FBCloakPhase4Methods {
	return &FBCloakPhase4Methods{service: service, router: router, cfg: cfg}
}

// Register wires the three Phase-4 methods onto the existing router.
func (m *FBCloakPhase4Methods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodFBCloakDisclaimerStatus, m.handleDisclaimerStatus)
	router.Register(protocol.MethodFBCloakDisclaimerAck, m.handleDisclaimerAck)
	router.Register(protocol.MethodFBCloakSendProactive, m.handleSendProactive)
}

// preflight enforces edition gate, kill-switch, RBAC, and tenant scope —
// matches FBCloakMethods.preflight (Phase 1) so every fbcloak entry point
// short-circuits with the same i18n messages and HTTP-equivalent codes.
func (m *FBCloakPhase4Methods) preflight(ctx context.Context, client *gateway.Client, reqID string) (uuid.UUID, bool) {
	locale := store.LocaleFromContext(ctx)

	if !edition.Current().FBCloakEnabled || m.service == nil {
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakUnavailable)))
		return uuid.Nil, false
	}
	if m.service.Killed() {
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrUnavailable,
			i18n.T(locale, i18n.MsgFBCloakKillswitch)))
		return uuid.Nil, false
	}
	if !canSeeAll(client.Role(), m.cfg.Gateway.OwnerIDs, client.UserID()) {
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrUnauthorized,
			i18n.T(locale, i18n.MsgPermissionDenied, "fbcloak")))
		return uuid.Nil, false
	}
	tenantID := store.TenantIDFromContext(ctx)
	if tenantID == uuid.Nil {
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgRequired, "tenant_id")))
		return uuid.Nil, false
	}
	return tenantID, true
}

func (m *FBCloakPhase4Methods) handleDisclaimerStatus(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	status, err := m.service.DisclaimerStatus(ctx, tenantID)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "disclaimer_status")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"status": status}))
}

func (m *FBCloakPhase4Methods) handleDisclaimerAck(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	var params struct {
		Version string `json:"version,omitempty"` // empty → CurrentDisclaimerVersion
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	// Best-effort capture of acking user's UUID. Falls back to nil when
	// the WS context only has a sender_id (non-UUID).
	var userIDPtr *uuid.UUID
	if uid := store.UserIDFromContext(ctx); uid != "" {
		if parsed, perr := uuid.Parse(uid); perr == nil {
			userIDPtr = &parsed
		}
	}
	if err := m.service.AckDisclaimer(ctx, tenantID, userIDPtr, params.Version); err != nil {
		respondServiceError(client, req.ID, ctx, err, "disclaimer_ack")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"ok": true}))
}

func (m *FBCloakPhase4Methods) handleSendProactive(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	if m.router == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnavailable,
			i18n.T(locale, i18n.MsgFBCloakGraphUnavailable)))
		return
	}
	var params struct {
		FanpageID     string `json:"fanpageId"`
		RecipientPSID string `json:"recipientPsid"`
		Message       string `json:"message"`
		// DryRun is a pointer so we can distinguish "field omitted" from
		// "explicitly false". Plan principle: dry-run by default — caller
		// must explicitly opt in to live by sending {"dryRun": false}.
		DryRun *bool `json:"dryRun,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	if params.FanpageID == "" || params.RecipientPSID == "" || params.Message == "" {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgRequired, "fanpageId/recipientPsid/message")))
		return
	}
	dryRun := true
	if params.DryRun != nil {
		dryRun = *params.DryRun
	}
	// Server-enforced disclaimer gate before any backend dispatch.
	dStatus, err := m.service.DisclaimerStatus(ctx, tenantID)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "send_proactive")
		return
	}
	if dStatus.Required {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakDisclaimerRequired)))
		return
	}
	// Bound the call so a stuck browser cannot tie the WS connection up
	// indefinitely. 90s mirrors WaitSendConfirmed's 15s × buffers.
	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	res, err := m.router.SendProactive(cctx, tenantID, params.FanpageID, params.RecipientPSID, params.Message, dryRun)
	if err != nil {
		// Surface router-typed errors via the shared mapper; everything
		// else falls through to ErrInternal.
		if errors.Is(err, fbcloak.ErrOutOfWindow) ||
			errors.Is(err, fbcloak.ErrNoConversationHistory) ||
			errors.Is(err, fbcloak.ErrGraphSenderUnconfigured) ||
			errors.Is(err, fbcloak.ErrDisclaimerRequired) ||
			errors.Is(err, fbcloak.ErrCheckpoint) {
			respondServiceError(client, req.ID, ctx, err, "send_proactive")
			return
		}
		respondServiceError(client, req.ID, ctx, err, "send_proactive")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"result": res}))
}
