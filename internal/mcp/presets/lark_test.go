package presets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// newMCPServerDataFromParts builds a minimal MCPServerData for MergeUpdate tests.
func newMCPServerDataFromParts(name string, env, settings json.RawMessage) *store.MCPServerData {
	return &store.MCPServerData{
		Name:      name,
		Transport: "stdio",
		Command:   "npx",
		Env:       env,
		Settings:  settings,
	}
}

var testTenant = uuid.New()

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func validLarkPayload() map[string]any {
	return map[string]any{
		"display_name": "Lark Prod",
		"app_id":       "cli_abc123",
		"app_secret":   "secret-xyz",
		"domain":       "https://open.larksuite.com",
		"token_mode":   "tenant_access_token",
		"tool_presets": []string{"preset.default"},
		"timeout_sec":  90,
		"enabled":      true,
	}
}

func TestLarkPreset_Metadata(t *testing.T) {
	m := NewLarkPreset().Metadata()
	if m.ID != "lark" {
		t.Fatalf("expected id=lark, got %q", m.ID)
	}
	if m.DisplayName == "" {
		t.Fatalf("display_name should not be empty")
	}
	if len(m.Schema) == 0 {
		t.Fatalf("schema should be embedded")
	}
	if !strings.HasPrefix(m.Icon, "data:image/svg+xml;base64,") {
		t.Fatalf("icon should be base64 svg data uri, got %q", m.Icon)
	}
	if len(m.Defaults) == 0 {
		t.Fatalf("defaults should be embedded")
	}
}

func TestLarkPreset_Build_Tenant(t *testing.T) {
	data, err := NewLarkPreset().Build(context.Background(), mustJSON(validLarkPayload()), testTenant, "admin-user")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if data.Transport != "stdio" {
		t.Errorf("transport=%q, want stdio", data.Transport)
	}
	if data.Command != "npx" {
		t.Errorf("command=%q, want npx", data.Command)
	}
	if data.APIKey != "secret-xyz" {
		t.Errorf("api_key=%q, want secret-xyz (store layer will encrypt)", data.APIKey)
	}
	// TenantID is injected by the store via context; not set on the struct itself.
	if data.CreatedBy != "admin-user" {
		t.Errorf("created_by=%q, want admin-user", data.CreatedBy)
	}
	if data.Name != "lark-prod" {
		t.Errorf("slugified name=%q, want lark-prod", data.Name)
	}
	if data.DisplayName != "Lark Prod" {
		t.Errorf("display_name=%q, want Lark Prod", data.DisplayName)
	}
	if data.ToolPrefix != "lark" {
		t.Errorf("tool_prefix=%q, want lark", data.ToolPrefix)
	}
	if data.TimeoutSec != 90 {
		t.Errorf("timeout_sec=%d, want 90", data.TimeoutSec)
	}

	// Args assertions
	var args []string
	if err := json.Unmarshal(data.Args, &args); err != nil {
		t.Fatalf("unmarshal args: %v", err)
	}
	argsStr := strings.Join(args, "|")
	for _, needle := range []string{"-y", "@larksuiteoapi/lark-mcp", "mcp", "-a", "cli_abc123", "--domain", "https://open.larksuite.com", "--token-mode", "tenant_access_token", "-t", "preset.default"} {
		if !strings.Contains(argsStr, needle) {
			t.Errorf("args missing %q, got %v", needle, args)
		}
	}
	if strings.Contains(argsStr, "secret-xyz") {
		t.Errorf("AppSecret must NOT appear in args (leaks via /proc/*/cmdline): %v", args)
	}
	if strings.Contains(argsStr, "--oauth") {
		t.Errorf("tenant mode must not include --oauth flag: %v", args)
	}

	// Env assertions
	var env map[string]string
	if err := json.Unmarshal(data.Env, &env); err != nil {
		t.Fatalf("unmarshal env: %v", err)
	}
	if env["LARK_APP_ID"] != "cli_abc123" {
		t.Errorf("LARK_APP_ID=%q", env["LARK_APP_ID"])
	}
	if env["LARK_APP_SECRET"] != "secret-xyz" {
		t.Errorf("LARK_APP_SECRET=%q", env["LARK_APP_SECRET"])
	}

	// Settings assertions
	var settings map[string]any
	if err := json.Unmarshal(data.Settings, &settings); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	if settings["preset"] != "lark" {
		t.Errorf("settings.preset=%v, want lark", settings["preset"])
	}
	pcfg, ok := settings["preset_config"].(map[string]any)
	if !ok {
		t.Fatalf("preset_config missing or wrong type: %T", settings["preset_config"])
	}
	if _, has := pcfg["app_secret"]; has {
		t.Errorf("preset_config must NOT contain app_secret")
	}
	if pcfg["app_id"] != "cli_abc123" {
		t.Errorf("preset_config.app_id=%v, want cli_abc123", pcfg["app_id"])
	}
}

func TestLarkPreset_Build_FeishuDomain(t *testing.T) {
	payload := validLarkPayload()
	payload["domain"] = "https://open.feishu.cn"
	data, err := NewLarkPreset().Build(context.Background(), mustJSON(payload), testTenant, "admin")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var args []string
	_ = json.Unmarshal(data.Args, &args)
	if !strings.Contains(strings.Join(args, "|"), "https://open.feishu.cn") {
		t.Errorf("feishu domain not propagated: %v", args)
	}
}

func TestLarkPreset_Build_UserMode_NotSupported(t *testing.T) {
	payload := validLarkPayload()
	payload["token_mode"] = "user_access_token"
	_, err := NewLarkPreset().Build(context.Background(), mustJSON(payload), testTenant, "admin")
	if !errors.Is(err, ErrUserModeNotSupported) {
		t.Fatalf("expected ErrUserModeNotSupported, got %v", err)
	}
}

func TestLarkPreset_Build_EmptyDisplayNameDefaultsToLark(t *testing.T) {
	payload := validLarkPayload()
	delete(payload, "display_name")
	data, err := NewLarkPreset().Build(context.Background(), mustJSON(payload), testTenant, "admin")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if data.Name != "lark" {
		t.Errorf("name=%q, want lark", data.Name)
	}
	if data.DisplayName != "Lark" {
		t.Errorf("display_name=%q, want Lark", data.DisplayName)
	}
}

func TestLarkPreset_Build_AppliesDefaults(t *testing.T) {
	payload := map[string]any{
		"app_id":       "cli_xyz",
		"app_secret":   "s",
		"tool_presets": []string{"preset.default"},
		"enabled":      true,
	}
	data, err := NewLarkPreset().Build(context.Background(), mustJSON(payload), testTenant, "admin")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if data.TimeoutSec != larkDefaultTimeoutSec {
		t.Errorf("default timeout not applied: %d", data.TimeoutSec)
	}
	var args []string
	_ = json.Unmarshal(data.Args, &args)
	if !strings.Contains(strings.Join(args, "|"), larkDomainInternational) {
		t.Errorf("default domain not applied: %v", args)
	}
}

func TestLarkPreset_Build_ValidationErrors(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(m map[string]any)
		wantField string
	}{
		{"missing_app_id", func(m map[string]any) { delete(m, "app_id") }, "app_id"},
		{"empty_app_id", func(m map[string]any) { m["app_id"] = "" }, "app_id"},
		{"bad_app_id_format", func(m map[string]any) { m["app_id"] = "not_cli_format" }, "app_id"},
		{"missing_app_secret", func(m map[string]any) { delete(m, "app_secret") }, "app_secret"},
		{"empty_app_secret", func(m map[string]any) { m["app_secret"] = "" }, "app_secret"},
		{"unknown_domain", func(m map[string]any) { m["domain"] = "https://evil.example.com" }, "domain"},
		{"bad_token_mode", func(m map[string]any) { m["token_mode"] = "hacker" }, "token_mode"},
		{"no_tool_presets", func(m map[string]any) { m["tool_presets"] = []string{} }, "tool_presets"},
		{"unknown_tool_preset", func(m map[string]any) { m["tool_presets"] = []string{"preset.evil"} }, "tool_preset"},
		{"duplicate_tool_preset", func(m map[string]any) {
			m["tool_presets"] = []string{"preset.default", "preset.default"}
		}, "duplicate"},
		{"timeout_too_low", func(m map[string]any) { m["timeout_sec"] = 1 }, "timeout_sec"},
		{"timeout_too_high", func(m map[string]any) { m["timeout_sec"] = 9999 }, "timeout_sec"},
		{"display_name_too_long", func(m map[string]any) { m["display_name"] = strings.Repeat("x", 81) }, "display_name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload := validLarkPayload()
			c.mutate(payload)
			_, err := NewLarkPreset().Build(context.Background(), mustJSON(payload), testTenant, "admin")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.wantField)
			}
			if !strings.Contains(err.Error(), c.wantField) {
				t.Errorf("error %q should mention %q", err.Error(), c.wantField)
			}
		})
	}
}

func TestLarkPreset_Build_RejectUnknownFields(t *testing.T) {
	payload := validLarkPayload()
	payload["evil_field"] = "bad"
	_, err := NewLarkPreset().Build(context.Background(), mustJSON(payload), testTenant, "admin")
	if err == nil {
		t.Fatalf("expected error on unknown field")
	}
}

func TestLarkPreset_MergeUpdate_KeepsExistingSecretWhenEmpty(t *testing.T) {
	existingEnv, _ := json.Marshal(map[string]string{
		"LARK_APP_ID":     "cli_old",
		"LARK_APP_SECRET": "old-secret",
	})
	existingSettings, _ := json.Marshal(map[string]any{"preset": "lark"})
	existing := newMCPServerDataFromParts("lark", existingEnv, existingSettings)

	payload := validLarkPayload()
	delete(payload, "app_secret")

	updates, err := NewLarkPreset().MergeUpdate(context.Background(), existing, mustJSON(payload))
	if err != nil {
		t.Fatalf("MergeUpdate: %v", err)
	}
	if _, has := updates["api_key"]; has {
		t.Errorf("empty app_secret must NOT touch api_key, got updates[api_key]=%v", updates["api_key"])
	}
	// Env in updates must carry the RESTORED secret so subprocess still works.
	var env map[string]string
	_ = json.Unmarshal(updates["env"].([]byte), &env)
	if env["LARK_APP_SECRET"] != "old-secret" {
		t.Errorf("env secret lost: %v", env)
	}
}

func TestLarkPreset_MergeUpdate_RotatesSecret(t *testing.T) {
	existing := newMCPServerDataFromParts("lark", mustJSON(map[string]string{
		"LARK_APP_ID":     "cli_old",
		"LARK_APP_SECRET": "old-secret",
	}), mustJSON(map[string]any{"preset": "lark"}))

	payload := validLarkPayload()
	payload["app_secret"] = "new-secret"

	updates, err := NewLarkPreset().MergeUpdate(context.Background(), existing, mustJSON(payload))
	if err != nil {
		t.Fatalf("MergeUpdate: %v", err)
	}
	if updates["api_key"] != "new-secret" {
		t.Errorf("api_key not rotated: %v", updates["api_key"])
	}
	var env map[string]string
	_ = json.Unmarshal(updates["env"].([]byte), &env)
	if env["LARK_APP_SECRET"] != "new-secret" {
		t.Errorf("env secret not rotated: %v", env)
	}
}

func TestLarkPreset_MergeUpdate_NilExisting(t *testing.T) {
	_, err := NewLarkPreset().MergeUpdate(context.Background(), nil, mustJSON(validLarkPayload()))
	if err == nil {
		t.Fatalf("expected error on nil existing")
	}
}

func TestLarkPreset_MergeUpdate_RejectsUserMode(t *testing.T) {
	existing := newMCPServerDataFromParts("lark", mustJSON(map[string]string{
		"LARK_APP_ID": "cli_x", "LARK_APP_SECRET": "s",
	}), mustJSON(map[string]any{"preset": "lark"}))
	payload := validLarkPayload()
	payload["token_mode"] = "user_access_token"
	_, err := NewLarkPreset().MergeUpdate(context.Background(), existing, mustJSON(payload))
	if !errors.Is(err, ErrUserModeNotSupported) {
		t.Fatalf("expected ErrUserModeNotSupported, got %v", err)
	}
}

// --- slugify helper ---

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Lark Prod":     "lark-prod",
		"Lark — Prod!":  "lark-prod",
		"   ":           "",
		"":              "",
		"ALL-CAPS":      "all-caps",
		"abc__def":      "abc-def",
		strings.Repeat("a", 60): strings.Repeat("a", 40),
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q)=%q want %q", in, got, want)
		}
	}
}

// --- Registry ---

func TestRegistry_ListIncludesLark(t *testing.T) {
	got := List()
	found := false
	for _, m := range got {
		if m.ID == "lark" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("lark preset not registered")
	}
}

func TestRegistry_GetReturnsLark(t *testing.T) {
	p, ok := Get("lark")
	if !ok {
		t.Fatalf("Get(lark) not found")
	}
	if p.Metadata().ID != "lark" {
		t.Fatalf("wrong preset returned: %v", p.Metadata())
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	if _, ok := Get("nope"); ok {
		t.Fatalf("expected Get(nope) to return false")
	}
}

func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate register")
		}
	}()
	Register(NewLarkPreset()) // already registered via init
}
