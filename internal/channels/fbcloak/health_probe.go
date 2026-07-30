//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-rod/rod"

	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// HealthProbe runs a lightweight authentication check on a credential by
// loading https://www.facebook.com/me with the stored cookies + proxy and
// inspecting the redirect outcome. Phase 2 will reuse this probe before each
// job run; Phase 1 only exposes it via RPC fbcloak.test_credential.
type HealthProbe struct {
	NewBrowser func(ctx context.Context, c Credential) (*rod.Browser, func(), error)
	Now        func() time.Time
}

// Run performs the probe and returns a typed verdict. The browser obtained
// from NewBrowser is closed via the returned closer regardless of outcome.
func (h *HealthProbe) Run(ctx context.Context, c Credential) (ProbeResult, error) {
	if h.NewBrowser == nil {
		return ProbeResult{}, errors.New("HealthProbe.NewBrowser not configured")
	}
	now := time.Now
	if h.Now != nil {
		now = h.Now
	}

	b, closer, err := h.NewBrowser(ctx, c)
	if err != nil {
		return ProbeResult{}, err
	}
	defer closer()

	cookies, err := browser.UnmarshalCookies(c.Cookies)
	if err != nil {
		return ProbeResult{Status: StatusExpired, Detail: "cookies unparsable"}, nil
	}
	if err := browser.SetCookies(ctx, b, cookies); err != nil {
		return ProbeResult{}, err
	}

	page, err := b.Page(rodTargetBlank())
	if err != nil {
		return ProbeResult{}, err
	}
	defer page.Close()
	if err := browser.ApplyStealth(page); err != nil {
		return ProbeResult{}, err
	}

	if err := page.Navigate("https://www.facebook.com/me"); err != nil {
		return ProbeResult{Status: StatusExpired, Detail: "navigate failed: " + err.Error()}, nil
	}
	_ = page.WaitLoad()

	info, err := page.Info()
	if err != nil {
		return ProbeResult{Status: StatusExpired, Detail: "info failed"}, nil
	}

	verdict := classifyMeURL(info.URL)
	verdict.OK = verdict.Status == StatusActive
	if h.Now != nil {
		_ = now() // testable hook reserved for future timestamping
	}
	return verdict, nil
}

// classifyMeURL is the pure-function core of the probe — extracted for unit
// testing without spinning up a real browser.
func classifyMeURL(finalURL string) ProbeResult {
	switch {
	case strings.Contains(finalURL, "/login"):
		return ProbeResult{Status: StatusExpired, Detail: "redirected to /login"}
	case strings.Contains(finalURL, "/checkpoint"):
		return ProbeResult{Status: StatusCheckpoint, Detail: "redirected to /checkpoint"}
	case finalURL == "":
		return ProbeResult{Status: StatusExpired, Detail: "blank URL"}
	default:
		return ProbeResult{Status: StatusActive, Detail: finalURL}
	}
}
