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
	"strings"
	"time"
)

// webhookForwarder POSTs signed events to the GoClaw webhook.
type webhookForwarder struct {
	url    string
	secret string
	http   *http.Client
}

func newWebhookForwarder(url, secret string) *webhookForwarder {
	return &webhookForwarder{
		url:    url,
		secret: secret,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SidecarEvent matches the shape expected by GoClaw's event_mapper.go.
// Field names and types MUST stay in sync with sidecarInboundEvent in the main repo.
type SidecarEvent struct {
	APIVersion string          `json:"api_version"`
	EventType  string          `json:"event_type"`
	MessageID  string          `json:"message_id"`
	ThreadID   string          `json:"thread_id"`
	IsGroup    bool            `json:"is_group"`
	SenderID   string          `json:"sender_id"`
	SenderName string          `json:"sender_name,omitempty"`
	Content    string          `json:"content,omitempty"`
	Media      []EventMedia    `json:"media,omitempty"`
	ReplyTo    string          `json:"reply_to,omitempty"`
	Timestamp  int64           `json:"timestamp"`
}

type EventMedia struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Filename    string `json:"filename,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

// Post delivers an event to GoClaw. Retries with simple backoff on transient errors.
func (f *webhookForwarder) Post(ctx context.Context, ev SidecarEvent) error {
	ev.APIVersion = "v1"
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	sig := SignWebhook(body, f.secret, time.Now())

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.url, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fbm-Api-Version", "v1")
		req.Header.Set("X-Fbm-Signature", sig)

		resp, err := f.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		// 4xx = permanent error; don't retry.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			return fmt.Errorf("webhook %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		}
		lastErr = fmt.Errorf("webhook %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}
	if lastErr == nil {
		lastErr = errors.New("webhook failed after retries")
	}
	return lastErr
}

// --- /sync loop: turn Matrix timeline events into SidecarEvents ---

// syncLoop runs forever, pulling Matrix events and forwarding messages.
// The loop maintains a since_token to only fetch new events after restart.
func syncLoop(ctx context.Context, mc *matrixClient, wf *webhookForwarder) {
	since := ""
	for {
		if ctx.Err() != nil {
			return
		}

		syncCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		resp, err := mc.Sync(syncCtx, since, 30000) // 30s long-poll
		cancel()
		if err != nil {
			slog.Warn("sync.error", "err", err)
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}
		since = resp.NextBatch

		// Auto-join any pending portal invites so future messages in those rooms
		// appear in subsequent /sync responses. mautrix-meta creates one portal
		// room per FB thread and invites the admin user; without auto-join the
		// thread stays invisible to the shim.
		for roomID := range resp.Rooms.Invite {
			joinCtx, jcancel := context.WithTimeout(ctx, 10*time.Second)
			if err := mc.JoinRoom(joinCtx, roomID); err != nil {
				slog.Warn("sync.auto_join_failed", "room_id", roomID, "err", err)
			} else {
				slog.Info("sync.auto_joined", "room_id", roomID)
			}
			jcancel()
			// Small delay between joins to avoid Synapse M_LIMIT_EXCEEDED.
			select {
			case <-time.After(500 * time.Millisecond):
			case <-ctx.Done():
				return
			}
		}

		for roomID, roomData := range resp.Rooms.Join {
			// Record room for future resolution.
			threadID := extractThreadIDFromState(roomData.State.Events)
			if threadID != "" {
				mc.RememberThreadRoom(threadID, roomID)
			} else {
				// State events are only delivered once per sync; fall back to cache or
				// fetch the full room state on-demand to recover the thread ID.
				threadID = mc.ThreadIDForRoom(roomID)
				if threadID == "" {
					fetchCtx, fcancel := context.WithTimeout(ctx, 10*time.Second)
					if fetched, err := mc.FetchThreadIDForRoom(fetchCtx, roomID); err != nil {
						slog.Warn("sync.fetch_thread_id_failed", "room_id", roomID, "err", err)
					} else {
						threadID = fetched
					}
					fcancel()
				}
			}

			for _, ev := range roomData.Timeline.Events {
				outEv, ok := translateTimelineEvent(ev, roomID, threadID)
				if !ok {
					continue
				}
				if err := wf.Post(ctx, outEv); err != nil {
					slog.Warn("sync.webhook_post_failed", "err", err, "msg_id", outEv.MessageID)
				}
			}
		}
	}
}

// translateTimelineEvent converts a Matrix m.room.message into a SidecarEvent.
// Returns false for events we don't forward (our own sends, non-message types).
func translateTimelineEvent(ev map[string]any, roomID, threadID string) (SidecarEvent, bool) {
	evType, _ := ev["type"].(string)
	if evType != "m.room.message" {
		return SidecarEvent{}, false
	}
	sender, _ := ev["sender"].(string)
	// Skip echoes from the admin user itself (we're the one sending through the bridge).
	if strings.HasPrefix(sender, "@admin:") {
		return SidecarEvent{}, false
	}
	// Skip the bridge bot's system notices.
	if strings.HasPrefix(sender, "@metabot:") {
		return SidecarEvent{}, false
	}

	content, _ := ev["content"].(map[string]any)
	body, _ := content["body"].(string)
	msgID, _ := ev["event_id"].(string)

	ts := int64(0)
	if tsRaw, ok := ev["origin_server_ts"].(float64); ok {
		ts = int64(tsRaw) / 1000
	}

	// Try to extract FB thread ID from ghost user mxid pattern:
	// mautrix-meta ghost users are typically @meta_<fb_user_id>:fbm.local
	senderFBID := sender
	if idx := strings.Index(sender, "@meta_"); idx == 0 {
		// Strip prefix/suffix.
		rest := strings.TrimPrefix(sender, "@meta_")
		if colon := strings.Index(rest, ":"); colon > 0 {
			senderFBID = rest[:colon]
		}
	}

	return SidecarEvent{
		EventType: "message",
		MessageID: msgID,
		ThreadID:  threadID,
		SenderID:  senderFBID,
		Content:   body,
		Timestamp: ts,
	}, true
}

// extractThreadIDFromState parses m.bridge state events to find the remote thread ID.
// Returns "" if not found.
func extractThreadIDFromState(stateEvents []map[string]any) string {
	for _, ev := range stateEvents {
		evType, _ := ev["type"].(string)
		if evType != "m.bridge" && evType != "uk.half-shot.bridge" {
			continue
		}
		content, _ := ev["content"].(map[string]any)
		channel, _ := content["channel"].(map[string]any)
		if id, ok := channel["id"].(string); ok && id != "" {
			return id
		}
	}
	return ""
}
