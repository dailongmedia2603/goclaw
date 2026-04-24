// fbm-mock: a minimal HTTP server that mimics the mautrix-meta-shim API.
// Used in E2E integration tests so we can exercise the gateway's facebook_personal
// channel without running Synapse + mautrix-meta + Matrix.
//
// Behavior:
//   - Accepts /healthz, /send (returns canned IDs)
//   - Accepts POST /mock/emit-event (test-only) → signs + forwards to MOCK_WEBHOOK_URL
//
// NOT production code. Compile with `go build -o fbm-mock .`
package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type env struct {
	authToken    string
	hmacSecret   string
	webhookURL   string
	port         string
	sentMessages []sendRequest
}

type sendRequest struct {
	ChatID  string `json:"chat_id"`
	Content string `json:"content"`
}

func main() {
	e := &env{
		authToken:  os.Getenv("MOCK_AUTH_TOKEN"),
		hmacSecret: os.Getenv("MOCK_HMAC_SECRET"),
		webhookURL: os.Getenv("MOCK_WEBHOOK_URL"),
		port:       getenvDefault("MOCK_PORT", "29318"),
	}
	if e.authToken == "" || e.hmacSecret == "" || e.webhookURL == "" {
		log.Fatal("MOCK_AUTH_TOKEN, MOCK_HMAC_SECRET, MOCK_WEBHOOK_URL required")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", e.health)
	mux.HandleFunc("/send", e.authGate(e.handleSend))
	mux.HandleFunc("/mock/emit-event", e.emit)
	mux.HandleFunc("/mock/sent", e.dump)

	log.Printf("fbm-mock listening on :%s", e.port)
	if err := http.ListenAndServe(":"+e.port, mux); err != nil {
		log.Fatal(err)
	}
}

func (e *env) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (e *env) authGate(next http.HandlerFunc) http.HandlerFunc {
	prefix := "Bearer " + e.authToken
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != prefix {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (e *env) handleSend(w http.ResponseWriter, r *http.Request) {
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad json"}`, http.StatusBadRequest)
		return
	}
	e.sentMessages = append(e.sentMessages, req)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"message_id": "mock-msg-" + strconv.FormatInt(time.Now().UnixNano(), 10),
		"timestamp":  time.Now().Unix(),
	})
}

// emit is a test-only endpoint that signs an event and forwards it to
// MOCK_WEBHOOK_URL (the gateway). Test harness POSTs here to simulate an
// inbound FB message.
func (e *env) emit(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ts := time.Now().Unix()
	signed := fmt.Sprintf("%d.%s", ts, string(body))
	mac := hmac.New(sha256.New, []byte(e.hmacSecret))
	mac.Write([]byte(signed))
	sig := fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))

	req, _ := http.NewRequest(http.MethodPost, e.webhookURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Fbm-Api-Version", "v1")
	req.Header.Set("X-Fbm-Signature", sig)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (e *env) dump(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(e.sentMessages)
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
