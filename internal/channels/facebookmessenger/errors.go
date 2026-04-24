package facebookmessenger

import "errors"

var (
	ErrNotImplemented    = errors.New("facebook_personal: not implemented")
	ErrNotStarted        = errors.New("facebook_personal: channel not started")
	ErrMissingSidecarURL = errors.New("facebook_personal: sidecar_url is required")
	ErrMissingAuthToken  = errors.New("facebook_personal: auth_token is required")
	ErrMissingSecret     = errors.New("facebook_personal: webhook_secret is required")
	ErrInvalidCreds      = errors.New("facebook_personal: invalid credentials JSON")
	ErrInvalidConfig     = errors.New("facebook_personal: invalid config JSON")

	// Webhook / signature errors.
	ErrSignatureMissing   = errors.New("facebook_personal: webhook signature missing")
	ErrSignatureMalformed = errors.New("facebook_personal: webhook signature malformed")
	ErrSignatureExpired   = errors.New("facebook_personal: webhook signature expired (timestamp outside 60s window)")
	ErrSignatureInvalid   = errors.New("facebook_personal: webhook signature invalid (HMAC mismatch)")
	ErrBodyTooLarge       = errors.New("facebook_personal: webhook body exceeds 4 MiB limit")

	// Event mapping errors.
	ErrNotAMessage   = errors.New("facebook_personal: event is not a message type")
	ErrMissingThread = errors.New("facebook_personal: event missing thread_id")
	ErrMissingSender = errors.New("facebook_personal: event missing sender_id")

	// Sidecar client errors.
	ErrSidecarUnreachable = errors.New("facebook_personal: sidecar unreachable")
	ErrSidecarBadStatus   = errors.New("facebook_personal: sidecar returned non-2xx status")
	ErrSidecarTimeout     = errors.New("facebook_personal: sidecar request timed out")
)
