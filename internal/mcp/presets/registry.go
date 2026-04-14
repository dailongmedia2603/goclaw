package presets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ErrUserModeNotSupported is returned when a preset config asks for
// user_access_token mode, which v1 does not implement end-to-end.
var ErrUserModeNotSupported = errors.New("user_access_token mode is not supported yet — use tenant_access_token")

// ErrPresetNotFound is returned by Get when the preset id is unknown.
var ErrPresetNotFound = errors.New("preset not found")

// PresetMetadata describes a preset for the UI catalog.
type PresetMetadata struct {
	ID          string          `json:"id"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Icon        string          `json:"icon"`
	DocURL      string          `json:"doc_url,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Defaults    json.RawMessage `json:"defaults,omitempty"`
}

// Preset is the contract for a curated MCP server preset.
type Preset interface {
	Metadata() PresetMetadata
	Build(ctx context.Context, rawConfig json.RawMessage, tenantID uuid.UUID, createdBy string) (*store.MCPServerData, error)
	MergeUpdate(ctx context.Context, existing *store.MCPServerData, rawConfig json.RawMessage) (map[string]any, error)
}

var (
	registryMu sync.RWMutex
	registry   = map[string]Preset{}
)

// Register adds a preset to the global registry. Panics on duplicate id.
func Register(p Preset) {
	registryMu.Lock()
	defer registryMu.Unlock()
	id := p.Metadata().ID
	if id == "" {
		panic("presets.Register: empty preset id")
	}
	if _, dup := registry[id]; dup {
		panic(fmt.Sprintf("presets.Register: duplicate preset id %q", id))
	}
	registry[id] = p
}

// Get returns a preset by id.
func Get(id string) (Preset, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := registry[id]
	return p, ok
}

// List returns metadata for all registered presets, sorted by id.
func List() []PresetMetadata {
	registryMu.RLock()
	out := make([]PresetMetadata, 0, len(registry))
	for _, p := range registry {
		out = append(out, p.Metadata())
	}
	registryMu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// resetForTest clears the registry. Only used by tests in this package.
func resetForTest() {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = map[string]Preset{}
}
