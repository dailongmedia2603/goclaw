// mautrix-meta-shim: HTTP bridge between GoClaw and mautrix/meta.
//
// This file reads configuration from environment variables.
// Keep env parsing here so main.go stays focused on orchestration.
package main

import (
	"fmt"
	"os"
	"strings"
)

// Config holds runtime settings derived from env vars.
// All fields are required unless noted.
type Config struct {
	Port              string // shim HTTP port (default 29320)
	AuthToken         string // Bearer token GoClaw presents on inbound calls
	HMACSecret        string // HMAC-SHA256 key for outbound webhook signing
	WebhookURL        string // GoClaw's /channels/facebook_personal/<name>/webhook
	SynapseURL        string // local Synapse base URL (default http://localhost:8008)
	SynapseAdminToken string // admin access token (used by shim's Matrix client)
	BridgeBotMXID     string // bridge bot Matrix ID (default @metabot:fbm.local)
	LogLevel          string // "debug" | "info" | "warn" | "error"
}

// LoadConfigFromEnv reads env vars and validates required fields.
func LoadConfigFromEnv() (*Config, error) {
	cfg := &Config{
		Port:              envOrDefault("FBM_PORT", "29320"),
		AuthToken:         strings.TrimSpace(os.Getenv("FBM_AUTH_TOKEN")),
		HMACSecret:        strings.TrimSpace(os.Getenv("FBM_HMAC_SECRET")),
		WebhookURL:        strings.TrimSpace(os.Getenv("FBM_WEBHOOK_URL")),
		SynapseURL:        envOrDefault("SYNAPSE_URL", "http://localhost:8008"),
		SynapseAdminToken: strings.TrimSpace(os.Getenv("SYNAPSE_ADMIN_TOKEN")),
		BridgeBotMXID:     envOrDefault("BRIDGE_BOT_MXID", "@metabot:fbm.local"),
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
	}

	var missing []string
	if cfg.AuthToken == "" {
		missing = append(missing, "FBM_AUTH_TOKEN")
	}
	if cfg.HMACSecret == "" {
		missing = append(missing, "FBM_HMAC_SECRET")
	}
	if cfg.WebhookURL == "" {
		missing = append(missing, "FBM_WEBHOOK_URL")
	}
	if cfg.SynapseAdminToken == "" {
		missing = append(missing, "SYNAPSE_ADMIN_TOKEN")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
