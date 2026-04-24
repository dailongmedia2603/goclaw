package fbbackfill

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// Protocol method names. camelCase matches upstream convention
// (see pkg/protocol/frames.go method name constants).
const (
	MethodStart  = "fb_backfill.start"
	MethodPause  = "fb_backfill.pause"
	MethodResume = "fb_backfill.resume"
	MethodCancel = "fb_backfill.cancel"
	MethodRetry  = "fb_backfill.retry"
	MethodStatus = "fb_backfill.status"
	MethodList   = "fb_backfill.list"
)

// rpcDeps is the narrow surface the RPC handlers need from the gateway.
// Defined as interfaces so this package does not take a hard import on
// *gateway.Client or *gateway.Server (which would invert the dependency
// graph and violate fork-safety).
type rpcClient interface {
	TenantID() uuid.UUID
	Role() permissions.Role
	UserID() string
	SendResponse(frame *protocol.ResponseFrame)
}

// HandlerFunc matches gateway.MethodHandler.
type HandlerFunc func(ctx context.Context, client rpcClient, req *protocol.RequestFrame)

// RPC bundles the five handler methods on the JobRunner. Construction
// wires the runner + state store; Register attaches each handler to a
// MethodRouter.
type RPC struct {
	runner *JobRunner
	state  *StateStore
}

// NewRPC constructs an RPC facade.
func NewRPC(runner *JobRunner, state *StateStore) *RPC {
	return &RPC{runner: runner, state: state}
}

// authz verifies the caller is tenant-admin or higher on the channel
// instance's tenant. Returns nil if allowed, an error for SendResponse
// if rejected.
func (r *RPC) authz(ctx context.Context, client rpcClient, instanceID uuid.UUID) (*InstanceWithState, error) {
	iws, err := r.state.Load(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	if iws.TenantID != client.TenantID() {
		// Owner role is cross-tenant by design.
		if client.Role() != permissions.RoleOwner {
			return nil, errors.New("forbidden: channel instance belongs to another tenant")
		}
	}
	switch client.Role() {
	case permissions.RoleOwner, permissions.RoleAdmin, permissions.RoleOperator:
		// OK
	default:
		return nil, errors.New("forbidden: backfill control requires operator or admin role")
	}
	return iws, nil
}

// --- Handlers ---

type startParams struct {
	ChannelInstanceID string        `json:"channelInstanceId"`
	MaxConversations  int           `json:"maxConversations,omitempty"`
	SkipExisting      *bool         `json:"skipExisting,omitempty"`
	ForceRecreate     bool          `json:"forceRecreate,omitempty"`
	TriggeredBy       TriggerSource `json:"triggeredBy,omitempty"`
}

// HandleStart implements fb_backfill.start.
func (r *RPC) HandleStart(ctx context.Context, client rpcClient, req *protocol.RequestFrame) {
	var p startParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, err.Error()))
		return
	}
	id, err := uuid.Parse(p.ChannelInstanceID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid channelInstanceId"))
		return
	}
	iws, err := r.authz(ctx, client, id)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, err.Error()))
		return
	}
	skip := true
	if p.SkipExisting != nil {
		skip = *p.SkipExisting
	}
	if p.TriggeredBy == "" {
		p.TriggeredBy = "manual"
	}
	if err := r.runner.Start(ctx, iws.InstanceID, StartOpts{
		MaxConversations: p.MaxConversations,
		SkipExisting:     skip,
		ForceRecreate:    p.ForceRecreate,
		TriggeredBy:      p.TriggeredBy,
	}); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	st, _ := r.state.Get(ctx, iws.InstanceID)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"status": string(stateStatus(st)),
		"state":  st,
	}))
}

type instanceOnlyParams struct {
	ChannelInstanceID string `json:"channelInstanceId"`
}

func (r *RPC) parseInstanceID(req *protocol.RequestFrame) (uuid.UUID, error) {
	var p instanceOnlyParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(p.ChannelInstanceID)
}

// HandlePause implements fb_backfill.pause.
func (r *RPC) HandlePause(ctx context.Context, client rpcClient, req *protocol.RequestFrame) {
	id, err := r.parseInstanceID(req)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid channelInstanceId"))
		return
	}
	if _, err := r.authz(ctx, client, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, err.Error()))
		return
	}
	if err := r.runner.Pause(ctx, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	st, _ := r.state.Get(ctx, id)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"status": string(stateStatus(st))}))
}

// HandleResume implements fb_backfill.resume.
func (r *RPC) HandleResume(ctx context.Context, client rpcClient, req *protocol.RequestFrame) {
	id, err := r.parseInstanceID(req)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid channelInstanceId"))
		return
	}
	if _, err := r.authz(ctx, client, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, err.Error()))
		return
	}
	if err := r.runner.Resume(ctx, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	st, _ := r.state.Get(ctx, id)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"status": string(stateStatus(st))}))
}

// HandleCancel implements fb_backfill.cancel.
func (r *RPC) HandleCancel(ctx context.Context, client rpcClient, req *protocol.RequestFrame) {
	id, err := r.parseInstanceID(req)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid channelInstanceId"))
		return
	}
	if _, err := r.authz(ctx, client, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, err.Error()))
		return
	}
	if err := r.runner.Cancel(ctx, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"status": string(StatusCancelled)}))
}

// HandleRetry implements fb_backfill.retry.
func (r *RPC) HandleRetry(ctx context.Context, client rpcClient, req *protocol.RequestFrame) {
	id, err := r.parseInstanceID(req)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid channelInstanceId"))
		return
	}
	if _, err := r.authz(ctx, client, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, err.Error()))
		return
	}
	if err := r.runner.Retry(ctx, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	st, _ := r.state.Get(ctx, id)
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"status": string(stateStatus(st))}))
}

// HandleStatus implements fb_backfill.status.
func (r *RPC) HandleStatus(ctx context.Context, client rpcClient, req *protocol.RequestFrame) {
	id, err := r.parseInstanceID(req)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid channelInstanceId"))
		return
	}
	if _, err := r.authz(ctx, client, id); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrUnauthorized, err.Error()))
		return
	}
	st, err := r.state.Get(ctx, id)
	if err != nil && !errors.Is(err, ErrNoState) {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"state":   st, // may be null for "never backfilled"
	}))
}

// HandleList implements fb_backfill.list — lists all active/recent jobs
// for the calling client's tenant (owner sees all).
func (r *RPC) HandleList(ctx context.Context, client rpcClient, req *protocol.RequestFrame) {
	all, err := r.state.ListActive(ctx)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, err.Error()))
		return
	}
	callerTenant := client.TenantID()
	isOwner := client.Role() == permissions.RoleOwner
	out := make([]map[string]any, 0)
	for _, iws := range all {
		if !isOwner && iws.TenantID != callerTenant {
			continue
		}
		out = append(out, map[string]any{
			"channelInstanceId": iws.InstanceID.String(),
			"tenantId":          iws.TenantID.String(),
			"name":              iws.Name,
			"state":             iws.State,
		})
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"jobs": out}))
}

func stateStatus(st *BackfillState) JobStatus {
	if st == nil {
		return JobStatus("none")
	}
	return st.Status
}

// Unused-import guard against gofmt complaints when we only use it in
// switch statements.
var _ = store.WithTenantID
