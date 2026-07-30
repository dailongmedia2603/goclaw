//go:build !sqliteonly

package methods

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/channels/fbcloak"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// RegisterJobs wires Phase 2+3 RPC methods onto the same router as
// FBCloakMethods. Invoked by the wire helper after credentials handlers.
func (m *FBCloakMethods) RegisterJobs(router *gateway.MethodRouter) {
	router.Register(protocol.MethodFBCloakJobsList, m.handleJobsList)
	router.Register(protocol.MethodFBCloakJobsCreate, m.handleJobsCreate)
	router.Register(protocol.MethodFBCloakJobsToggle, m.handleJobsToggle)
	router.Register(protocol.MethodFBCloakJobsDelete, m.handleJobsDelete)
	router.Register(protocol.MethodFBCloakJobsRunNow, m.handleJobsRunNow)
	router.Register(protocol.MethodFBCloakLogList, m.handleLogList)
	router.Register(protocol.MethodFBCloakLogScreenshot, m.handleLogScreenshot)
}

func (m *FBCloakMethods) handleJobsList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	jobs, err := m.service.ListJobs(ctx, tenantID)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "list_jobs")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"jobs": jobs}))
}

func (m *FBCloakMethods) handleJobsCreate(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	var params fbcloak.CreateJobInput
	if req.Params != nil {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
				i18n.T(locale, i18n.MsgInvalidJSON)))
			return
		}
	}
	created, err := m.service.CreateJob(ctx, tenantID, params)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "create_job")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"job": created}))
}

func (m *FBCloakMethods) handleJobsToggle(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	var params struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	id, err := uuid.Parse(params.ID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "job")))
		return
	}
	if err := m.service.ToggleJob(ctx, tenantID, id, params.Enabled); err != nil {
		respondServiceError(client, req.ID, ctx, err, "toggle_job")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"ok": true}))
}

func (m *FBCloakMethods) handleJobsDelete(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
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
			i18n.T(locale, i18n.MsgInvalidID, "job")))
		return
	}
	if err := m.service.DeleteJob(ctx, tenantID, id); err != nil {
		respondServiceError(client, req.ID, ctx, err, "delete_job")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"ok": true}))
}

func (m *FBCloakMethods) handleJobsRunNow(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
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
			i18n.T(locale, i18n.MsgInvalidID, "job")))
		return
	}
	status, err := m.service.RunJobNow(ctx, tenantID, id)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "run_job_now")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"status": string(status)}))
}

// handleLogList lists fbcloak_send_log rows with optional filters.
// Phase-3 expanded the param surface: status (sent|dry_run|skipped|failed),
// fromDate/toDate (RFC3339), offset for pagination. All filters are
// optional; tenant scope is server-enforced via preflight.
func (m *FBCloakMethods) handleLogList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	var params struct {
		JobID    string `json:"jobId,omitempty"`
		Status   string `json:"status,omitempty"`
		FromDate string `json:"fromDate,omitempty"` // RFC3339
		ToDate   string `json:"toDate,omitempty"`   // RFC3339
		Limit    int    `json:"limit"`
		Offset   int    `json:"offset"`
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	opts := fbcloak.SendLogFilter{Limit: params.Limit, Offset: params.Offset}
	if params.JobID != "" {
		if id, err := uuid.Parse(params.JobID); err == nil {
			opts.JobID = &id
		}
	}
	if params.Status != "" {
		opts.Status = fbcloak.SendStatus(params.Status)
	}
	if params.FromDate != "" {
		if t, err := time.Parse(time.RFC3339, params.FromDate); err == nil {
			opts.FromDate = t
		}
	}
	if params.ToDate != "" {
		if t, err := time.Parse(time.RFC3339, params.ToDate); err == nil {
			opts.ToDate = t
		}
	}
	logs, err := m.service.ListSendLogFiltered(ctx, tenantID, opts)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "list_log")
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"logs": logs}))
}

// handleLogScreenshot returns the on-disk path for a send-log screenshot
// (pre|post|checkpoint). The UI must POST the path to /v1/files/sign to
// receive a signed URL — this RPC deliberately does NOT mint URLs to keep
// the WS layer stateless. Tenant scope verified via preflight + the
// SendLog row tenant_id check inside GetSendLog.
func (m *FBCloakMethods) handleLogScreenshot(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	tenantID, ok := m.preflight(ctx, client, req.ID)
	if !ok {
		return
	}
	locale := store.LocaleFromContext(ctx)
	var params struct {
		SendLogID string `json:"sendLogId"`
		Kind      string `json:"kind"` // pre | post | checkpoint
	}
	if req.Params != nil {
		_ = json.Unmarshal(req.Params, &params)
	}
	id, err := uuid.Parse(params.SendLogID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "sendLog")))
		return
	}
	log, err := m.service.GetSendLog(ctx, tenantID, id)
	if err != nil {
		respondServiceError(client, req.ID, ctx, err, "get_send_log")
		return
	}
	var path string
	switch params.Kind {
	case "pre":
		if log.ScreenshotPre != nil {
			path = *log.ScreenshotPre
		}
	case "post":
		if log.ScreenshotPost != nil {
			path = *log.ScreenshotPost
		}
	default:
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidRequest, "kind must be 'pre' or 'post'")))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"path": path}))
}
