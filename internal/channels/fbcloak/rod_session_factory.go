//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/pkg/browser"
)

// RodSessionFactory builds per-credential page sessions backed by a real
// rod browser. One incognito context per credential keeps cookies isolated
// across fanpages — critical when one Chrome instance serves multiple
// credentials concurrently.
//
// Lifecycle:
//   - Open() acquires a fresh incognito context, sets cookies + viewport
//     + UA + stealth, opens a blank page, and returns a PageSession.
//   - The closer (returned alongside) MUST be called by the caller (use
//     defer) to release the page + incognito context. We never hold them
//     beyond the job run.
//
// jobID arrives via Open() (per-run scope), so the factory itself stays
// stateless across runs — safe to share across goroutines without locks.
type RodSessionFactory struct {
	BrowserMgr *browser.Manager
	Writer     *ScreenshotWriter
}

// Compile-time assertion.
var _ SessionFactory = (*RodSessionFactory)(nil)

// Open implements SessionFactory. The runner passes its current jobID so
// screenshots persist under {tenant}/{job}/{sendLog}_{kind}_*.png.
func (f *RodSessionFactory) Open(ctx context.Context, jobID uuid.UUID, c Credential) (PageSession, func(), error) {
	if f.BrowserMgr == nil {
		return nil, noopCloser, errors.New("rod_session_factory: nil BrowserMgr")
	}
	if c.ID == uuid.Nil {
		return nil, noopCloser, errors.New("rod_session_factory: credential.ID required")
	}

	incog, err := f.BrowserMgr.NewIncognitoContext(ctx)
	if err != nil {
		return nil, noopCloser, fmt.Errorf("incognito: %w", err)
	}
	cleanupIncog := func() {
		_ = incog.Close()
	}

	// Inject cookies BEFORE the first navigation. Decode happens here
	// rather than in NewService so credentials in storage stay
	// encrypted-at-rest until the runner actually opens a session.
	cookies, err := browser.UnmarshalCookies(c.Cookies)
	if err != nil {
		cleanupIncog()
		return nil, noopCloser, fmt.Errorf("decode cookies: %w", err)
	}
	if err := browser.SetCookies(ctx, incog, cookies); err != nil {
		cleanupIncog()
		return nil, noopCloser, fmt.Errorf("set cookies: %w", err)
	}

	page, err := incog.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		cleanupIncog()
		return nil, noopCloser, fmt.Errorf("create page: %w", err)
	}

	// User-Agent override. Non-fatal on failure — rod's default UA is
	// browser-realistic, and the credential UA is a hardening rather
	// than a hard requirement.
	if c.UserAgent != "" {
		_ = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: c.UserAgent})
	}
	// Viewport — same logic, best-effort.
	if c.ViewportW > 0 && c.ViewportH > 0 {
		_ = page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
			Width:  c.ViewportW,
			Height: c.ViewportH,
			// DeviceScaleFactor 1 is sane; Mobile false matches desktop UA.
			DeviceScaleFactor: 1,
			Mobile:            false,
		})
	}

	if err := browser.ApplyStealth(page); err != nil {
		_ = page.Close()
		cleanupIncog()
		return nil, noopCloser, fmt.Errorf("apply stealth: %w", err)
	}

	sess := NewRodPageSession(page, c.TenantID, jobID, c.ID, f.Writer)
	closer := func() {
		_ = page.Close()
		cleanupIncog()
	}
	return sess, closer, nil
}

func noopCloser() {}
