// mautrix-meta-shim main entry point.
// Starts HTTP API + background Matrix /sync → webhook forwarder.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	setupLogger(cfg.LogLevel)

	slog.Info("fbm-sidecar starting",
		"port", cfg.Port,
		"synapse_url", cfg.SynapseURL,
		"bot_mxid", cfg.BridgeBotMXID,
	)

	mc := newMatrixClient(cfg.SynapseURL, cfg.SynapseAdminToken)
	wf := newWebhookForwarder(cfg.WebhookURL, cfg.HMACSecret)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Matrix /sync → webhook forwarder loop.
	go syncLoop(ctx, mc, wf)

	// HTTP server.
	mux := http.NewServeMux()
	mux.Handle("/healthz", requireAuth(cfg.AuthToken, healthHandler(mc, cfg.SynapseURL)))
	mux.Handle("/login", requireAuth(cfg.AuthToken, loginHandler(mc, cfg.BridgeBotMXID)))
	mux.Handle("/send", requireAuth(cfg.AuthToken, sendHandler(mc)))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown.
	done := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		slog.Info("shutdown signal received")
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		_ = srv.Shutdown(shutdownCtx)
		close(done)
	}()

	slog.Info("HTTP listening", "port", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
	}
	<-done
	slog.Info("fbm-sidecar exited")
}

func setupLogger(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}
