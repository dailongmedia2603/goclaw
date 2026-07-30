package facebookmessenger

import (
	"errors"
	"testing"
	"time"
)

const testSecret = "test-hmac-secret-should-be-long-enough"

func fixedTime(unix int64) func() time.Time {
	return func() time.Time { return time.Unix(unix, 0) }
}

func TestVerifySignature_ValidFreshTimestamp(t *testing.T) {
	body := []byte(`{"hello":"world"}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)

	// Clock exactly matches the signing time — should be accepted.
	if err := VerifyWebhookSignature(body, header, testSecret, fixedTime(1_700_000_000)); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestVerifySignature_WithinWindow(t *testing.T) {
	body := []byte(`{"x":1}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)

	// 30s later — still within 60s window.
	if err := VerifyWebhookSignature(body, header, testSecret, fixedTime(1_700_000_030)); err != nil {
		t.Errorf("expected valid within window, got %v", err)
	}
}

func TestVerifySignature_ExpiredTimestamp(t *testing.T) {
	body := []byte(`{"x":1}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)

	// 120s later — outside 60s window.
	err := VerifyWebhookSignature(body, header, testSecret, fixedTime(1_700_000_120))
	if !errors.Is(err, ErrSignatureExpired) {
		t.Errorf("expected ErrSignatureExpired, got %v", err)
	}
}

func TestVerifySignature_FutureTimestampBeyondWindow(t *testing.T) {
	body := []byte(`{"x":1}`)
	// Sign with future time, verify now (now is well earlier).
	ts := time.Unix(1_700_000_200, 0)
	header := SignWebhook(body, testSecret, ts)

	err := VerifyWebhookSignature(body, header, testSecret, fixedTime(1_700_000_000))
	if !errors.Is(err, ErrSignatureExpired) {
		t.Errorf("expected ErrSignatureExpired (future), got %v", err)
	}
}

func TestVerifySignature_TamperedBody(t *testing.T) {
	body := []byte(`{"x":1}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)

	tampered := []byte(`{"x":2}`)
	err := VerifyWebhookSignature(tampered, header, testSecret, fixedTime(1_700_000_000))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifySignature_WrongSecret(t *testing.T) {
	body := []byte(`{"x":1}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)

	err := VerifyWebhookSignature(body, header, "wrong-secret", fixedTime(1_700_000_000))
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Errorf("expected ErrSignatureInvalid, got %v", err)
	}
}

func TestVerifySignature_MissingHeader(t *testing.T) {
	err := VerifyWebhookSignature([]byte(`{}`), "", testSecret, nil)
	if !errors.Is(err, ErrSignatureMissing) {
		t.Errorf("expected ErrSignatureMissing, got %v", err)
	}
}

func TestVerifySignature_MissingSecret(t *testing.T) {
	err := VerifyWebhookSignature([]byte(`{}`), "t=1,v1=abc", "", nil)
	if !errors.Is(err, ErrSignatureMissing) {
		t.Errorf("expected ErrSignatureMissing (secret), got %v", err)
	}
}

func TestVerifySignature_MalformedHeader(t *testing.T) {
	cases := []string{
		"garbage",
		"t=",
		"v1=abc",       // no timestamp
		"t=abc,v1=def", // non-numeric ts
		"t=123",        // no sig
	}
	for _, h := range cases {
		err := VerifyWebhookSignature([]byte(`{}`), h, testSecret, fixedTime(123))
		if !errors.Is(err, ErrSignatureMalformed) {
			t.Errorf("header %q: expected ErrSignatureMalformed, got %v", h, err)
		}
	}
}

func TestSignWebhook_RoundTrip(t *testing.T) {
	// Prove SignWebhook + VerifyWebhookSignature agree — this is the shared
	// test vector that the sidecar shim must match.
	body := []byte(`{"event_type":"message","content":"hello"}`)
	ts := time.Unix(1_700_012_345, 0)
	header := SignWebhook(body, testSecret, ts)
	if err := VerifyWebhookSignature(body, header, testSecret, fixedTime(1_700_012_345)); err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
}
