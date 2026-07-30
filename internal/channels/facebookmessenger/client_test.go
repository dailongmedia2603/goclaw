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
)

type mockSidecarHandler struct {
	lastAuthHeader string
	lastBody       []byte
	status         int
	responseBody   string
	sendCalled     int
	healthCalled   int
}

func (m *mockSidecarHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.lastAuthHeader = r.Header.Get("Authorization")
	switch r.URL.Path {
	case "/healthz":
		m.healthCalled++
		status := m.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
	case "/send":
		m.sendCalled++
		body, _ := io.ReadAll(r.Body)
		m.lastBody = body
		status := m.status
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		resp := m.responseBody
		if resp == "" {
			resp = `{"message_id":"mock-msg-1","timestamp":1700000000}`
		}
		_, _ = io.WriteString(w, resp)
	default:
		http.NotFound(w, r)
	}
}

func TestClient_Send_Success(t *testing.T) {
	mock := &mockSidecarHandler{}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	client := newSidecarClient(srv.URL, "my-token")
	resp, err := client.Send(context.Background(), sendRequest{ChatID: "c1", Content: "hi"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if resp.MessageID != "mock-msg-1" {
		t.Errorf("MessageID=%q want=mock-msg-1", resp.MessageID)
	}
	if mock.lastAuthHeader != "Bearer my-token" {
		t.Errorf("auth header=%q want=Bearer my-token", mock.lastAuthHeader)
	}
	if mock.sendCalled != 1 {
		t.Errorf("sendCalled=%d want=1", mock.sendCalled)
	}
}

func TestClient_Send_BodyIncludesFields(t *testing.T) {
	mock := &mockSidecarHandler{}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	client := newSidecarClient(srv.URL, "tok")
	_, err := client.Send(context.Background(), sendRequest{
		ChatID:  "thread123",
		Content: "hello",
		Media:   []mediaUpload{{URL: "https://x/a.jpg", ContentType: "image/jpeg"}},
		ReplyTo: "msg42",
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	var parsed sendRequest
	if err := json.Unmarshal(mock.lastBody, &parsed); err != nil {
		t.Fatalf("body unmarshal: %v", err)
	}
	if parsed.ChatID != "thread123" || parsed.Content != "hello" || parsed.ReplyTo != "msg42" {
		t.Errorf("body fields mismatch: %+v", parsed)
	}
	if len(parsed.Media) != 1 || parsed.Media[0].URL != "https://x/a.jpg" {
		t.Errorf("media not propagated: %+v", parsed.Media)
	}
}

func TestClient_Send_Non2xxReturnsErr(t *testing.T) {
	mock := &mockSidecarHandler{status: http.StatusInternalServerError, responseBody: `{"error":"boom"}`}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	client := newSidecarClient(srv.URL, "tok")
	_, err := client.Send(context.Background(), sendRequest{ChatID: "c1"})
	if !errors.Is(err, ErrSidecarBadStatus) {
		t.Errorf("expected ErrSidecarBadStatus, got %v", err)
	}
}

func TestClient_Send_UnreachableReturnsErr(t *testing.T) {
	// Use a port nobody's listening on.
	client := newSidecarClient("http://127.0.0.1:1", "tok")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Send(ctx, sendRequest{ChatID: "c1"})
	if err == nil {
		t.Fatal("expected error on unreachable sidecar")
	}
	// Accept either unreachable or timeout depending on OS behavior.
	if !errors.Is(err, ErrSidecarUnreachable) && !errors.Is(err, ErrSidecarTimeout) {
		t.Errorf("expected unreachable/timeout, got %v", err)
	}
}

func TestClient_Health_Success(t *testing.T) {
	mock := &mockSidecarHandler{}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	client := newSidecarClient(srv.URL, "tok")
	if err := client.Health(context.Background()); err != nil {
		t.Errorf("Health: %v", err)
	}
	if mock.healthCalled != 1 {
		t.Errorf("healthCalled=%d want=1", mock.healthCalled)
	}
}

func TestClient_Health_Non200(t *testing.T) {
	mock := &mockSidecarHandler{status: http.StatusServiceUnavailable}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	client := newSidecarClient(srv.URL, "tok")
	err := client.Health(context.Background())
	if !errors.Is(err, ErrSidecarBadStatus) {
		t.Errorf("expected ErrSidecarBadStatus, got %v", err)
	}
}

func TestClient_Send_JSONDecodeError(t *testing.T) {
	mock := &mockSidecarHandler{responseBody: "not json at all"}
	srv := httptest.NewServer(mock)
	defer srv.Close()

	client := newSidecarClient(srv.URL, "tok")
	_, err := client.Send(context.Background(), sendRequest{ChatID: "c1"})
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected decode error, got %v", err)
	}
}
