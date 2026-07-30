package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// signatureMaxAge is the accepted clock skew for incoming webhook verification.
// Keep in sync with internal/channels/facebookmessenger/signature.go in the main repo.
const signatureMaxAge = 60 * time.Second

// SignWebhook produces the X-Fbm-Signature header value for a body + timestamp.
// The algorithm MUST remain byte-for-byte identical to the GoClaw side:
//
//	mac = HMAC-SHA256(secret, "<ts>." + body)
//	header = "t=<ts>,v1=" + hex(mac)
func SignWebhook(body []byte, secret string, ts time.Time) string {
	unix := ts.Unix()
	return fmt.Sprintf("t=%d,v1=%s", unix, computeSignatureHex(unix, body, secret))
}

// VerifyInbound checks the Authorization header presented by GoClaw for its
// inbound calls to the shim. We use Bearer-token auth (not HMAC) for simplicity —
// HMAC is reserved for the outbound webhook direction.
func VerifyBearer(header, expected string) bool {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	got := header[len(prefix):]
	// Constant-time compare.
	return hmac.Equal([]byte(got), []byte(expected))
}

// VerifySignature (used by tests to confirm roundtrip compat with GoClaw side).
func VerifySignature(body []byte, header, secret string, now time.Time) error {
	if header == "" || secret == "" {
		return fmt.Errorf("missing signature or secret")
	}
	var tsStr, sigHex string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.TrimSpace(kv[0]) {
		case "t":
			tsStr = strings.TrimSpace(kv[1])
		case "v1":
			sigHex = strings.TrimSpace(kv[1])
		}
	}
	if tsStr == "" || sigHex == "" {
		return fmt.Errorf("malformed signature header")
	}
	tsInt, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return fmt.Errorf("malformed timestamp")
	}
	diff := now.Sub(time.Unix(tsInt, 0))
	if diff < 0 {
		diff = -diff
	}
	if diff > signatureMaxAge {
		return fmt.Errorf("expired")
	}
	expected := computeSignatureHex(tsInt, body, secret)
	if !hmac.Equal([]byte(expected), []byte(sigHex)) {
		return fmt.Errorf("invalid")
	}
	return nil
}

func computeSignatureHex(ts int64, body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
