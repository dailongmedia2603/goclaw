//go:build !sqliteonly

package fbbackfill

import (
	"context"
	"log/slog"
)

// Register brings up the backfill subsystem and wires it into the gateway.
// Called once at gateway startup from cmd/gateway.go — the single upstream
// touch-point that activates this package.
//
// Behavior when optional deps are missing:
//   - RegisterRPC nil → RPC surface not exposed (programmatic use only)
//   - Broadcast nil → progress events discarded
//   - LLMResolver nil → concat path for all conversations (zero LLM cost)
//
// If any required dep is missing, Register logs a warning and becomes a
// no-op — this keeps the gateway startup robust in case of partial wiring.
func Register(ctx context.Context, deps *Deps) error {
	if deps == nil {
		slog.Warn("fb_backfill.register.nil_deps")
		return nil
	}
	if deps.Instances == nil || deps.EpisodicStore == nil {
		slog.Warn("fb_backfill.register.skipped",
			"reason", "missing required dep",
			"has_instances", deps.Instances != nil,
			"has_episodic", deps.EpisodicStore != nil)
		return nil
	}

	stateStore := NewStateStore(deps.Instances)

	// Stale cleanup: any job left in running after a gateway restart is
	// flipped to paused so we do not double-run under HA deployments.
	if n, err := stateStore.MarkStaleAsPaused(ctx); err != nil {
		slog.Warn("fb_backfill.startup_cleanup_failed", "err", err)
	} else if n > 0 {
		slog.Info("fb_backfill.startup.stale_paused", "count", n)
	}

	summarizer := NewSummarizer(deps.EpisodicStore, deps.LLMResolver, deps.SummarizerConfig)

	var emitter EventEmitter
	if deps.Broadcast != nil {
		emitter = NewThrottledEmitter(deps.Broadcast, 0)
	} else {
		emitter = noopEmitter{}
	}

	factory := deps.ClientFactory
	if factory == nil {
		factory = NewDefaultClientFactory()
	}

	runner := NewJobRunner(RunnerDeps{
		StateStore:        stateStore,
		Instances:         deps.Instances,
		ClientFactory:     factory,
		Summarizer:        summarizer,
		Emitter:           emitter,
		MaxConcurrentJobs: deps.MaxConcurrentJobs,
	})

	if deps.RegisterRPC != nil {
		rpc := NewRPC(runner, stateStore)
		deps.RegisterRPC(MethodStart, rpc.HandleStart)
		deps.RegisterRPC(MethodPause, rpc.HandlePause)
		deps.RegisterRPC(MethodResume, rpc.HandleResume)
		deps.RegisterRPC(MethodCancel, rpc.HandleCancel)
		deps.RegisterRPC(MethodRetry, rpc.HandleRetry)
		deps.RegisterRPC(MethodStatus, rpc.HandleStatus)
		deps.RegisterRPC(MethodList, rpc.HandleList)
		slog.Info("fb_backfill.register.ok",
			"rpc", true, "events", deps.Broadcast != nil,
			"llm", deps.LLMResolver != nil)
	} else {
		slog.Info("fb_backfill.register.ok",
			"rpc", false, "events", deps.Broadcast != nil,
			"llm", deps.LLMResolver != nil)
	}

	return nil
}
