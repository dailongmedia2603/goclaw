package fbbackfill

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// fakeRPCClient captures responses so tests can assert the RPC payload.
type fakeRPCClient struct {
	tenantID uuid.UUID
	role     permissions.Role
	userID   string
	last     *protocol.ResponseFrame
}

func (c *fakeRPCClient) TenantID() uuid.UUID       { return c.tenantID }
func (c *fakeRPCClient) Role() permissions.Role    { return c.role }
func (c *fakeRPCClient) UserID() string            { return c.userID }
func (c *fakeRPCClient) SendResponse(f *protocol.ResponseFrame) { c.last = f }

func newRPCHarness(t *testing.T) (*testHarness, *RPC) {
	h := newTestHarness(t)
	return h, NewRPC(h.runner, h.stateStore)
}

func TestRPC_Start_Success(t *testing.T) {
	h, rpc := newRPCHarness(t)
	h.graph.ConvosPages = [][]Conversation{{{ID: "t_1", Participants: participants("PSID1", "PAGE1")}}}
	h.graph.Messages["t_1"] = []Message{{ID: "m", From: ConversationParticipant{ID: "PSID1"}}}

	params, _ := json.Marshal(map[string]any{
		"channelInstanceId": h.instanceID.String(),
		"maxConversations":  100,
		"skipExisting":      true,
		"triggeredBy":       "manual",
	})
	client := &fakeRPCClient{tenantID: h.tenantID, role: permissions.RoleAdmin, userID: "u1"}
	req := &protocol.RequestFrame{ID: "req-1", Method: MethodStart, Params: params}
	rpc.HandleStart(context.Background(), client, req)

	if client.last == nil {
		t.Fatal("no response sent")
	}
	if !client.last.OK {
		t.Errorf("expected ok, got OK=%v err=%+v", client.last.OK, client.last.Error)
	}
	// Wait for job to complete.
	h.runner.Wait(h.instanceID)
	st, _ := h.stateStore.Get(context.Background(), h.instanceID)
	if st.Status != StatusCompleted {
		t.Errorf("expected completed, got %s", st.Status)
	}
}

func TestRPC_Start_InvalidInstanceID(t *testing.T) {
	_, rpc := newRPCHarness(t)
	params, _ := json.Marshal(map[string]any{"channelInstanceId": "not-a-uuid"})
	client := &fakeRPCClient{tenantID: uuid.New(), role: permissions.RoleAdmin}
	req := &protocol.RequestFrame{ID: "r", Method: MethodStart, Params: params}
	rpc.HandleStart(context.Background(), client, req)
	if client.last.OK {
		t.Errorf("expected error on invalid uuid")
	}
}

func TestRPC_Authz_CrossTenantRejected(t *testing.T) {
	h, rpc := newRPCHarness(t)
	// Caller has a different tenant ID than the instance's tenant.
	params, _ := json.Marshal(map[string]any{"channelInstanceId": h.instanceID.String()})
	client := &fakeRPCClient{tenantID: uuid.New(), role: permissions.RoleAdmin, userID: "u1"}
	req := &protocol.RequestFrame{ID: "r", Method: MethodStart, Params: params}
	rpc.HandleStart(context.Background(), client, req)
	if client.last.OK {
		t.Errorf("cross-tenant request should be rejected")
	}
	if client.last.Error == nil {
		t.Errorf("expected error code")
	}
}

func TestRPC_Authz_OwnerCrossTenantAllowed(t *testing.T) {
	h, rpc := newRPCHarness(t)
	h.graph.ConvosPages = [][]Conversation{{}}
	params, _ := json.Marshal(map[string]any{"channelInstanceId": h.instanceID.String()})
	client := &fakeRPCClient{tenantID: uuid.New(), role: permissions.RoleOwner, userID: "owner"}
	req := &protocol.RequestFrame{ID: "r", Method: MethodStart, Params: params}
	rpc.HandleStart(context.Background(), client, req)
	if !client.last.OK {
		t.Errorf("owner should bypass tenant check; got OK=%v err=%+v", client.last.OK, client.last.Error)
	}
}

func TestRPC_Authz_ViewerRejected(t *testing.T) {
	h, rpc := newRPCHarness(t)
	params, _ := json.Marshal(map[string]any{"channelInstanceId": h.instanceID.String()})
	client := &fakeRPCClient{tenantID: h.tenantID, role: permissions.RoleViewer, userID: "u1"}
	req := &protocol.RequestFrame{ID: "r", Method: MethodStart, Params: params}
	rpc.HandleStart(context.Background(), client, req)
	if client.last.OK {
		t.Errorf("viewer role should not be able to start backfill")
	}
}

func TestRPC_Status(t *testing.T) {
	h, rpc := newRPCHarness(t)
	// Seed a state.
	st := NewBackfillState(StartOpts{TriggeredBy: "manual"})
	st.Status = StatusCompleted
	st.ConversationsDone = 7
	if err := h.stateStore.Save(context.Background(), h.instanceID, st); err != nil {
		t.Fatal(err)
	}
	params, _ := json.Marshal(map[string]any{"channelInstanceId": h.instanceID.String()})
	client := &fakeRPCClient{tenantID: h.tenantID, role: permissions.RoleOperator}
	req := &protocol.RequestFrame{ID: "r", Method: MethodStatus, Params: params}
	rpc.HandleStatus(context.Background(), client, req)
	if !client.last.OK {
		t.Fatalf("status call failed: %+v", client.last.Error)
	}
	// Decode result → should contain state.
	resultRaw, _ := json.Marshal(client.last.Payload)
	if !bytesContain(resultRaw, `"conversations_done":7`) {
		t.Errorf("response missing state fields: %s", string(resultRaw))
	}
}

func TestRPC_List_FiltersByTenant(t *testing.T) {
	h, rpc := newRPCHarness(t)

	// Seed 2 instances: one in caller's tenant, one in another tenant.
	otherInst := uuid.New()
	otherTenant := uuid.New()
	creds, _ := json.Marshal(facebookCreds{PageAccessToken: "PAT"})
	h.instances.seed(&store.ChannelInstanceData{
		BaseModel:   store.BaseModel{ID: otherInst},
		TenantID:    otherTenant,
		Name:        "fb-other",
		ChannelType: "facebook",
		Credentials: creds,
		Config:      json.RawMessage(`{"page_id":"PAGE_B","_backfill":{"version":1,"status":"running"}}`),
	})
	ownState := NewBackfillState(StartOpts{TriggeredBy: "manual"})
	ownState.Status = StatusRunning
	if err := h.stateStore.Save(context.Background(), h.instanceID, ownState); err != nil {
		t.Fatal(err)
	}

	client := &fakeRPCClient{tenantID: h.tenantID, role: permissions.RoleAdmin, userID: "u1"}
	req := &protocol.RequestFrame{ID: "r", Method: MethodList, Params: json.RawMessage(`{}`)}
	rpc.HandleList(context.Background(), client, req)
	if !client.last.OK {
		t.Fatalf("list failed: %+v", client.last.Error)
	}
	result, _ := json.Marshal(client.last.Payload)
	// Only the caller's tenant instance should be listed.
	if !bytesContain(result, h.instanceID.String()) {
		t.Errorf("caller's instance missing from list")
	}
	if bytesContain(result, otherInst.String()) {
		t.Errorf("other-tenant instance should be hidden from non-owner")
	}
}

func TestThrottledEmitter_Throttles(t *testing.T) {
	var calls int
	bcast := func(_ string, _ *protocol.EventFrame) { calls++ }
	e := NewThrottledEmitter(bcast, 100*time.Millisecond)

	tid := uuid.New()
	iid := uuid.New()
	st := &BackfillState{Status: StatusRunning}
	// 10 rapid progress emits should collapse to 1.
	for i := 0; i < 10; i++ {
		e.EmitProgress(tid, iid, st)
	}
	if calls != 1 {
		t.Errorf("progress throttled to 1, got %d", calls)
	}

	// Started/paused/completed/failed not throttled.
	calls = 0
	e.EmitStarted(tid, iid)
	e.EmitPaused(tid, iid, "reason")
	e.EmitCompleted(tid, iid, st)
	e.EmitFailed(tid, iid, "err")
	if calls != 4 {
		t.Errorf("lifecycle events should pass through, got %d", calls)
	}
}

func TestThrottledEmitter_NilBroadcaster(t *testing.T) {
	e := NewThrottledEmitter(nil, 0)
	// Must not panic.
	e.EmitStarted(uuid.New(), uuid.New())
	e.EmitProgress(uuid.New(), uuid.New(), &BackfillState{})
}

func bytesContain(b []byte, sub string) bool { return indexOf(string(b), sub) >= 0 }
