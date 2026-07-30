package main

import (
	"strings"
	"testing"
)

func TestLoadConfig_AllRequired(t *testing.T) {
	setEnv(t, map[string]string{
		"FBM_AUTH_TOKEN":      "tok",
		"FBM_HMAC_SECRET":     "sec",
		"FBM_WEBHOOK_URL":     "http://gw/webhook",
		"SYNAPSE_ADMIN_TOKEN": "at",
	})
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.AuthToken != "tok" || cfg.HMACSecret != "sec" || cfg.WebhookURL != "http://gw/webhook" {
		t.Errorf("fields not populated: %+v", cfg)
	}
	if cfg.Port != "29320" {
		t.Errorf("default port wrong: %q", cfg.Port)
	}
	if cfg.SynapseURL != "http://localhost:8008" {
		t.Errorf("default synapse URL wrong: %q", cfg.SynapseURL)
	}
	if cfg.BridgeBotMXID != "@metabot:fbm.local" {
		t.Errorf("default bot MXID wrong: %q", cfg.BridgeBotMXID)
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	setEnv(t, map[string]string{}) // all unset
	_, err := LoadConfigFromEnv()
	if err == nil {
		t.Fatal("expected error on missing vars")
	}
	for _, k := range []string{"FBM_AUTH_TOKEN", "FBM_HMAC_SECRET", "FBM_WEBHOOK_URL", "SYNAPSE_ADMIN_TOKEN"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error should mention %q: %v", k, err)
		}
	}
}

func TestLoadConfig_CustomPort(t *testing.T) {
	setEnv(t, map[string]string{
		"FBM_AUTH_TOKEN":      "t",
		"FBM_HMAC_SECRET":     "s",
		"FBM_WEBHOOK_URL":     "u",
		"SYNAPSE_ADMIN_TOKEN": "a",
		"FBM_PORT":            "9999",
	})
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "9999" {
		t.Errorf("port=%q want=9999", cfg.Port)
	}
}

func setEnv(t *testing.T, vars map[string]string) {
	t.Helper()
	keys := []string{"FBM_AUTH_TOKEN", "FBM_HMAC_SECRET", "FBM_WEBHOOK_URL",
		"SYNAPSE_ADMIN_TOKEN", "FBM_PORT", "SYNAPSE_URL", "BRIDGE_BOT_MXID", "LOG_LEVEL"}
	for _, k := range keys {
		if v, ok := vars[k]; ok {
			t.Setenv(k, v)
		} else {
			t.Setenv(k, "")
		}
	}
}
