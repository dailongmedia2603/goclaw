package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/nhanh"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// nhanhSettings holds the Nhanh.vn API credentials stored in builtin_tools settings.
type nhanhSettings struct {
	AppID        string `json:"app_id"`
	BusinessID   string `json:"business_id"`
	AccessToken  string `json:"access_token"`
	AutoKGIngest bool   `json:"auto_kg_ingest"`
}

// nhanhSettingsToolName is the canonical tool whose settings hold the shared credentials.
const nhanhSettingsToolName = "nhanh_products"

// loadNhanhClient reads credentials from builtin_tools settings and creates an API client.
func loadNhanhClient(ctx context.Context, bts store.BuiltinToolStore) (*nhanh.Client, error) {
	if bts == nil {
		return nil, fmt.Errorf("builtin tool store not available")
	}
	raw, err := bts.GetSettings(ctx, nhanhSettingsToolName)
	if err != nil {
		return nil, fmt.Errorf("failed to read nhanh settings: %w", err)
	}
	var s nhanhSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("invalid nhanh settings: %w", err)
	}
	if s.AppID == "" || s.BusinessID == "" || s.AccessToken == "" {
		return nil, fmt.Errorf("nhanh.vn credentials not configured — go to Settings → Nhanh Products to set App ID, Business ID, and Access Token")
	}
	return nhanh.NewClient(s.AppID, s.BusinessID, s.AccessToken), nil
}

// loadNhanhSettings reads the settings without creating a client.
func loadNhanhSettings(ctx context.Context, bts store.BuiltinToolStore) (*nhanhSettings, error) {
	if bts == nil {
		return nil, fmt.Errorf("builtin tool store not available")
	}
	raw, err := bts.GetSettings(ctx, nhanhSettingsToolName)
	if err != nil {
		return nil, fmt.Errorf("failed to read nhanh settings: %w", err)
	}
	var s nhanhSettings
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("invalid nhanh settings: %w", err)
	}
	return &s, nil
}

// ingestToKG ingests entities and relations into the Knowledge Graph in the background.
func ingestToKG(ctx context.Context, kgStore store.KnowledgeGraphStore, entities []store.Entity, relations []store.Relation) {
	if kgStore == nil || (len(entities) == 0 && len(relations) == 0) {
		return
	}

	agentID := store.AgentIDFromContext(ctx)
	if agentID == uuid.Nil {
		return
	}
	userID := store.KGUserID(ctx)

	// Run ingestion in background to avoid blocking tool response
	bgCtx := context.WithoutCancel(ctx)
	go func() {
		if err := kgStore.IngestExtraction(bgCtx, agentID.String(), userID, entities, relations); err != nil {
			slog.Warn("nhanh: KG ingestion failed", "error", err, "entities", len(entities), "relations", len(relations))
		} else {
			slog.Info("nhanh: KG ingestion complete", "entities", len(entities), "relations", len(relations))
		}
	}()
}

// --- Arg helpers for nhanh tools ---

func stringArg(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func intArg(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	default:
		return 0
	}
}

func intFromString(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
