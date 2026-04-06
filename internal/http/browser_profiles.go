package http

import (
	"net/http"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// BrowserProfilesHandler serves browser profile status to the dashboard.
// Merges running state from ProfileRegistry with configured state from Config
// so newly-added profiles appear immediately (even before restart).
type BrowserProfilesHandler struct {
	registry *browser.ProfileRegistry
	cfg      *config.Config
}

// NewBrowserProfilesHandler creates the handler.
func NewBrowserProfilesHandler(registry *browser.ProfileRegistry, cfg *config.Config) *BrowserProfilesHandler {
	return &BrowserProfilesHandler{registry: registry, cfg: cfg}
}

// RegisterRoutes registers browser profile routes.
func (h *BrowserProfilesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/browser/profiles", h.auth(h.handleList))
}

func (h *BrowserProfilesHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth("", next)
}

func (h *BrowserProfilesHandler) handleList(w http.ResponseWriter, r *http.Request) {
	type profileResponse struct {
		Name    string   `json:"name"`
		Running bool     `json:"running"`
		Tabs    int      `json:"tabs"`
		Shared  bool     `json:"shared"`
		Domains []string `json:"domains,omitempty"`
		VNCURL  string   `json:"vnc_url,omitempty"`
		Active  bool     `json:"active"` // true = running in registry, false = config-only (needs restart)
	}

	// Build map of running profiles from registry
	runningMap := make(map[string]*browser.Profile)
	for _, p := range h.registry.Profiles() {
		runningMap[p.Name] = p
	}

	// Merge: config profiles (source of truth for what's configured) + registry (running state)
	seen := make(map[string]bool)
	var profiles []profileResponse

	// 1. Show all configured profiles from current config
	for name, pc := range h.cfg.Tools.Browser.ResolvedProfiles() {
		seen[name] = true
		resp := profileResponse{
			Name:    name,
			Shared:  pc.Shared,
			Domains: pc.Domains,
			VNCURL:  pc.VNCURL,
		}
		if rp, ok := runningMap[name]; ok {
			status := rp.Manager.Status()
			resp.Running = status.Running
			resp.Tabs = status.Tabs
			resp.Active = true
			// Use runtime values (may differ from config if config was patched)
			resp.Shared = rp.Shared
			resp.Domains = rp.Domains
			resp.VNCURL = rp.VNCURL
		}
		profiles = append(profiles, resp)
	}

	// 2. Show running profiles not in config (shouldn't happen, but be safe)
	for name, rp := range runningMap {
		if seen[name] {
			continue
		}
		status := rp.Manager.Status()
		profiles = append(profiles, profileResponse{
			Name:    name,
			Running: status.Running,
			Tabs:    status.Tabs,
			Shared:  rp.Shared,
			Domains: rp.Domains,
			VNCURL:  rp.VNCURL,
			Active:  true,
		})
	}

	if profiles == nil {
		profiles = []profileResponse{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
}
