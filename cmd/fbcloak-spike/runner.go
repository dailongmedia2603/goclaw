//go:build spike

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

// runnerConfig holds the spike CLI inputs.
type runnerConfig struct {
	Cookies        []SpikeCookie
	FanpageID      string
	ConversationID string
	PageUsername   string
	ProxyURL       string
	UserAgent      string
	Headless       bool
	IdleMinutes    int
	SkipIdle       bool
	AssetsDir      string
}

// SpikeCookie matches the JSON exported by the EditThisCookie /
// Cookie-Editor browser extensions, with the fields rod needs.
type SpikeCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expirationDate,omitempty"`
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	SameSite string  `json:"sameSite,omitempty"`
}

// runner owns the rod browser for one spike run.
type runner struct {
	cfg      runnerConfig
	launcher *launcher.Launcher
	browser  *rod.Browser
}

func newRunner(ctx context.Context, cfg runnerConfig) (*runner, error) {
	l := launcher.New().
		Headless(cfg.Headless).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-features", "IsolateOrigins,site-per-process").
		Set("no-first-run").
		Set("no-default-browser-check").
		Set("disable-extensions").
		Set("disable-background-networking").
		Set("disable-gpu").
		Leakless(true)

	if cfg.ProxyURL != "" {
		// Validate URL early; rod accepts socks5://host:port and http://host:port.
		if _, err := url.Parse(cfg.ProxyURL); err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		l.Set("proxy-server", cfg.ProxyURL)
	}

	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch chrome: %w", err)
	}

	b := rod.New().Context(ctx).ControlURL(controlURL)
	if err := b.Connect(); err != nil {
		l.Kill()
		return nil, fmt.Errorf("connect chrome: %w", err)
	}

	r := &runner{cfg: cfg, launcher: l, browser: b}
	if err := r.applyStealth(ctx); err != nil {
		r.Close()
		return nil, fmt.Errorf("apply stealth: %w", err)
	}
	if err := r.injectCookies(ctx); err != nil {
		r.Close()
		return nil, fmt.Errorf("inject cookies: %w", err)
	}
	return r, nil
}

func (r *runner) Close() {
	if r.browser != nil {
		_ = r.browser.Close()
	}
	if r.launcher != nil {
		r.launcher.Kill()
		r.launcher.Cleanup()
	}
}

// stealthJS bundles the minimal JS patches that handle 90 % of trivial bot
// detectors. Phase 1 will replace this with the github.com/go-rod/stealth
// package; for the spike we keep zero new deps.
const stealthJS = `
(() => {
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  // Languages — match VN locale + en
  Object.defineProperty(navigator, 'languages', { get: () => ['vi-VN', 'vi', 'en-US', 'en'] });
  // Plugins — non-empty so detectors that count !== 0 pass
  Object.defineProperty(navigator, 'plugins', {
    get: () => [1, 2, 3, 4, 5].map(i => ({ name: 'Plugin ' + i, filename: 'p' + i + '.so', description: '' })),
  });
  // permissions.query — return 'prompt' for notifications instead of 'denied' which is a common HL signal
  const origQuery = window.navigator.permissions && window.navigator.permissions.query;
  if (origQuery) {
    window.navigator.permissions.query = (params) =>
      params && params.name === 'notifications'
        ? Promise.resolve({ state: Notification.permission })
        : origQuery(params);
  }
  // chrome.runtime — present
  if (!window.chrome) { window.chrome = {}; }
  if (!window.chrome.runtime) { window.chrome.runtime = {}; }
  // WebGL renderer / vendor spoof to a common GPU
  const getParameter = WebGLRenderingContext.prototype.getParameter;
  WebGLRenderingContext.prototype.getParameter = function (parameter) {
    if (parameter === 37445) return 'Intel Inc.';
    if (parameter === 37446) return 'Intel Iris OpenGL Engine';
    return getParameter.call(this, parameter);
  };
})();
`

func (r *runner) applyStealth(_ context.Context) error {
	// EvalOnNewDocument is applied per page in newPage() so each page gets a
	// fresh injection. Browser-wide attach is implicit via the launcher flags.
	return nil
}

// newPage opens a new page, applies stealth on document load, and sets the UA.
func (r *runner) newPage(ctx context.Context) (*rod.Page, error) {
	page, err := r.browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, err
	}
	if r.cfg.UserAgent != "" {
		_ = page.SetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: r.cfg.UserAgent})
	}
	_, err = page.EvalOnNewDocument(stealthJS)
	if err != nil {
		return nil, fmt.Errorf("eval stealth: %w", err)
	}
	return page, nil
}

// injectCookies maps the JSON cookie schema to rod's CDP type and SetCookies-es them.
func (r *runner) injectCookies(_ context.Context) error {
	cdpCookies := make([]*proto.NetworkCookieParam, 0, len(r.cfg.Cookies))
	for _, c := range r.cfg.Cookies {
		domain := c.Domain
		if !strings.HasPrefix(domain, ".") && domain != "" && domain[0] != '.' {
			// Cookie-Editor sometimes drops the leading dot
			if !strings.Contains(domain, ".") {
				continue
			}
		}
		var sameSite proto.NetworkCookieSameSite
		switch strings.ToLower(c.SameSite) {
		case "lax", "no_restriction", "":
			sameSite = proto.NetworkCookieSameSiteLax
		case "strict":
			sameSite = proto.NetworkCookieSameSiteStrict
		case "none", "unspecified":
			sameSite = proto.NetworkCookieSameSiteNone
		}
		cookie := &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: sameSite,
		}
		if c.Expires > 0 {
			t := proto.TimeSinceEpoch(c.Expires)
			cookie.Expires = t
		}
		cdpCookies = append(cdpCookies, cookie)
	}
	if err := r.browser.SetCookies(cdpCookies); err != nil {
		return fmt.Errorf("set cookies: %w", err)
	}
	return nil
}

// loadCookies parses a JSON file in EditThisCookie/Cookie-Editor format.
func loadCookies(path string) ([]SpikeCookie, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cookies []SpikeCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return nil, fmt.Errorf("parse cookies (expected JSON array): %w", err)
	}
	requiredCount := 0
	for _, c := range cookies {
		switch c.Name {
		case "c_user", "xs", "datr":
			requiredCount++
		}
	}
	if requiredCount < 3 {
		return cookies, fmt.Errorf("cookies missing required FB names (need c_user, xs, datr — got %d/3)", requiredCount)
	}
	return cookies, nil
}

// SpikeReport is the JSON dumped at the end of a run.
type SpikeReport struct {
	StartedAt       string                  `json:"startedAt"`
	FinishedAt      string                  `json:"finishedAt"`
	Decision        string                  `json:"decision"` // GREEN | YELLOW | RED
	Config          ReportConfig            `json:"config"`
	CookieInject    *CookieInjectResult     `json:"cookieInject,omitempty"`
	Sannysoft       *ProbeCapture           `json:"sannysoft,omitempty"`
	Creepjs         *ProbeCapture           `json:"creepjs,omitempty"`
	DirectThreadURL *DirectThreadURLResult  `json:"directThreadURL,omitempty"`
	InboxScanner    *InboxScannerResult     `json:"inboxScanner,omitempty"`
	IdleStress      *IdleStressResult       `json:"idleStress,omitempty"`
}

type ReportConfig struct {
	FanpageID       string `json:"fanpageId"`
	ConversationID  string `json:"conversationId,omitempty"`
	ProxyConfigured bool   `json:"proxyConfigured"`
	Headless        bool   `json:"headless"`
	IdleMinutes     int    `json:"idleMinutes"`
}

type CookieInjectResult struct {
	LoggedIn      bool   `json:"loggedIn"`
	FinalURL      string `json:"finalUrl"`
	Title         string `json:"title"`
	UserIDFromDOM string `json:"userIdFromDom,omitempty"`
	ScreenshotPNG string `json:"screenshot,omitempty"`
	Err           string `json:"err,omitempty"`
}

type ProbeCapture struct {
	Captured      bool   `json:"captured"`
	URL           string `json:"url"`
	ScreenshotPNG string `json:"screenshot,omitempty"`
	Err           string `json:"err,omitempty"`
}

type DirectThreadURLResult struct {
	BusinessSuiteURL string `json:"businessSuiteUrl"`
	BusinessSuiteOK  bool   `json:"businessSuiteOk"`
	BusinessSuiteErr string `json:"businessSuiteErr,omitempty"`
	BusinessShot     string `json:"businessShot,omitempty"`

	MessengerWebURL string `json:"messengerWebUrl"`
	MessengerWebOK  bool   `json:"messengerWebOk"`
	MessengerWebErr string `json:"messengerWebErr,omitempty"`
	MessengerShot   string `json:"messengerShot,omitempty"`

	PagesClassicURL string `json:"pagesClassicUrl,omitempty"`
	PagesClassicOK  bool   `json:"pagesClassicOk,omitempty"`
	PagesClassicErr string `json:"pagesClassicErr,omitempty"`
	PagesShot       string `json:"pagesShot,omitempty"`

	WinningPattern string `json:"winningPattern,omitempty"`
}

type InboxScannerResult struct {
	ListitemCount int    `json:"listitemCount"`
	Tier1AXName   string `json:"tier1AxName,omitempty"`
	Tier2HTMLLen  int    `json:"tier2HtmlLen,omitempty"`
	Tier3RegexHit string `json:"tier3RegexHit,omitempty"`
	ScreenshotPNG string `json:"screenshot,omitempty"`
	Err           string `json:"err,omitempty"`
}

type IdleStressResult struct {
	DurationMin     int    `json:"durationMin"`
	CheckpointHits  int    `json:"checkpointHits"`
	CaptchaHits     int    `json:"captchaHits"`
	FinalURL        string `json:"finalUrl"`
	ScreenshotPNG   string `json:"screenshot,omitempty"`
	Err             string `json:"err,omitempty"`
}

// --- S0.2 Cookie inject health probe ---

func (r *runner) RunCookieInject(ctx context.Context) *CookieInjectResult {
	res := &CookieInjectResult{}
	page, err := r.newPage(ctx)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer page.Close()

	if err := page.Navigate("https://www.facebook.com/me"); err != nil {
		res.Err = "navigate: " + err.Error()
		return res
	}
	if err := page.WaitLoad(); err != nil {
		res.Err = "wait load: " + err.Error()
	}
	time.Sleep(3 * time.Second)

	info, _ := page.Info()
	res.FinalURL = info.URL
	res.Title = info.Title

	// Heuristic: a logged-in /me redirects to /<vanity> or /profile.php?id=...
	// Login page contains "/login" path; checkpoint contains "/checkpoint".
	switch {
	case strings.Contains(info.URL, "/login"):
		res.LoggedIn = false
	case strings.Contains(info.URL, "/checkpoint"):
		res.LoggedIn = false
	default:
		res.LoggedIn = true
	}

	res.ScreenshotPNG = r.savePNG(page, "01_cookie_health")
	return res
}

// --- S0.3 Stealth probes ---

func (r *runner) RunSannysoft(ctx context.Context) *ProbeCapture {
	return r.runSimpleProbe(ctx, "https://bot.sannysoft.com/", "02_sannysoft")
}

func (r *runner) RunCreepjs(ctx context.Context) *ProbeCapture {
	return r.runSimpleProbe(ctx, "https://abrahamjuliot.github.io/creepjs/", "03_creepjs")
}

func (r *runner) runSimpleProbe(ctx context.Context, target, slug string) *ProbeCapture {
	res := &ProbeCapture{URL: target}
	page, err := r.newPage(ctx)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer page.Close()
	if err := page.Navigate(target); err != nil {
		res.Err = err.Error()
		return res
	}
	_ = page.WaitLoad()
	time.Sleep(8 * time.Second) // give creepjs time to compute fingerprint
	res.ScreenshotPNG = r.savePNG(page, slug)
	res.Captured = res.ScreenshotPNG != ""
	return res
}

// --- S0.4 Direct thread URL ---

func (r *runner) RunDirectThreadURL(ctx context.Context) *DirectThreadURLResult {
	res := &DirectThreadURLResult{}
	cv := r.cfg.ConversationID
	pg := r.cfg.FanpageID

	// Pattern 1 — Business Suite latest inbox with active_chat_thread_id
	res.BusinessSuiteURL = fmt.Sprintf("https://business.facebook.com/latest/inbox?asset_id=%s&active_chat_thread_id=%s", pg, cv)
	res.BusinessSuiteOK, res.BusinessSuiteErr, res.BusinessShot = r.probeThreadURL(ctx, res.BusinessSuiteURL, "04_business_suite")

	// Pattern 2 — Messenger web
	res.MessengerWebURL = fmt.Sprintf("https://www.facebook.com/messages/t/%s", strings.TrimPrefix(cv, "t_"))
	res.MessengerWebOK, res.MessengerWebErr, res.MessengerShot = r.probeThreadURL(ctx, res.MessengerWebURL, "05_messenger_web")

	// Pattern 3 — Pages classic (only if username supplied)
	if r.cfg.PageUsername != "" {
		res.PagesClassicURL = fmt.Sprintf("https://business.facebook.com/%s/inbox/?selected_thread_id=%s", r.cfg.PageUsername, cv)
		res.PagesClassicOK, res.PagesClassicErr, res.PagesShot = r.probeThreadURL(ctx, res.PagesClassicURL, "06_pages_classic")
	}

	switch {
	case res.BusinessSuiteOK:
		res.WinningPattern = "business_suite"
	case res.MessengerWebOK:
		res.WinningPattern = "messenger_web"
	case res.PagesClassicOK:
		res.WinningPattern = "pages_classic"
	}
	return res
}

// probeThreadURL navigates and reports whether a textbox + send button are
// visible (i.e. we have a writable thread). The selector list is intentionally
// loose — we want to distinguish "thread loaded" from "redirected to /login".
func (r *runner) probeThreadURL(ctx context.Context, target, slug string) (ok bool, errStr, shot string) {
	page, err := r.newPage(ctx)
	if err != nil {
		return false, err.Error(), ""
	}
	defer page.Close()

	if err := page.Navigate(target); err != nil {
		return false, "navigate: " + err.Error(), ""
	}
	_ = page.WaitLoad()
	time.Sleep(6 * time.Second)

	info, _ := page.Info()
	finalURL := info.URL
	if strings.Contains(finalURL, "/login") || strings.Contains(finalURL, "/checkpoint") {
		shot = r.savePNG(page, slug+"_redirect")
		return false, "redirected to " + finalURL, shot
	}

	// Selector probe: look for a textbox-shaped element. We try several known
	// roles/aria patterns; the first hit wins.
	textboxFound := false
	candidates := []string{
		`[role="textbox"][contenteditable="true"]`,
		`div[contenteditable="true"][aria-label]`,
		`[data-lexical-editor="true"]`,
		`textarea[name="message_body"]`,
	}
	for _, sel := range candidates {
		if el, err := page.Timeout(3 * time.Second).Element(sel); err == nil && el != nil {
			textboxFound = true
			break
		}
	}

	shot = r.savePNG(page, slug)
	if !textboxFound {
		return false, "no textbox-like element found at " + finalURL, shot
	}
	return true, "", shot
}

// --- S0.4b Inbox scanner fallback (called only when direct URL fails or no conv) ---

func (r *runner) RunInboxScannerProbe(ctx context.Context) *InboxScannerResult {
	res := &InboxScannerResult{}
	page, err := r.newPage(ctx)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer page.Close()

	target := fmt.Sprintf("https://business.facebook.com/latest/inbox?asset_id=%s", r.cfg.FanpageID)
	if err := page.Navigate(target); err != nil {
		res.Err = "navigate: " + err.Error()
		return res
	}
	_ = page.WaitLoad()
	time.Sleep(8 * time.Second)

	// Tier 1 — count listitems via DOM
	listitems, err := page.Elements(`[role="listitem"]`)
	if err != nil {
		res.Err = "query listitems: " + err.Error()
		return res
	}
	res.ListitemCount = len(listitems)

	if len(listitems) > 0 {
		// First listitem aria-label often contains "Name • 3 ngày"
		if name, err := listitems[0].Attribute("aria-label"); err == nil && name != nil {
			res.Tier1AXName = *name
		}
		// Tier 2 — outerHTML length (we don't dump full HTML to keep report small)
		if html, err := listitems[0].HTML(); err == nil {
			res.Tier2HTMLLen = len(html)
		}
		// Tier 3 — regex on innerText for "X ngày" / "X tháng" / "tuần"
		if txt, err := listitems[0].Text(); err == nil {
			for _, kw := range []string{"ngày", "tuần", "tháng", "giờ", "phút", "giây"} {
				if strings.Contains(txt, kw) {
					res.Tier3RegexHit = kw
					break
				}
			}
		}
	}

	res.ScreenshotPNG = r.savePNG(page, "07_inbox_scanner")
	return res
}

// --- S0.5 Idle stress ---

func (r *runner) RunIdleStress(ctx context.Context) *IdleStressResult {
	res := &IdleStressResult{DurationMin: r.cfg.IdleMinutes}
	page, err := r.newPage(ctx)
	if err != nil {
		res.Err = err.Error()
		return res
	}
	defer page.Close()

	target := fmt.Sprintf("https://business.facebook.com/latest/inbox?asset_id=%s", r.cfg.FanpageID)
	if err := page.Navigate(target); err != nil {
		res.Err = "navigate: " + err.Error()
		return res
	}
	_ = page.WaitLoad()

	deadline := time.Now().Add(time.Duration(r.cfg.IdleMinutes) * time.Minute)
	tick := time.NewTicker(90 * time.Second)
	defer tick.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			res.Err = "ctx cancelled"
			return res
		case <-tick.C:
			info, _ := page.Info()
			finalURL := info.URL
			if strings.Contains(finalURL, "/checkpoint") {
				res.CheckpointHits++
			}
			if hasCaptcha(page) {
				res.CaptchaHits++
			}
			// Soft scroll to keep session "live"
			_, _ = page.Eval(`() => window.scrollBy(0, 200 + Math.floor(Math.random()*400))`)
		}
	}

	info, _ := page.Info()
	res.FinalURL = info.URL
	res.ScreenshotPNG = r.savePNG(page, "08_idle_final")
	return res
}

func hasCaptcha(page *rod.Page) bool {
	for _, sel := range []string{`iframe[src*="recaptcha"]`, `[data-testid="captcha"]`, `[id*="captcha"]`} {
		if el, _ := page.Timeout(time.Second).Element(sel); el != nil {
			return true
		}
	}
	return false
}

// --- helpers ---

func (r *runner) savePNG(page *rod.Page, slug string) string {
	path := filepath.Join(r.cfg.AssetsDir, fmt.Sprintf("%s_%s.png", time.Now().UTC().Format("20060102T150405"), slug))
	data, err := page.Screenshot(false, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if err != nil {
		return ""
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return ""
	}
	return path
}
