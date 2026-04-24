//go:build !sqliteonly

package fbbackfill

import (
	"context"
	"log/slog"
)

// Deps is the dependency bundle passed from cmd/gateway.go.
//
// Intentionally uses interface{} for cross-package deps so that this
// package does not take a hard import on upstream packages at the
// type level. The Register implementation (added in later phases)
// will type-assert each dep it needs.
//
// This keeps the upstream touch-point in cmd/gateway.go to a single
// function call and preserves upstream-safety even if dep shapes
// change later.
type Deps struct {
	// ChannelInstanceStore is used to read credentials and persist
	// BackfillState into channel_instances.config._backfill.
	// Expected concrete type: store.ChannelInstanceStore.
	ChannelInstanceStore any

	// EpisodicStore is where per-PSID summaries are written.
	// Expected concrete type: store.EpisodicStore.
	EpisodicStore any

	// Router registers WS RPC method handlers.
	// Expected concrete type: *gateway.MethodRouter (has .Register(name, handler)).
	Router any

	// Broadcaster emits progress events to connected WS clients.
	// Expected concrete type: to be determined in phase 5.
	Broadcaster any

	// ProviderResolver resolves the background LLM provider per tenant
	// for summarization. Expected: providerresolve.ResolveBackgroundProvider.
	ProviderResolver any
}

// Register wires fbbackfill into the gateway. Called once at startup from
// cmd/gateway.go — the single upstream touch-point (other than additive UI
// constants).
//
// Phase 0 implementation is a no-op stub that logs a startup marker. Later
// phases plug in the state store, job runner, RPC handlers, and the
// stale-job cleanup pass.
func Register(ctx context.Context, deps *Deps) error {
	if deps == nil {
		slog.Warn("fb_backfill.register.nil_deps")
		return nil
	}
	slog.Info("fb_backfill.register.ok", "phase", "0-skeleton")
	return nil
}
