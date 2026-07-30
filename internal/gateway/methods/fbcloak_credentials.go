//go:build !sqliteonly

package methods

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/edition"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// FBCloakMethods routes RPC for the fbcloak feature. The Service may be nil
// when the binary was built without the channel package (Lite); handlers
// then short-circuit with FAILED_PRECONDITION + i18n message.
type FBCloakMethods struct {
	service *fbcloak.Service
	cfg     *config.Config
}

// NewFBCloakMethods constructs the router shim. svc may be nil on Lite builds.
func NewFBCloakMethods(svc *fbcloak.Service, cfg *config.Config) *FBCloakMethods {
	return &FBCloakMethods{service: svc, cfg: cfg}
}

// Register wires every Phase 1 method. Phase 2/3/4 add to this list.
func (m *FBCloakMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodFBCloakCredentialsList, m.handleList)
	router.Register(protocol.MethodFBCloakCredentialsAdd, m.handleAdd)
	router.Register(protocol.MethodFBCloakCredentialsTest, m.handleTest)
	router.Register(protocol.MethodFBCloakCredentialsDelete, m.handleDelete)
}

// preflight enforces edition gate, kill-switch, tenant scope, and RBAC. It
// returns a tenant UUID on success or sends an error response and returns
// uuid.Nil + false on failure.
func (m *FBCloakMethods) preflight(ctx context.Context, client *gateway.Client, reqID string) (uuid.UUID, bool) {
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

func (m *FBCloakMethods) handleList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	creds, err := m.service.ListCredentials(ctx, tenantID)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "list")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"credentials": creds,
	}))
}

func (m *FBCloakMethods) handleAdd(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	var params fbcloak.CreateCredentialInput
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
				i18n.T(locale, i18n.MsgInvalidJSON)))
			return
		}
	}
	created, err := m.service.AddCredential(ctx, tenantID, params)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "add")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"credential": created,
	}))
}

func (m *FBCloakMethods) handleTest(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "credential")))
		return
	}
	res, err := m.service.TestCredential(ctx, tenantID, id)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "test")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"result": res,
	}))
}

func (m *FBCloakMethods) handleDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	var params struct {
		ID string `json:"id"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "credential")))
		return
	}
	if err := m.service.DeleteCredential(ctx, tenantID, id); err != nil {
		respondServiceError(client, req.ID, ctx, err, "delete")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"ok": true}))
}

// respondServiceError maps fbcloak typed errors onto wire-protocol error codes
// + i18n messages.
func respondServiceError(client *gateway.Client, reqID string, ctx context.Context, err error, op string) {
	locale := store.LocaleFromContext(ctx)
	switch {
	case errors.Is(err, fbcloak.ErrFeatureDisabled):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakUnavailable)))
	case errors.Is(err, fbcloak.ErrKillswitchActive):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrUnavailable,
			i18n.T(locale, i18n.MsgFBCloakKillswitch)))
	case errors.Is(err, fbcloak.ErrCredentialNotFound), errors.Is(err, fbcloak.ErrJobNotFound):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrNotFound,
			i18n.T(locale, i18n.MsgNotFound, "fbcloak", op)))
	case errors.Is(err, fbcloak.ErrInvalidProxyURL):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgFBCloakInvalidProxy)))
	case errors.Is(err, fbcloak.ErrDisclaimerRequired):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakDisclaimerRequired)))
	case errors.Is(err, fbcloak.ErrOutOfWindow):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgFBCloakOutOfWindow)))
	case errors.Is(err, fbcloak.ErrNoConversationHistory):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrNotFound,
			i18n.T(locale, i18n.MsgFBCloakNoConversation)))
	case errors.Is(err, fbcloak.ErrGraphSenderUnconfigured):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrUnavailable,
			i18n.T(locale, i18n.MsgFBCloakGraphUnavailable)))
	case errors.Is(err, fbcloak.ErrCheckpoint):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakCheckpoint)))
	case errors.Is(err, fbcloak.ErrPlanNotFound):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrNotFound,
			i18n.T(locale, i18n.MsgNotFound, "plan", op)))
	case errors.Is(err, fbcloak.ErrActiveConflict):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakPlanActiveConflict)))
	case errors.Is(err, fbcloak.ErrScheduleTooFar):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgFBCloakPlanScheduleTooFar)))
	case errors.Is(err, fbcloak.ErrPlanTerminal):
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakPlanTerminal)))
	default:
		client.SendResponse(protocol.NewErrorResponse(reqID, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error())))
	}
}
