package facebookmessenger

import (
	"encoding/json"
	"strings"
)

// Credentials is the decrypted JSON payload stored in channel_instances.credentials.
// All fields are secrets except SidecarURL.
type Credentials struct {
	// SidecarURL is the HTTP base URL of the mautrix-meta-shim sidecar (e.g. "http://fbm-sidecar:29318").
	SidecarURL string `json:"sidecar_url"`

	// AuthToken is the Bearer token GoClaw uses to authenticate outbound calls to the sidecar.
	AuthToken string `json:"auth_token"`

	// WebhookSecret is the HMAC-SHA256 key used to verify inbound webhook signatures.
	// Must match the sidecar's FBM_HMAC_SECRET env var.
	WebhookSecret string `json:"webhook_secret"`

	// FBCookies can optionally be pre-populated here for bootstrap.
	// Normally cookies are set via the wizard re-auth flow, which updates credentials
	// via the sidecar's /login endpoint. Stored here for reference / disaster recovery.
	FBCookies *FBCookies `json:"fb_cookies,omitempty"`
}

// FBCookies holds the 4-5 cookies required by mautrix/meta for messenger.com login.
type FBCookies struct {
	CUser string `json:"c_user"`
	Xs    string `json:"xs"`
	Datr  string `json:"datr"`
	Sb    string `json:"sb"`
	Fr    string `json:"fr,omitempty"`
}

// Config is the non-secret JSONB config stored in channel_instances.config.
type Config struct {
	// AccountLabel is a display-only label to distinguish instances (e.g. "Alice's FB").
	AccountLabel string `json:"account_label,omitempty"`

	// DMPolicy: pairing | open | allowlist | disabled (defaults to "pairing").
	DMPolicy string `json:"dm_policy,omitempty"`

	// GroupPolicy: open | pairing | allowlist | disabled (defaults to "disabled" — group chat has higher ban risk).
	GroupPolicy string `json:"group_policy,omitempty"`

	// RateLimitPerMin caps outbound messages per minute per account to mitigate ban risk.
	// Default 20. Recommend ≤ 30.
	RateLimitPerMin int `json:"rate_limit_per_min,omitempty"`

	// BlockReply: inherit | true | false. "inherit" uses gateway-level default.
	BlockReply string `json:"block_reply,omitempty"`

	// ExperimentalAck must be true — signals the tenant has acknowledged the ToS risk warning.
	ExperimentalAck bool `json:"experimental_ack"`
}

// parseCredentials unmarshals a JSON RawMessage and validates required fields.
func parseCredentials(raw json.RawMessage) (*Credentials, error) {
	var c Credentials
	if len(raw) == 0 {
		return nil, ErrInvalidCreds
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, ErrInvalidCreds
	}
	c.SidecarURL = strings.TrimSpace(strings.TrimRight(c.SidecarURL, "/"))
	c.AuthToken = strings.TrimSpace(c.AuthToken)
	c.WebhookSecret = strings.TrimSpace(c.WebhookSecret)
	if c.SidecarURL == "" {
		return nil, ErrMissingSidecarURL
	}
	if c.AuthToken == "" {
		return nil, ErrMissingAuthToken
	}
	if c.WebhookSecret == "" {
		return nil, ErrMissingSecret
	}
	return &c, nil
}

// parseConfig unmarshals config JSON with defaults. Empty raw is allowed.
func parseConfig(raw json.RawMessage) (*Config, error) {
	cfg := &Config{
		DMPolicy:        "pairing",
		GroupPolicy:     "disabled",
		RateLimitPerMin: 20,
		BlockReply:      "inherit",
	}
	if len(raw) == 0 || string(raw) == "null" {
		return cfg, nil
	}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, ErrInvalidConfig
	}
	if cfg.DMPolicy == "" {
		cfg.DMPolicy = "pairing"
	}
	if cfg.GroupPolicy == "" {
		cfg.GroupPolicy = "disabled"
	}
	if cfg.RateLimitPerMin <= 0 {
		cfg.RateLimitPerMin = 20
	}
	if cfg.BlockReply == "" {
		cfg.BlockReply = "inherit"
	}
	return cfg, nil
}
