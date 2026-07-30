package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// healthHandler reports shim + upstream Synapse readiness.
// Returns 200 only when Synapse /versions responds.
func healthHandler(mc *matrixClient, synapseURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		// Synapse versions endpoint is unauthenticated + cheap.
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, synapseURL+"/_matrix/client/versions", nil)
		resp, err := mc.http.Do(req)
		if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				_ = resp.Body.Close()
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":  "degraded",
				"synapse": "unreachable",
			})
			return
		}
		_ = resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"synapse": "reachable",
		})
	})
}
