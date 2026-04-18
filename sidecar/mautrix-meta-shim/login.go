package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// loginRequest is the body GoClaw posts to /login.
type loginRequest struct {
	// Cookies maps Facebook cookie names to values (c_user, xs, datr, sb, fr).
	Cookies map[string]string `json:"cookies"`
}

type loginResponse struct {
	Success bool   `json:"success"`
	Detail  string `json:"detail,omitempty"`
	RoomID  string `json:"room_id,omitempty"`
}

// loginHandler:
//  1. Creates a Matrix DM room with the bridge bot (metabot).
//  2. Sends "login messenger" command.
//  3. Sends the cookies JSON.
//  4. Waits briefly for the bot's confirmation (polling /sync would be cleaner
//     but /messages is sufficient for the MVP).
//
// The bridge bot persists the session independently — the shim does NOT store cookies.
func loginHandler(mc *matrixClient, botMXID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
			return
		}
		if req.Cookies["c_user"] == "" || req.Cookies["xs"] == "" {
			http.Error(w, `{"error":"cookies must include c_user and xs"}`, http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		roomID, err := mc.CreateDMRoom(ctx, botMXID)
		if err != nil {
			slog.Warn("login.create_dm_failed", "err", err)
			writeLoginErr(w, http.StatusInternalServerError, "create DM room: "+err.Error())
			return
		}
		// Bot auto-joins via appservice — give it a moment.
		time.Sleep(2 * time.Second)

		if _, err := mc.SendText(ctx, roomID, "login messenger"); err != nil {
			slog.Warn("login.send_command_failed", "err", err)
			writeLoginErr(w, http.StatusInternalServerError, "send command: "+err.Error())
			return
		}
		time.Sleep(3 * time.Second)

		cookieJSON, err := json.Marshal(req.Cookies)
		if err != nil {
			writeLoginErr(w, http.StatusInternalServerError, "marshal cookies: "+err.Error())
			return
		}
		if _, err := mc.SendText(ctx, roomID, string(cookieJSON)); err != nil {
			slog.Warn("login.send_cookies_failed", "err", err)
			writeLoginErr(w, http.StatusInternalServerError, "send cookies: "+err.Error())
			return
		}

		slog.Info("login.sent", "room_id", roomID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(loginResponse{
			Success: true,
			Detail:  "cookies forwarded to bridge; check logs for confirmation",
			RoomID:  roomID,
		})
	})
}

func writeLoginErr(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `{"success":false,"detail":%q}`, detail)
}
