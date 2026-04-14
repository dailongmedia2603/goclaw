package presets

import (
	"bytes"
	_ "embed"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// --- Constants ---

const (
	larkDomainInternational = "https://open.larksuite.com"
	larkDomainFeishu        = "https://open.feishu.cn"

	larkTokenModeTenant = "tenant_access_token"
	larkTokenModeUser   = "user_access_token"

	larkDefaultTimeoutSec = 90
	larkMaxTimeoutSec     = 600
	larkMinTimeoutSec     = 10

	larkNpmPackage = "@larksuiteoapi/lark-mcp"
)

var larkValidDomains = map[string]struct{}{
	larkDomainInternational: {},
	larkDomainFeishu:        {},
}

var larkValidToolPresets = map[string]struct{}{
	"preset.default":          {},
	"preset.im.default":       {},
	"preset.calendar.default": {},
	"preset.docs.default":     {},
	"preset.contact.default":  {},
}

var larkAppIDRe = regexp.MustCompile(`^cli_[A-Za-z0-9]+$`)

//go:embed lark_icon.svg
var larkIconSVG []byte

//go:embed lark_schema.json
var larkSchemaJSON []byte

// --- Config ---

// LarkConfig is the canonical form payload for the Lark preset. Stored in
// MCPServerData.Settings.preset_config (AppSecret stripped before storage).
type LarkConfig struct {
	DisplayName string   `json:"display_name,omitempty"`
	AppID       string   `json:"app_id"`
	AppSecret   string   `json:"app_secret,omitempty"` // omitted on update = keep existing
	Domain      string   `json:"domain"`
	TokenMode   string   `json:"token_mode"`
	ToolPresets []string `json:"tool_presets"`
	TimeoutSec  int      `json:"timeout_sec,omitempty"`
	Enabled     bool     `json:"enabled"`
}

// --- Preset implementation ---

type larkPreset struct{}

// NewLarkPreset returns a new Lark preset instance.
func NewLarkPreset() Preset { return larkPreset{} }

func (larkPreset) Metadata() PresetMetadata {
	defaults, _ := json.Marshal(map[string]any{
		"domain":       larkDomainInternational,
		"token_mode":   larkTokenModeTenant,
		"tool_presets": []string{"preset.default"},
		"timeout_sec":  larkDefaultTimeoutSec,
		"enabled":      true,
	})
	icon := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(larkIconSVG)
	return PresetMetadata{
		ID:          "lark",
		DisplayName: "Lark",
		Description: "Connect Goclaw agents to Lark Open Platform (messages, docs, calendar, contacts) using the official @larksuiteoapi/lark-mcp package.",
		Icon:        icon,
		DocURL:      "https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/mcp_integration/mcp_introduction",
		Schema:      json.RawMessage(larkSchemaJSON),
		Defaults:    json.RawMessage(defaults),
	}
}

func (larkPreset) Build(ctx context.Context, raw json.RawMessage, tenantID uuid.UUID, createdBy string) (*store.MCPServerData, error) {
	cfg, err := decodeLarkConfig(raw)
	if err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := cfg.validate(true /*requireSecret*/); err != nil {
		return nil, err
	}
	if cfg.TokenMode == larkTokenModeUser {
		return nil, ErrUserModeNotSupported
	}

	name := slugify(cfg.DisplayName)
	if name == "" {
		name = "lark"
	}

	args := buildLarkArgs(cfg, false /*includeSecretInArgs*/)
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("lark preset: marshal args: %w", err)
	}
	envJSON, err := json.Marshal(map[string]string{
		"LARK_APP_ID":     cfg.AppID,
		"LARK_APP_SECRET": cfg.AppSecret,
	})
	if err != nil {
		return nil, fmt.Errorf("lark preset: marshal env: %w", err)
	}

	settings := map[string]any{
		"preset":                   "lark",
		"preset_config":           cfg.sanitized(),
		"require_user_credentials": false,
	}
	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("lark preset: marshal settings: %w", err)
	}

	displayName := strings.TrimSpace(cfg.DisplayName)
	if displayName == "" {
		displayName = "Lark"
	}

	// TenantID is injected by the store layer from context; don't set here.
	_ = tenantID
	return &store.MCPServerData{
		Name:        name,
		DisplayName: displayName,
		Transport:   "stdio",
		Command:     "npx",
		Args:        argsJSON,
		Env:         envJSON,
		APIKey:      cfg.AppSecret, // store layer encrypts on insert
		ToolPrefix:  "lark",
		TimeoutSec:  cfg.TimeoutSec,
		Settings:    settingsJSON,
		Enabled:     cfg.Enabled,
		CreatedBy:   createdBy,
	}, nil
}

func (larkPreset) MergeUpdate(ctx context.Context, existing *store.MCPServerData, raw json.RawMessage) (map[string]any, error) {
	if existing == nil {
		return nil, fmt.Errorf("lark preset: existing server is nil")
	}
	cfg, err := decodeLarkConfig(raw)
	if err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	// Secret is optional on update — keep existing when empty.
	requireSecret := false
	if err := cfg.validate(requireSecret); err != nil {
		return nil, err
	}
	if cfg.TokenMode == larkTokenModeUser {
		return nil, ErrUserModeNotSupported
	}

	// Resolve effective secret: new value if provided, otherwise keep existing from env.
	effectiveSecret := cfg.AppSecret
	if effectiveSecret == "" {
		// Decode existing env to recover AppSecret (store layer returns decrypted env).
		var existingEnv map[string]string
		if len(existing.Env) > 0 {
			_ = json.Unmarshal(existing.Env, &existingEnv)
		}
		effectiveSecret = existingEnv["LARK_APP_SECRET"]
		if effectiveSecret == "" {
			return nil, fmt.Errorf("lark preset: app_secret missing and no existing secret found")
		}
	}
	cfg.AppSecret = effectiveSecret

	args := buildLarkArgs(cfg, false)
	argsJSON, _ := json.Marshal(args)
	envJSON, _ := json.Marshal(map[string]string{
		"LARK_APP_ID":     cfg.AppID,
		"LARK_APP_SECRET": cfg.AppSecret,
	})
	settings := map[string]any{
		"preset":                   "lark",
		"preset_config":           cfg.sanitized(),
		"require_user_credentials": false,
	}
	settingsJSON, _ := json.Marshal(settings)

	updates := map[string]any{
		"transport":   "stdio",
		"command":     "npx",
		"args":        argsJSON,
		"env":         envJSON,
		"settings":    settingsJSON,
		"enabled":     cfg.Enabled,
		"timeout_sec": cfg.TimeoutSec,
	}
	// Only bump api_key when the user supplied a new secret.
	if raw != nil {
		var probe struct {
			AppSecret *string `json:"app_secret"`
		}
		if err := json.Unmarshal(raw, &probe); err == nil && probe.AppSecret != nil && *probe.AppSecret != "" {
			updates["api_key"] = *probe.AppSecret
		}
	}
	displayName := strings.TrimSpace(cfg.DisplayName)
	if displayName != "" {
		updates["display_name"] = displayName
	}
	return updates, nil
}

// --- Helpers ---

func init() {
	Register(NewLarkPreset())
}

// decodeLarkConfig uses DisallowUnknownFields to reject unexpected fields.
func decodeLarkConfig(raw json.RawMessage) (*LarkConfig, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("lark preset: empty body")
	}
	var cfg LarkConfig
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("lark preset: invalid config: %w", err)
	}
	return &cfg, nil
}

func (cfg *LarkConfig) applyDefaults() {
	if cfg.Domain == "" {
		cfg.Domain = larkDomainInternational
	}
	if cfg.TokenMode == "" {
		cfg.TokenMode = larkTokenModeTenant
	}
	if cfg.TimeoutSec == 0 {
		cfg.TimeoutSec = larkDefaultTimeoutSec
	}
}

func (cfg *LarkConfig) validate(requireSecret bool) error {
	cfg.AppID = strings.TrimSpace(cfg.AppID)
	cfg.AppSecret = strings.TrimSpace(cfg.AppSecret)
	cfg.Domain = strings.TrimSpace(cfg.Domain)
	cfg.TokenMode = strings.TrimSpace(cfg.TokenMode)

	if cfg.AppID == "" {
		return fmt.Errorf("lark preset: app_id is required")
	}
	if !larkAppIDRe.MatchString(cfg.AppID) {
		return fmt.Errorf("lark preset: app_id must match ^cli_[A-Za-z0-9]+$")
	}
	if requireSecret && cfg.AppSecret == "" {
		return fmt.Errorf("lark preset: app_secret is required")
	}
	if _, ok := larkValidDomains[cfg.Domain]; !ok {
		return fmt.Errorf("lark preset: domain must be %q or %q", larkDomainInternational, larkDomainFeishu)
	}
	if cfg.TokenMode != larkTokenModeTenant && cfg.TokenMode != larkTokenModeUser {
		return fmt.Errorf("lark preset: token_mode must be %q or %q", larkTokenModeTenant, larkTokenModeUser)
	}
	if len(cfg.ToolPresets) == 0 {
		return fmt.Errorf("lark preset: tool_presets must have at least one entry")
	}
	seen := make(map[string]struct{}, len(cfg.ToolPresets))
	for _, tp := range cfg.ToolPresets {
		tp = strings.TrimSpace(tp)
		if tp == "" {
			return fmt.Errorf("lark preset: tool_presets contains empty entry")
		}
		if _, ok := larkValidToolPresets[tp]; !ok {
			return fmt.Errorf("lark preset: unknown tool_preset %q", tp)
		}
		if _, dup := seen[tp]; dup {
			return fmt.Errorf("lark preset: duplicate tool_preset %q", tp)
		}
		seen[tp] = struct{}{}
	}
	if cfg.TimeoutSec < larkMinTimeoutSec || cfg.TimeoutSec > larkMaxTimeoutSec {
		return fmt.Errorf("lark preset: timeout_sec must be between %d and %d", larkMinTimeoutSec, larkMaxTimeoutSec)
	}
	if len(cfg.DisplayName) > 80 {
		return fmt.Errorf("lark preset: display_name too long (max 80)")
	}
	return nil
}

// sanitized returns a copy of cfg safe to persist in settings.preset_config
// (no secret, stable field order).
func (cfg *LarkConfig) sanitized() map[string]any {
	return map[string]any{
		"display_name": cfg.DisplayName,
		"app_id":       cfg.AppID,
		"domain":       cfg.Domain,
		"token_mode":   cfg.TokenMode,
		"tool_presets": cfg.ToolPresets,
		"timeout_sec":  cfg.TimeoutSec,
		"enabled":      cfg.Enabled,
	}
}

// buildLarkArgs returns the npx argv for spawning lark-mcp.
// Secret is intentionally NOT passed via args (would leak in /proc/*/cmdline);
// it's delivered via env LARK_APP_SECRET instead.
func buildLarkArgs(cfg *LarkConfig, includeSecretInArgs bool) []string {
	args := []string{
		"-y", larkNpmPackage, "mcp",
		"-a", cfg.AppID,
		"--domain", cfg.Domain,
		"--token-mode", cfg.TokenMode,
		"-t", strings.Join(cfg.ToolPresets, ","),
	}
	if includeSecretInArgs {
		args = append(args, "-s", cfg.AppSecret)
	}
	if cfg.TokenMode == larkTokenModeUser {
		args = append(args, "--oauth")
	}
	return args
}

var slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a display name to a lowercase slug suitable for MCPServerData.Name.
// Falls back to empty string if the input yields nothing usable.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	s = slugNonAlnumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
		s = strings.Trim(s, "-")
	}
	return s
}
