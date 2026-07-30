package http

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// fbmCredentials is the decrypted shape stored per facebook_personal instance.
type fbmCredentials struct {
	SidecarURL    string `json:"sidecar_url"`
	AuthToken     string `json:"auth_token"`
	WebhookSecret string `json:"webhook_secret"`
}

// fbmLoginRequest is the body the UI wizard POSTs.
type fbmLoginRequest struct {
	Cookies map[string]string `json:"cookies"`
}

// RegisterFBMLoginRoute mounts the Facebook Messenger (Personal) re-auth proxy.
// Path matches the UI fetch in ui/web/src/pages/channels/facebook-messenger/fbm-auth-step.tsx.
func (h *ChannelInstancesHandler) RegisterFBMLoginRoute(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/channels/{id}/facebook_personal/login",
		requireAuth(permissions.RoleAdmin, h.handleFBMLogin))
}

// handleFBMLogin forwards FB cookies from the UI to the sidecar's /login endpoint.
// Auth: admin. The sidecar returns once mautrix-meta accepts the cookies — note that
// downstream FB validation still happens asynchronously (bridge logs confirm).
func (h *ChannelInstancesHandler) handleFBMLogin(w http.ResponseWriter, r *http.Request) {
	locale := store.LocaleFromContext(r.Context())

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidID, "instance"))
		return
	}

	inst, err := h.store.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, protocol.ErrNotFound, i18n.T(locale, i18n.MsgInstanceNotFound))
		return
	}

	if inst.ChannelType != "facebook_personal" {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest, "instance is not a facebook_personal channel")
		return
	}

	var req fbmLoginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest, i18n.T(locale, i18n.MsgInvalidJSON))
		return
	}
	if req.Cookies["c_user"] == "" || req.Cookies["xs"] == "" {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest, "cookies must include c_user and xs")
		return
	}

	var creds fbmCredentials
	if err := json.Unmarshal(inst.Credentials, &creds); err != nil {
		slog.Error("fbm_login.decode_credentials", "err", err)
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal, "credentials malformed")
		return
	}
	if creds.SidecarURL == "" || creds.AuthToken == "" {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest, "sidecar_url and auth_token must be set on the channel")
		return
	}

	body, err := json.Marshal(map[string]any{"cookies": req.Cookies})
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal, "marshal body")
		return
	}

	sidecarURL := strings.TrimRight(creds.SidecarURL, "/") + "/login"
	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, sidecarURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal, "build sidecar request")
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+creds.AuthToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		slog.Warn("fbm_login.sidecar_unreachable", "err", err, "instance", inst.Name)
		writeError(w, http.StatusBadGateway, protocol.ErrInternal, "sidecar unreachable: "+err.Error())
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(respBody)

	slog.Info("fbm_login.forwarded",
		"instance", inst.Name, "status", resp.StatusCode)
}
