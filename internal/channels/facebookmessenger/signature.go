package facebookmessenger

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// signatureMaxAge is the accepted clock skew for webhook timestamps.
// Anything older/newer than this window is rejected as replay.
const signatureMaxAge = 60 * time.Second

// Header format (Stripe-style):
//
//	X-Fbm-Signature: t=<unix_seconds>,v1=<hex_hmac>
//
// The signed payload is: "<timestamp>.<raw_body>".
// HMAC-SHA256 with the shared webhook secret.

// VerifyWebhookSignature validates the signature header against the body + secret.
// Returns nil on success, a sentinel error on failure. Uses hmac.Equal (constant-time).
//
// nowFn is optional — pass nil for time.Now. Accepts a function so tests can
// inject a fixed clock.
func VerifyWebhookSignature(body []byte, header, secret string, nowFn func() time.Time) error {
	if header == "" {
		return ErrSignatureMissing
	}
	if secret == "" {
		return ErrSignatureMissing
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
		return ErrSignatureMalformed
	}

	tsInt, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return ErrSignatureMalformed
	}

	now := time.Now
	if nowFn != nil {
		now = nowFn
	}
	signed := now().Sub(time.Unix(tsInt, 0))
	if signed < 0 {
		signed = -signed
	}
	if signed > signatureMaxAge {
		return ErrSignatureExpired
	}

	expected := computeSignatureHex(tsInt, body, secret)
	// hmac.Equal is constant-time to mitigate timing attacks.
	if !hmac.Equal([]byte(expected), []byte(sigHex)) {
		return ErrSignatureInvalid
	}
	return nil
}

// SignWebhook produces the X-Fbm-Signature header value for a body + timestamp.
// Exported for sidecar/ integration testing. Sidecar shim uses the same algorithm.
func SignWebhook(body []byte, secret string, ts time.Time) string {
	unix := ts.Unix()
	return fmt.Sprintf("t=%d,v1=%s", unix, computeSignatureHex(unix, body, secret))
}

func computeSignatureHex(ts int64, body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
