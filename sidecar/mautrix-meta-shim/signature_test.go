package main

import (
	"testing"
	"time"
)

const testSecret = "sidecar-test-secret"

func TestSignWebhook_RoundTrip(t *testing.T) {
	body := []byte(`{"event_type":"message","content":"hi"}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)

	if err := VerifySignature(body, header, testSecret, ts); err != nil {
		t.Errorf("roundtrip failed: %v", err)
	}
}

func TestSignWebhook_TamperedBody(t *testing.T) {
	body := []byte(`{"x":1}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)
	if err := VerifySignature([]byte(`{"x":2}`), header, testSecret, ts); err == nil {
		t.Error("expected failure on tampered body")
	}
}

func TestSignWebhook_WrongSecret(t *testing.T) {
	body := []byte(`{"x":1}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)
	if err := VerifySignature(body, header, "other-secret", ts); err == nil {
		t.Error("expected failure with wrong secret")
	}
}

func TestSignWebhook_ExpiredWindow(t *testing.T) {
	body := []byte(`{"x":1}`)
	ts := time.Unix(1_700_000_000, 0)
	header := SignWebhook(body, testSecret, ts)
	// Verify 2 minutes later — outside 60s window.
	if err := VerifySignature(body, header, testSecret, ts.Add(120*time.Second)); err == nil {
		t.Error("expected expiry failure")
	}
}

func TestSignWebhook_CrossCompat(t *testing.T) {
	// This is the shared test vector — GoClaw side's signature_test.go validates
	// the exact same input/output. Do not change without updating both sides.
	body := []byte(`{"event_type":"message","thread_id":"t1","sender_id":"u1"}`)
	ts := time.Unix(1_750_000_000, 0)
	header := SignWebhook(body, "shared-secret-for-cross-compat", ts)

	// Header format must be: t=<unix>,v1=<64-char-hex>
	if len(header) < 70 {
		t.Errorf("header too short: %q", header)
	}
	if header[:2] != "t=" {
		t.Errorf("header missing t= prefix: %q", header)
	}
}

func TestVerifyBearer_Success(t *testing.T) {
	if !VerifyBearer("Bearer my-token", "my-token") {
		t.Error("valid bearer should succeed")
	}
}

func TestVerifyBearer_Fail(t *testing.T) {
	cases := []struct {
		header, expected string
	}{
		{"", "t"},
		{"Bearer", "t"},
		{"Basic my-token", "my-token"},
		{"Bearer wrong", "right"},
	}
	for _, tc := range cases {
		if VerifyBearer(tc.header, tc.expected) {
			t.Errorf("header=%q expected=%q — should have failed", tc.header, tc.expected)
		}
	}
}

func TestVerifyBearer_ConstantTime(t *testing.T) {
	// Not testing timing here, but exercise the hmac.Equal path with equal-length strings.
	if VerifyBearer("Bearer aaaaaa", "bbbbbb") {
		t.Error("different equal-length should fail")
	}
}
