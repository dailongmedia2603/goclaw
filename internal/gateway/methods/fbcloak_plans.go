//go:build !sqliteonly

package methods

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/edition"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// FBCloakPlansMethods handles RPC for Phase 5 Plan-Based Brain Mode.
// Generator and executor are optional — when nil, generate-now / run-due
// short-circuit with ErrUnavailable.
type FBCloakPlansMethods struct {
	service   *fbcloak.Service
	generator *fbcloak.PlanGenerator
	executor  *fbcloak.PlanExecutor
	cfg       *config.Config
}

// NewFBCloakPlansMethods constructs the handler. cfg is used for owner ID
// admin gating; service must be non-nil; generator/executor may be nil
// (then generate-now / run-due RPCs return Unavailable).
func NewFBCloakPlansMethods(svc *fbcloak.Service, gen *fbcloak.PlanGenerator, exec *fbcloak.PlanExecutor, cfg *config.Config) *FBCloakPlansMethods {
	return &FBCloakPlansMethods{service: svc, generator: gen, executor: exec, cfg: cfg}
}

func (m *FBCloakPlansMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodFBCloakPlansList, m.handleList)
	router.Register(protocol.MethodFBCloakPlansGet, m.handleGet)
	router.Register(protocol.MethodFBCloakPlansGenerateNow, m.handleGenerateNow)
	router.Register(protocol.MethodFBCloakPlansCancel, m.handleCancel)
	router.Register(protocol.MethodFBCloakPlansRunDue, m.handleRunDue)
	router.Register(protocol.MethodFBCloakPlansStats, m.handleStats)
}

// preflight enforces the same gate chain Phase 1-4 fbcloak handlers use:
// edition + killswitch + admin RBAC + tenant scope.
func (m *FBCloakPlansMethods) preflight(ctx context.Context, client *gateway.Client, reqID string) (uuid.UUID, bool) {
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
			i18n.T(locale, i18n.MsgPermissionDenied, "fbcloak.plans")))
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

func (m *FBCloakPlansMethods) handleList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	var params struct {
		Status          []string `json:"status,omitempty"`
		CredentialID    string   `json:"credentialId,omitempty"`
		PSID            string   `json:"psid,omitempty"`
		ScheduledAfter  string   `json:"scheduledAfter,omitempty"`
		ScheduledBefore string   `json:"scheduledBefore,omitempty"`
		Limit           int      `json:"limit,omitempty"`
		Offset          int      `json:"offset,omitempty"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}

	filter := fbcloak.PlanFilter{Limit: params.Limit, Offset: params.Offset, PSID: params.PSID}
	for _, s := range params.Status {
		filter.Status = append(filter.Status, fbcloak.PlanStatus(s))
	}
	if params.CredentialID != "" {
		if id, err := uuid.Parse(params.CredentialID); err == nil {
			filter.CredentialID = &id
		}
	}
	if params.ScheduledAfter != "" {
		if t, err := time.Parse(time.RFC3339, params.ScheduledAfter); err == nil {
			filter.ScheduledAfter = t
		}
	}
	if params.ScheduledBefore != "" {
		if t, err := time.Parse(time.RFC3339, params.ScheduledBefore); err == nil {
			filter.ScheduledBefore = t
		}
	}

	plans, total, err := m.service.ListPlans(ctx, tenantID, filter)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "list_plans")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"plans": plans, "total": total,
	}))
}

func (m *FBCloakPlansMethods) handleGet(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
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
			i18n.T(locale, i18n.MsgInvalidID, "plan")))
		return
	}
	plan, err := m.service.GetPlan(ctx, tenantID, id)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "get_plan")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"plan": plan}))
}

func (m *FBCloakPlansMethods) handleGenerateNow(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	if m.generator == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakPlanModelMissing)))
		return
	}
	var params struct {
		CredentialID string `json:"credentialId"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	credID, err := uuid.Parse(params.CredentialID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "credential")))
		return
	}
	// Bound the call — Generator may take 30+ sec for ~50 LLM calls.
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	summary, err := m.generator.RunForCredential(cctx, tenantID, credID)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "generate_now")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"created": summary.Created,
		"skipped": summary.Skipped,
		"errors":  summary.Errors,
	}))
}

func (m *FBCloakPlansMethods) handleCancel(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
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
			i18n.T(locale, i18n.MsgInvalidID, "plan")))
		return
	}
	if err := m.service.CancelPlan(ctx, tenantID, id); err != nil {
		respondServiceError(client, req.ID, ctx, err, "cancel_plan")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"ok": true}))
}

func (m *FBCloakPlansMethods) handleRunDue(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	if m.executor == nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrFailedPrecondition,
			i18n.T(locale, i18n.MsgFBCloakUnavailable)))
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	count, err := m.executor.RunDueForTenant(cctx, tenantID)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "run_due")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"executed": count}))
}

func (m *FBCloakPlansMethods) handleStats(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	stats, err := m.service.PlanStats(ctx, tenantID)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "plan_stats")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"stats": stats}))
}
