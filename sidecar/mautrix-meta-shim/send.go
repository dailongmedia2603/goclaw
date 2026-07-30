package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// sendRequest: payload posted by GoClaw to /send.
type sendRequest struct {
	ChatID  string         `json:"chat_id"`
	Content string         `json:"content,omitempty"`
	Media   []sendMedia    `json:"media,omitempty"`
	ReplyTo string         `json:"reply_to,omitempty"`
}

type sendMedia struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Caption     string `json:"caption,omitempty"`
}

type sendResponse struct {
	MessageID string `json:"message_id"`
	Timestamp int64  `json:"timestamp"`
}

// sendHandler resolves the Matrix portal room for the given FB thread ID and
// sends the message via Matrix. mautrix-meta picks it up and forwards to FB.
func sendHandler(mc *matrixClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req sendRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
			return
		}
		if req.ChatID == "" {
			http.Error(w, `{"error":"chat_id required"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		roomID, err := mc.FindPortalRoomForThread(ctx, req.ChatID)
		if err != nil {
			if errors.Is(err, errRoomNotFound) {
				// No mapping yet — likely a thread that hasn't been bridged yet.
				// Client should ensure /login succeeded and wait for the thread
				// to appear in /sync before attempting to send.
				slog.Warn("send.no_portal", "chat_id", req.ChatID)
				http.Error(w, `{"error":"portal room not found for chat_id","detail":"the thread has not been synced yet"}`, http.StatusNotFound)
				return
			}
			slog.Warn("send.resolve_failed", "err", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		// MVP: plain text. Media upload to Matrix content repo is Phase 5 hardening.
		body := req.Content
		if body == "" && len(req.Media) > 0 {
			// Fall back to first caption if content empty.
			body = req.Media[0].Caption
		}
		if body == "" {
			body = "(empty message)"
		}

		eventID, err := mc.SendText(ctx, roomID, body)
		if err != nil {
			slog.Warn("send.send_text_failed", "err", err, "room_id", roomID)
			http.Error(w, `{"error":"send failed","detail":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		resp := sendResponse{
			MessageID: eventID,
			Timestamp: time.Now().Unix(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})
}
