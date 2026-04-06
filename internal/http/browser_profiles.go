package http

import (
	"net/http"

	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// BrowserProfilesHandler serves browser profile status to the dashboard.
type BrowserProfilesHandler struct {
	registry *browser.ProfileRegistry
}

// NewBrowserProfilesHandler creates the handler.
func NewBrowserProfilesHandler(registry *browser.ProfileRegistry) *BrowserProfilesHandler {
	return &BrowserProfilesHandler{registry: registry}
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
	}

	var profiles []profileResponse
	for _, p := range h.registry.Profiles() {
		status := p.Manager.Status()
		profiles = append(profiles, profileResponse{
			Name:    p.Name,
			Running: status.Running,
			Tabs:    status.Tabs,
			Shared:  p.Shared,
			Domains: p.Domains,
			VNCURL:  p.VNCURL,
		})
	}

	if profiles == nil {
		profiles = []profileResponse{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
}
