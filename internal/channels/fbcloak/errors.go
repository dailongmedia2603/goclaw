//go:build !sqliteonly

package fbcloak

import "errors"

var (
	// ErrFeatureDisabled is returned when fbcloak is invoked on an edition
	// where Edition.FBCloakEnabled == false (e.g. Lite desktop).
	ErrFeatureDisabled = errors.New("fbcloak: feature not available in this edition")

	// ErrKillswitchActive is returned when GOCLAW_FBCLOAK_KILLSWITCH=1 or the
	// service has been programmatically killswitched.
	ErrKillswitchActive = errors.New("fbcloak: killswitch active — feature disabled")

	// ErrCookieExpired indicates the stored credential's cookies no longer
	// authenticate; user must re-upload.
	ErrCookieExpired = errors.New("fbcloak: cookie expired or invalid")

	// ErrCheckpoint indicates Meta has placed the account on a security
	// checkpoint mid-flow; the credential is locked until manual intervention.
	ErrCheckpoint = errors.New("fbcloak: account checkpoint detected")

	// ErrCredentialNotFound is the typed not-found for credentials.
	ErrCredentialNotFound = errors.New("fbcloak: credential not found")

	// ErrJobNotFound is the typed not-found for jobs.
	ErrJobNotFound = errors.New("fbcloak: job not found")

	// ErrSendLogNotFound is the typed not-found for fbcloak_send_log rows.
	ErrSendLogNotFound = errors.New("fbcloak: send log not found")

	// ErrDisclaimerRequired is returned when an action requires the
	// current disclaimer to be acked but the tenant has not yet done so.
	ErrDisclaimerRequired = errors.New("fbcloak: disclaimer acknowledgement required")

	// ErrNoConversationHistory means the dual-mode router could not find
	// a `last_inbound_at` for the (tenantID, fanpageID, recipientPSID)
	// tuple — fail-closed so the caller never silently picks a window.
	ErrNoConversationHistory = errors.New("fbcloak: no conversation history for recipient")

	// ErrOutOfWindow means the recipient's last inbound is beyond the
	// supported window (>6 months). Sponsored Messages would handle this
	// but is intentionally out of scope.
	ErrOutOfWindow = errors.New("fbcloak: recipient outside supported re-engagement window")

	// ErrGraphSenderUnconfigured is returned by the router when the
	// chosen path is Graph API (≤7d) but no GraphSender is wired. Caller
	// must surface this as a clear "feature unavailable" error rather
	// than fall through to fbcloak (which has different consent rules).
	ErrGraphSenderUnconfigured = errors.New("fbcloak: graph API path not configured")

	// ErrTenantMismatch is returned when a cross-tenant access is attempted.
	ErrTenantMismatch = errors.New("fbcloak: tenant mismatch")

	// ErrInvalidProxyURL is returned for malformed proxy URLs.
	ErrInvalidProxyURL = errors.New("fbcloak: invalid proxy URL")
)
