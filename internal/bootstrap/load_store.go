package bootstrap

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// LoadFromStore loads agent-level context files from the agent store (DB).
// Returns files as ContextFile slice ready for system prompt injection.
// Returns nil if no files found or on error.
//
// BOOTSTRAP.md is always filtered out — the onboarding greeting ritual is
// globally disabled. Legacy rows in the store are ignored without being deleted.
func LoadFromStore(ctx context.Context, agentStore store.AgentStore, agentID uuid.UUID) []ContextFile {
	files, err := agentStore.GetAgentContextFiles(ctx, agentID)
	if err != nil {
		slog.Warn("failed to load context files from store", "agent", agentID, "error", err)
		return nil
	}

	var contextFiles []ContextFile
	for _, f := range files {
		if f.Content == "" {
			continue
		}
		if f.FileName == BootstrapFile {
			continue // onboarding disabled — ignore legacy rows
		}
		contextFiles = append(contextFiles, ContextFile{
			Path:    f.FileName,
			Content: f.Content,
		})
	}

	return contextFiles
}
