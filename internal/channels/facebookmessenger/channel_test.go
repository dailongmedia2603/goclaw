package facebookmessenger

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
)

// --- types tests ---

func TestParseCredentials_Valid(t *testing.T) {
	raw := json.RawMessage(`{
		"sidecar_url": "http://fbm-sidecar:29318/",
		"auth_token": "secret-token",
		"webhook_secret": "hmac-secret"
	}`)
	c, err := parseCredentials(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.SidecarURL != "http://fbm-sidecar:29318" {
		t.Errorf("SidecarURL trim/normalize failed: %q", c.SidecarURL)
	}
	if c.AuthToken != "secret-token" || c.WebhookSecret != "hmac-secret" {
		t.Error("tokens not parsed correctly")
	}
}

func TestParseCredentials_MissingSidecar(t *testing.T) {
	raw := json.RawMessage(`{"auth_token":"x","webhook_secret":"y"}`)
	_, err := parseCredentials(raw)
	if !errors.Is(err, ErrMissingSidecarURL) {
		t.Errorf("expected ErrMissingSidecarURL, got %v", err)
	}
}

func TestParseCredentials_MissingAuthToken(t *testing.T) {
	raw := json.RawMessage(`{"sidecar_url":"http://x","webhook_secret":"y"}`)
	_, err := parseCredentials(raw)
	if !errors.Is(err, ErrMissingAuthToken) {
		t.Errorf("expected ErrMissingAuthToken, got %v", err)
	}
}

func TestParseCredentials_MissingWebhookSecret(t *testing.T) {
	raw := json.RawMessage(`{"sidecar_url":"http://x","auth_token":"y"}`)
	_, err := parseCredentials(raw)
	if !errors.Is(err, ErrMissingSecret) {
		t.Errorf("expected ErrMissingSecret, got %v", err)
	}
}

func TestParseCredentials_InvalidJSON(t *testing.T) {
	_, err := parseCredentials(json.RawMessage(`{not valid`))
	if !errors.Is(err, ErrInvalidCreds) {
		t.Errorf("expected ErrInvalidCreds, got %v", err)
	}
}

func TestParseCredentials_EmptyRaw(t *testing.T) {
	_, err := parseCredentials(nil)
	if !errors.Is(err, ErrInvalidCreds) {
		t.Errorf("expected ErrInvalidCreds on empty, got %v", err)
	}
}

func TestParseConfig_Defaults(t *testing.T) {
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DMPolicy != "pairing" {
		t.Errorf("default DMPolicy want=pairing got=%q", cfg.DMPolicy)
	}
	if cfg.GroupPolicy != "disabled" {
		t.Errorf("default GroupPolicy want=disabled got=%q", cfg.GroupPolicy)
	}
	if cfg.RateLimitPerMin != 20 {
		t.Errorf("default RateLimitPerMin want=20 got=%d", cfg.RateLimitPerMin)
	}
	if cfg.BlockReply != "inherit" {
		t.Errorf("default BlockReply want=inherit got=%q", cfg.BlockReply)
	}
}

func TestParseConfig_Override(t *testing.T) {
	raw := json.RawMessage(`{
		"dm_policy": "allowlist",
		"group_policy": "open",
		"rate_limit_per_min": 10,
		"block_reply": "true",
		"experimental_ack": true,
		"account_label": "Test"
	}`)
	cfg, err := parseConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DMPolicy != "allowlist" ||
		cfg.GroupPolicy != "open" ||
		cfg.RateLimitPerMin != 10 ||
		cfg.BlockReply != "true" ||
		!cfg.ExperimentalAck ||
		cfg.AccountLabel != "Test" {
		t.Errorf("override failed: %+v", cfg)
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	_, err := parseConfig(json.RawMessage(`{bad`))
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

// --- Factory tests ---

func TestFactory_ValidCreds(t *testing.T) {
	creds := json.RawMessage(`{"sidecar_url":"http://sidecar:29318","auth_token":"t","webhook_secret":"s"}`)
	cfg := json.RawMessage(`{"account_label":"alice","rate_limit_per_min":15}`)
	msgBus := bus.New()

	ch, err := Factory("test-fbm", creds, cfg, msgBus, nil)
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	if ch.Type() != channels.TypeFacebookPersonal {
		t.Errorf("Type=%q want=%q", ch.Type(), channels.TypeFacebookPersonal)
	}
	if ch.Name() != "test-fbm" {
		t.Errorf("Name=%q want=%q", ch.Name(), "test-fbm")
	}
}

func TestFactory_InvalidCreds(t *testing.T) {
	_, err := Factory("x", json.RawMessage(`{`), json.RawMessage(`{}`), bus.New(), nil)
	if err == nil {
		t.Error("expected error on malformed creds")
	}
}

func TestFactory_MissingRequired(t *testing.T) {
	_, err := Factory("x", json.RawMessage(`{}`), json.RawMessage(`{}`), bus.New(), nil)
	if err == nil {
		t.Error("expected error when required creds fields missing")
	}
}

// --- Channel lifecycle tests ---

// newTestChannelWithSidecar constructs a Channel pointed at a test httptest.Server.
// Returns channel + mock handler + server URL.
func newTestChannelWithSidecar(t *testing.T) (*Channel, *mockSidecarHandler, string, func()) {
	t.Helper()
	mock := &mockSidecarHandler{}
	srv := httptest.NewServer(mock)
	creds := &Credentials{
		SidecarURL:    srv.URL,
		AuthToken:     "tok",
		WebhookSecret: testSecret,
	}
	cfg := &Config{
		DMPolicy:        "pairing",
		GroupPolicy:     "disabled",
		RateLimitPerMin: 20,
		BlockReply:      "inherit",
	}
	ch := New("test-fbm", creds, cfg, bus.New())
	return ch, mock, srv.URL, srv.Close
}

// newOfflineChannel returns a channel pointed at a definitely-down URL
// (for tests that don't need sidecar interaction).
func newOfflineChannel(t *testing.T) *Channel {
	t.Helper()
	creds := &Credentials{
		SidecarURL:    "http://127.0.0.1:1",
		AuthToken:     "tok",
		WebhookSecret: testSecret,
	}
	cfg := &Config{RateLimitPerMin: 20}
	return New("test-fbm", creds, cfg, bus.New())
}

func TestChannel_StartInitializesClient(t *testing.T) {
	ch, _, _, closeSrv := newTestChannelWithSidecar(t)
	defer closeSrv()
	defer func() { _ = ch.Stop(context.Background()) }()

	if err := ch.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !ch.IsRunning() {
		t.Error("IsRunning should be true after Start")
	}
	// Give health loop a moment — not strictly required, but ensures no panic.
	time.Sleep(50 * time.Millisecond)
}

func TestChannel_StartIdempotent(t *testing.T) {
	ch, _, _, closeSrv := newTestChannelWithSidecar(t)
	defer closeSrv()
	defer func() { _ = ch.Stop(context.Background()) }()

	if err := ch.Start(context.Background()); err != nil {
		t.Fatalf("Start#1: %v", err)
	}
	if err := ch.Start(context.Background()); err != nil {
		t.Errorf("Start#2 should no-op, got %v", err)
	}
}

func TestChannel_StopIdempotent(t *testing.T) {
	ch, _, _, closeSrv := newTestChannelWithSidecar(t)
	defer closeSrv()

	_ = ch.Start(context.Background())
	if err := ch.Stop(context.Background()); err != nil {
		t.Fatalf("Stop#1: %v", err)
	}
	if ch.IsRunning() {
		t.Error("IsRunning should be false after Stop")
	}
	if err := ch.Stop(context.Background()); err != nil {
		t.Errorf("Stop#2 should no-op, got %v", err)
	}
}

func TestChannel_StopWithoutStart(t *testing.T) {
	ch := newOfflineChannel(t)
	if err := ch.Stop(context.Background()); err != nil {
		t.Errorf("Stop without Start should succeed, got %v", err)
	}
}

func TestChannel_Send_NotStarted(t *testing.T) {
	ch := newOfflineChannel(t)
	err := ch.Send(context.Background(), bus.OutboundMessage{})
	if !errors.Is(err, ErrNotStarted) {
		t.Errorf("expected ErrNotStarted, got %v", err)
	}
}

func TestChannel_Send_CallsSidecar(t *testing.T) {
	ch, mock, _, closeSrv := newTestChannelWithSidecar(t)
	defer closeSrv()
	if err := ch.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = ch.Stop(context.Background()) }()

	err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID:  "thread-1",
		Content: "hello world",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if mock.sendCalled != 1 {
		t.Errorf("sendCalled=%d want=1", mock.sendCalled)
	}
}

func TestChannel_Send_RespectsRateLimiter(t *testing.T) {
	ch, mock, _, closeSrv := newTestChannelWithSidecar(t)
	defer closeSrv()
	if err := ch.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = ch.Stop(context.Background()) }()

	// Inject a blocking limiter that always errors.
	sentinelErr := errors.New("limiter denied")
	ch.SetRateLimiter(fakeGate{err: sentinelErr})

	err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "t1", Content: "x"})
	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected rate limiter err, got %v", err)
	}
	if mock.sendCalled != 0 {
		t.Errorf("sidecar should NOT be called when limiter denies, got %d calls", mock.sendCalled)
	}
}

// --- Webhook handler tests ---

func TestChannel_WebhookHandlerPath(t *testing.T) {
	ch := newOfflineChannel(t)
	path, handler := ch.WebhookHandler()
	if !strings.HasPrefix(path, "/channels/facebook_personal/") {
		t.Errorf("webhook path prefix wrong: %q", path)
	}
	if !strings.HasSuffix(path, "/webhook") {
		t.Errorf("webhook path suffix wrong: %q", path)
	}
	if !strings.Contains(path, ch.Name()) {
		t.Errorf("path must contain name: %q", path)
	}
	if handler == nil {
		t.Fatal("handler must not be nil")
	}
}

func TestWebhook_MethodNotAllowed(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET expected 405, got %d", rec.Code)
	}
}

func TestWebhook_SignatureMissing(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing sig expected 401, got %d", rec.Code)
	}
}

func TestWebhook_SignatureInvalid(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	body := `{"event_type":"message"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-Fbm-Signature", "t=1,v1=wrong")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad sig expected 401, got %d", rec.Code)
	}
}

func TestWebhook_BodyTooLarge(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	big := strings.Repeat("x", webhookBodyLimit+1024)
	// Body larger than limit — MaxBytesReader will error on read.
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(big))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversize body expected 400/413, got %d", rec.Code)
	}
}

func TestWebhook_APIVersionMismatch(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	body := []byte(`{"event_type":"message","thread_id":"t","sender_id":"s"}`)
	sig := SignWebhook(body, testSecret, time.Now())

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Fbm-Signature", sig)
	req.Header.Set("X-Fbm-Api-Version", "v9-future")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("version mismatch expected 400, got %d", rec.Code)
	}
}

func TestWebhook_BadJSON(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	body := []byte(`{bad json`)
	sig := SignWebhook(body, testSecret, time.Now())

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Fbm-Signature", sig)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad json expected 400, got %d", rec.Code)
	}
}

func TestWebhook_NonMessageEvent_202(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	body := []byte(`{"event_type":"typing","thread_id":"t","sender_id":"s"}`)
	sig := SignWebhook(body, testSecret, time.Now())
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Fbm-Signature", sig)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("non-message expected 202, got %d", rec.Code)
	}
}

func TestWebhook_EventMissingFields_400(t *testing.T) {
	ch := newOfflineChannel(t)
	_, h := ch.WebhookHandler()
	body := []byte(`{"event_type":"message"}`) // no thread/sender
	sig := SignWebhook(body, testSecret, time.Now())
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Fbm-Signature", sig)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing fields expected 400, got %d", rec.Code)
	}
}

func TestWebhook_ValidMessage_PublishesToBus(t *testing.T) {
	ch := newOfflineChannel(t)
	tenantID := uuid.New()
	ch.BaseChannel.SetTenantID(tenantID)
	_, h := ch.WebhookHandler()

	body := []byte(`{
		"event_type": "message",
		"message_id": "mid.$abc",
		"thread_id": "thread-1",
		"sender_id": "user-1",
		"content": "hello from test"
	}`)
	sig := SignWebhook(body, testSecret, time.Now())
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set("X-Fbm-Signature", sig)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid expected 204, got %d", rec.Code)
	}

	// Consume from bus with a short deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	msg, ok := ch.Bus().ConsumeInbound(ctx)
	if !ok {
		t.Fatal("expected a message on bus")
	}
	if msg.Channel != ch.Name() {
		t.Errorf("channel=%q", msg.Channel)
	}
	if msg.SenderID != "user-1" || msg.ChatID != "thread-1" || msg.Content != "hello from test" {
		t.Errorf("wrong fields: %+v", msg)
	}
	if msg.TenantID != tenantID {
		t.Error("tenantID not propagated")
	}
}

// --- Interface assertion tests (fail-at-compile guard) ---

func TestChannel_ImplementsChannelInterface(t *testing.T) {
	var _ channels.Channel = (*Channel)(nil)
	var _ channels.WebhookChannel = (*Channel)(nil)
}

func TestFactory_MatchesChannelFactoryType(t *testing.T) {
	var _ channels.ChannelFactory = Factory
}

// --- Test helpers ---

type fakeGate struct {
	err error
}

func (g fakeGate) Wait(_ context.Context) error { return g.err }

// sanity helper — drain a reader to avoid dropping Close in edge cases.
var _ = io.Discard
