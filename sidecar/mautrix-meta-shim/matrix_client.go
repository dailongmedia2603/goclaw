package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// matrixClient is a minimal Matrix Client-Server API client, just enough to:
//   - create / resolve DM rooms with the bridge bot
//   - send m.room.message events
//   - /sync for incoming events
//
// We do NOT use a full Matrix SDK — the surface we need is tiny and a dependency
// on e.g. mautrix-go would pull in AGPL indirectly. Hand-rolled stays arms-length.
type matrixClient struct {
	baseURL     string
	accessToken string
	http        *http.Client

	mu          sync.Mutex
	mgmtRoomIDs map[string]string // loginID(FB user ID) → Matrix management room ID (cached)
	roomInfo    map[string]roomInfoCache // Matrix room ID → thread ID + isGroup (cached)
}

// roomInfoCache caches per-room metadata derived from Matrix state events.
type roomInfoCache struct {
	threadID   string
	isGroup    bool
	threadName string            // group title for groups; other party's name for DMs
	ghostNames map[string]string // ghost MXID → display name
}

func newMatrixClient(baseURL, accessToken string) *matrixClient {
	return &matrixClient{
		baseURL:     baseURL,
		accessToken: accessToken,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
		mgmtRoomIDs: make(map[string]string),
		roomInfo:    make(map[string]roomInfoCache),
	}
}

// --- Low-level helpers ---

func (c *matrixClient) authReq(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

// do executes a request, decodes JSON into out (if non-nil), returns error on non-2xx.
func (c *matrixClient) do(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB safety cap
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("matrix API status=%d path=%s body=%s",
			resp.StatusCode, req.URL.Path, truncate(string(body), 400))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(body, out)
}

// --- Room helpers ---

type createRoomResp struct {
	RoomID string `json:"room_id"`
}

// CreateDMRoom creates a Matrix room inviting the bridge bot and marks it is_direct.
func (c *matrixClient) CreateDMRoom(ctx context.Context, botMXID string) (string, error) {
	req, err := c.authReq(ctx, http.MethodPost, "/_matrix/client/v3/createRoom", map[string]any{
		"invite":    []string{botMXID},
		"is_direct": true,
		"preset":    "trusted_private_chat",
	})
	if err != nil {
		return "", err
	}
	var out createRoomResp
	if err := c.do(req, &out); err != nil {
		return "", err
	}
	return out.RoomID, nil
}

// SendText sends an m.text message to a room. Returns the event ID.
func (c *matrixClient) SendText(ctx context.Context, roomID, text string) (string, error) {
	txnID := "shim-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", roomID, txnID)
	req, err := c.authReq(ctx, http.MethodPut, path, map[string]any{
		"msgtype": "m.text",
		"body":    text,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		EventID string `json:"event_id"`
	}
	if err := c.do(req, &out); err != nil {
		return "", err
	}
	return out.EventID, nil
}

// JoinedRooms returns the Matrix rooms the shim's user (admin) is in.
func (c *matrixClient) JoinedRooms(ctx context.Context) ([]string, error) {
	req, err := c.authReq(ctx, http.MethodGet, "/_matrix/client/v3/joined_rooms", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		JoinedRooms []string `json:"joined_rooms"`
	}
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return out.JoinedRooms, nil
}

// --- Sync ---

// SyncResponse is a minimal subset of Matrix /sync — only fields we actually consume.
type SyncResponse struct {
	NextBatch string `json:"next_batch"`
	Rooms     struct {
		Join map[string]struct {
			Timeline struct {
				Events []map[string]any `json:"events"`
			} `json:"timeline"`
			State struct {
				Events []map[string]any `json:"events"`
			} `json:"state"`
		} `json:"join"`
		Invite map[string]struct {
			InviteState struct {
				Events []map[string]any `json:"events"`
			} `json:"invite_state"`
		} `json:"invite"`
	} `json:"rooms"`
}

// JoinRoom accepts a pending room invite.
func (c *matrixClient) JoinRoom(ctx context.Context, roomID string) error {
	req, err := c.authReq(ctx, http.MethodPost, "/_matrix/client/v3/rooms/"+roomID+"/join", map[string]any{})
	if err != nil {
		return err
	}
	var out map[string]any
	return c.do(req, &out)
}

// Sync performs a /sync with long-polling timeout (ms).
// since="" means initial sync.
func (c *matrixClient) Sync(ctx context.Context, since string, timeoutMs int) (*SyncResponse, error) {
	q := fmt.Sprintf("?timeout=%d", timeoutMs)
	if since != "" {
		q += "&since=" + since
	}
	req, err := c.authReq(ctx, http.MethodGet, "/_matrix/client/v3/sync"+q, nil)
	if err != nil {
		return nil, err
	}
	var out SyncResponse
	if err := c.do(req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

var errRoomNotFound = errors.New("room not found for thread")

// FindPortalRoomForThread searches joined rooms for the one mautrix-meta created
// for the given FB thread ID. The mautrix-meta portal room has an m.bridge state
// event with network id "facebook" and a remote-id matching the FB thread.
//
// In practice we accept any joined room that isn't the management room as a
// candidate, and let the bridge bot's send-by-thread semantics resolve ambiguity.
// For Phase 4 MVP we maintain a manual mapping populated via /send bookkeeping.
func (c *matrixClient) FindPortalRoomForThread(ctx context.Context, threadID string) (string, error) {
	_ = ctx // reserved for future resolver
	c.mu.Lock()
	defer c.mu.Unlock()
	if roomID, ok := c.mgmtRoomIDs[threadID]; ok {
		return roomID, nil
	}
	return "", errRoomNotFound
}

// RememberThreadRoom stores a known thread→room mapping (populated from /sync events).
// Does not set isGroup — callers who know the flag should use RememberRoomInfo.
func (c *matrixClient) RememberThreadRoom(threadID, roomID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mgmtRoomIDs[threadID] = roomID
	prev := c.roomInfo[roomID]
	prev.threadID = threadID
	c.roomInfo[roomID] = prev
}

// RememberRoomInfo caches both thread ID and group flag for a room.
// threadName and ghostNames are optional — pass "" / nil to keep defaults.
func (c *matrixClient) RememberRoomInfo(roomID, threadID string, isGroup bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if threadID != "" {
		c.mgmtRoomIDs[threadID] = roomID
	}
	prev := c.roomInfo[roomID]
	c.roomInfo[roomID] = roomInfoCache{
		threadID:   threadID,
		isGroup:    isGroup,
		threadName: prev.threadName,
		ghostNames: prev.ghostNames,
	}
}

// RememberRoomInfoFull stores all room metadata atomically (used by FetchRoomInfo).
func (c *matrixClient) RememberRoomInfoFull(roomID, threadID string, isGroup bool, threadName string, ghostNames map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if threadID != "" {
		c.mgmtRoomIDs[threadID] = roomID
	}
	c.roomInfo[roomID] = roomInfoCache{
		threadID:   threadID,
		isGroup:    isGroup,
		threadName: threadName,
		ghostNames: ghostNames,
	}
}

// RoomInfoCached returns cached (threadID, isGroup, ok) for a room.
func (c *matrixClient) RoomInfoCached(roomID string) (string, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ri, ok := c.roomInfo[roomID]
	return ri.threadID, ri.isGroup, ok && ri.threadID != ""
}

// RoomMetadata returns threadName + a shallow copy of the ghost name map.
// Empty threadName / nil map if the room hasn't been fetched yet.
func (c *matrixClient) RoomMetadata(roomID string) (threadName string, ghostNames map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ri, ok := c.roomInfo[roomID]
	if !ok {
		return "", nil
	}
	names := make(map[string]string, len(ri.ghostNames))
	for k, v := range ri.ghostNames {
		names[k] = v
	}
	return ri.threadName, names
}

// ThreadIDForRoom returns a cached FB thread ID for a Matrix room, or "" if unknown.
func (c *matrixClient) ThreadIDForRoom(roomID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.roomInfo[roomID].threadID
}

// FetchRoomInfo fetches /rooms/{id}/state (for threadID) and /joined_members
// (for isGroup) and derives both.
// - threadID: from m.bridge / uk.half-shot.bridge state event's channel.id.
// - isGroup: true when the room has 3+ distinct mautrix-meta ghost users
//   (@meta_<fb_id>:...). DM portals have exactly 2 ghosts (self + other);
//   FB groups require ≥ 3 participants so group portals have ≥ 3 ghosts.
// Result is cached. Returns ("", false, nil) when the room has no bridge state.
func (c *matrixClient) FetchRoomInfo(ctx context.Context, roomID string) (string, bool, error) {
	if tid, grp, ok := c.RoomInfoCached(roomID); ok {
		return tid, grp, nil
	}
	// 1. Scan state for m.bridge (threadID).
	stateReq, err := c.authReq(ctx, http.MethodGet, "/_matrix/client/v3/rooms/"+roomID+"/state", nil)
	if err != nil {
		return "", false, err
	}
	var events []map[string]any
	if err := c.do(stateReq, &events); err != nil {
		return "", false, err
	}
	var threadID, threadName string
	for _, ev := range events {
		evType, _ := ev["type"].(string)
		switch evType {
		case "m.bridge", "uk.half-shot.bridge":
			content, _ := ev["content"].(map[string]any)
			channel, _ := content["channel"].(map[string]any)
			if id, ok := channel["id"].(string); ok && id != "" {
				threadID = id
			}
			if name, _ := channel["displayname"].(string); name != "" {
				threadName = name
			}
		case "m.room.name":
			content, _ := ev["content"].(map[string]any)
			if name, _ := content["name"].(string); name != "" && threadName == "" {
				threadName = name
			}
		}
	}

	// 2. Fetch joined members for ghost count + display names.
	membersReq, err := c.authReq(ctx, http.MethodGet, "/_matrix/client/v3/rooms/"+roomID+"/joined_members", nil)
	if err != nil {
		return "", false, err
	}
	var memResp struct {
		Joined map[string]struct {
			DisplayName string `json:"display_name"`
		} `json:"joined"`
	}
	if err := c.do(membersReq, &memResp); err != nil {
		slog.Warn("matrix.joined_members_failed", "room_id", roomID, "err", err)
		if threadID != "" {
			c.RememberRoomInfo(roomID, threadID, false)
		}
		return threadID, false, nil
	}
	ghostCount := 0
	ghostNames := make(map[string]string)
	for mxid, info := range memResp.Joined {
		if strings.HasPrefix(mxid, "@meta_") {
			ghostCount++
			if info.DisplayName != "" {
				ghostNames[mxid] = info.DisplayName
			}
		}
	}
	isGroup := ghostCount >= 3

	if threadID != "" {
		c.RememberRoomInfoFull(roomID, threadID, isGroup, threadName, ghostNames)
	}
	return threadID, isGroup, nil
}

// FetchThreadIDForRoom is a thin wrapper around FetchRoomInfo for callers that
// only care about the thread ID.
func (c *matrixClient) FetchThreadIDForRoom(ctx context.Context, roomID string) (string, error) {
	tid, _, err := c.FetchRoomInfo(ctx, roomID)
	return tid, err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
