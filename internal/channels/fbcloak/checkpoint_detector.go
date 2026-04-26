//go:build !sqliteonly

package fbcloak

import (
	"context"
	"strings"
)

// CheckpointKind enumerates the failure modes the detector recognises.
// Empty (CheckpointNone) means "no checkpoint observed".
type CheckpointKind string

const (
	CheckpointNone      CheckpointKind = ""
	CheckpointSecurity  CheckpointKind = "security"  // /checkpoint/...
	CheckpointCaptcha   CheckpointKind = "captcha"   // recaptcha iframe / data-testid
	CheckpointLogin     CheckpointKind = "login"     // redirected to /login mid-session
	CheckpointSuspended CheckpointKind = "suspended" // account suspended notice
)

// PageInspector is the read-only surface of a Rod page the detector needs.
// Kept intentionally small so a fake implementation can drive the tests
// without spinning up a browser. Mirrors the production *rod.Page methods
// `MustInfo().URL`, `MustInfo().Title`, and `MustHTML()`.
type PageInspector interface {
	URL(ctx context.Context) (string, error)
	Title(ctx context.Context) (string, error)
	HTML(ctx context.Context) (string, error)
}

// DetectCheckpoint inspects the current page and classifies any visible
// checkpoint. Heuristics:
//   - URL contains "/checkpoint" → security
//   - URL contains "/login" → login (mid-session redirect)
//   - URL contains "/two_step_verification" → security
//   - HTML contains a recaptcha iframe or data-testid="captcha" → captcha
//   - Page title (lowercased) contains "your account is suspended" → suspended
//
// Order of checks matters: URL signals are cheaper than HTML scans, so we
// fail fast. Any inspector error returns CheckpointNone with the wrapped
// error — caller decides whether to treat that as a soft skip or a hard
// abort.
func DetectCheckpoint(ctx context.Context, p PageInspector) (CheckpointKind, error) {
	if p == nil {
		return CheckpointNone, nil
	}
	u, err := p.URL(ctx)
	if err != nil {
		return CheckpointNone, err
	}
	if k := classifyURL(u); k != CheckpointNone {
		return k, nil
	}
	title, err := p.Title(ctx)
	if err == nil { // title errors are non-fatal
		if k := classifyTitle(title); k != CheckpointNone {
			return k, nil
		}
	}
	html, err := p.HTML(ctx)
	if err != nil {
		// HTML access failure is more concerning than title — surface it.
		return CheckpointNone, err
	}
	if k := classifyHTML(html); k != CheckpointNone {
		return k, nil
	}
	return CheckpointNone, nil
}

// classifyURL is exported (lowercase by convention but used in tests via
// package-internal access) so each rule can be tuned independently.
func classifyURL(rawURL string) CheckpointKind {
	u := strings.ToLower(rawURL)
	switch {
	case strings.Contains(u, "/checkpoint"):
		return CheckpointSecurity
	case strings.Contains(u, "/two_step_verification"):
		return CheckpointSecurity
	case strings.Contains(u, "/login"):
		return CheckpointLogin
	}
	return CheckpointNone
}

func classifyTitle(title string) CheckpointKind {
	t := strings.ToLower(title)
	if strings.Contains(t, "your account is suspended") || strings.Contains(t, "tài khoản của bạn đã bị tạm khóa") {
		return CheckpointSuspended
	}
	return CheckpointNone
}

func classifyHTML(html string) CheckpointKind {
	h := strings.ToLower(html)
	switch {
	case strings.Contains(h, `iframe src="https://www.google.com/recaptcha`),
		strings.Contains(h, "iframe src=\"https://www.google.com/recaptcha"),
		strings.Contains(h, `data-testid="captcha"`),
		strings.Contains(h, "data-testid='captcha'"):
		return CheckpointCaptcha
	case strings.Contains(h, "your account is suspended"),
		strings.Contains(h, "tài khoản của bạn đã bị tạm khóa"):
		return CheckpointSuspended
	}
	return CheckpointNone
}

// MapToCredentialStatus translates a checkpoint kind into the persisted
// CredentialStatus the credential row should flip to. Login and security
// checkpoints both flag the credential as `checkpoint` so an operator
// reviews before the next run; suspended is terminal.
func MapToCredentialStatus(k CheckpointKind) CredentialStatus {
	switch k {
	case CheckpointSecurity, CheckpointCaptcha, CheckpointLogin:
		return StatusCheckpoint
	case CheckpointSuspended:
		return StatusDisabled
	default:
		return ""
	}
}
