package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/mcp/presets"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// handleListPresets returns metadata for all registered MCP presets.
// Read-only; any authenticated user can list presets to render the catalog.
func (h *MCPHandler) handleListPresets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"presets": presets.List()})
}

// handleCreateFromPreset creates a new MCP server from a preset configuration.
// Admin-only: infrastructure write.
func (h *MCPHandler) handleCreateFromPreset(w http.ResponseWriter, r *http.Request) {
	locale := store.LocaleFromContext(r.Context())
	id := r.PathValue("id")

	preset, ok := presets.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "preset", id)})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	tenantID := store.TenantIDFromContext(r.Context())
	userID := store.UserIDFromContext(r.Context())

	srv, err := preset.Build(r.Context(), body, tenantID, userID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, presets.ErrUserModeNotSupported) {
			status = http.StatusNotImplemented
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	// Resolve name collisions within tenant — append -2, -3…
	if err := h.assignUniquePresetName(r.Context(), srv); err != nil {
		slog.Error("mcp.preset.name_collision", "error", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}

	if err := h.store.CreateServer(r.Context(), srv); err != nil {
		slog.Error("mcp.preset.create_server", "preset", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "mcp_server.created", "mcp_server", srv.ID.String())

	writeJSON(w, http.StatusCreated, sanitizePresetResponse(srv))
}

// handleUpdateFromPreset updates an existing preset-backed server.
// Admin-only.
func (h *MCPHandler) handleUpdateFromPreset(w http.ResponseWriter, r *http.Request) {
	locale := store.LocaleFromContext(r.Context())
	id := r.PathValue("id")
	serverID, err := uuid.Parse(r.PathValue("serverID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidID, "server")})
		return
	}

	preset, ok := presets.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "preset", id)})
		return
	}

	existing, err := h.store.GetServer(r.Context(), serverID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": i18n.T(locale, i18n.MsgNotFound, "server", serverID.String())})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": i18n.T(locale, i18n.MsgInvalidJSON)})
		return
	}

	updates, err := preset.MergeUpdate(r.Context(), existing, body)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, presets.ErrUserModeNotSupported) {
			status = http.StatusNotImplemented
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}

	updates = filterAllowedKeys(updates, mcpServerAllowedFields)

	if err := h.store.UpdateServer(r.Context(), serverID, updates); err != nil {
		slog.Error("mcp.preset.update_server", "preset", id, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Credentials likely changed — evict pool so reconnect picks up new env.
	if h.poolEvictor != nil && existing.Name != "" {
		tid := store.TenantIDFromContext(r.Context())
		h.poolEvictor.Evict(tid, existing.Name)
	}

	h.emitCacheInvalidate()
	emitAudit(h.msgBus, r, "mcp_server.updated", "mcp_server", serverID.String())
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// assignUniquePresetName appends -2, -3… suffixes on name collision within tenant.
// GetServerByName is tenant-scoped (enforced in the store layer), so we only
// need to retry within the caller's tenant context.
func (h *MCPHandler) assignUniquePresetName(ctx context.Context, srv *store.MCPServerData) error {
	base := srv.Name
	if base == "" {
		return errors.New("empty server name")
	}
	for i := 0; i < 50; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i+1)
		}
		existing, err := h.store.GetServerByName(ctx, candidate)
		if err != nil || existing == nil {
			// Not found (or transient read error) — take the slot.
			srv.Name = candidate
			return nil
		}
	}
	return fmt.Errorf("too many name collisions for %q", base)
}

// sanitizePresetResponse strips secrets before returning to client.
// api_key is returned empty; LARK_APP_SECRET (and similar conventional
// secret-y env keys) are redacted in the env JSON.
func sanitizePresetResponse(srv *store.MCPServerData) *store.MCPServerData {
	if srv == nil {
		return nil
	}
	clone := *srv
	clone.APIKey = ""
	if len(clone.Env) > 0 {
		var env map[string]string
		if err := json.Unmarshal(clone.Env, &env); err == nil {
			for k := range env {
				if isLikelySecretKey(k) {
					env[k] = "***"
				}
			}
			if b, err := json.Marshal(env); err == nil {
				clone.Env = b
			}
		}
	}
	return &clone
}

// isLikelySecretKey matches common secret-bearing env variable names.
func isLikelySecretKey(k string) bool {
	switch k {
	case "LARK_APP_SECRET", "APP_SECRET":
		return true
	}
	return false
}
