package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Cookie is the portable, JSON-serializable cookie model used across GoClaw.
// It maps cleanly onto Chrome DevTools Protocol's Network.CookieParam and onto
// the JSON exported by Cookie-Editor / EditThisCookie browser extensions.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path,omitempty"`
	Expires  float64 `json:"expirationDate,omitempty"` // unix seconds; 0 = session
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	SameSite string  `json:"sameSite,omitempty"` // "Lax" | "Strict" | "None"
}

// ErrMissingRequiredCookies is returned when a required Facebook cookie name
// is absent from the input set.
var ErrMissingRequiredCookies = errors.New("missing required cookies")

// FBRequiredCookieNames lists the cookies that Facebook authentication relies
// on. Missing any of them in a session-restore flow yields a redirect-to-login.
var FBRequiredCookieNames = []string{"c_user", "xs", "datr"}

// ValidateFBCookies verifies every name in required is present in cookies.
// Returns ErrMissingRequiredCookies wrapped with the missing names.
func ValidateFBCookies(cookies []Cookie, required []string) error {
	have := make(map[string]struct{}, len(cookies))
	for _, c := range cookies {
		have[c.Name] = struct{}{}
	}
	var missing []string
	for _, n := range required {
		if _, ok := have[n]; !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %v", ErrMissingRequiredCookies, missing)
	}
	return nil
}

// MarshalCookies returns a JSON-encoded representation of cookies safe to
// persist in a TEXT column (after AES-GCM encryption).
func MarshalCookies(cookies []Cookie) (string, error) {
	b, err := json.Marshal(cookies)
	if err != nil {
		return "", fmt.Errorf("marshal cookies: %w", err)
	}
	return string(b), nil
}

// UnmarshalCookies parses a JSON array previously produced by MarshalCookies
// or exported by Cookie-Editor / EditThisCookie.
func UnmarshalCookies(data string) ([]Cookie, error) {
	if data == "" {
		return nil, nil
	}
	var out []Cookie
	if err := json.Unmarshal([]byte(data), &out); err != nil {
		return nil, fmt.Errorf("unmarshal cookies: %w", err)
	}
	return out, nil
}

// SetCookies pushes cookies onto a browser via the CDP Network.setCookies
// command. Use this BEFORE Page.Navigate; cookies set after navigation only
// affect subsequent requests.
func SetCookies(_ context.Context, browser *rod.Browser, cookies []Cookie) error {
	if browser == nil {
		return errors.New("browser is nil")
	}
	if len(cookies) == 0 {
		return nil
	}
	params := make([]*proto.NetworkCookieParam, 0, len(cookies))
	for _, c := range cookies {
		p := &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: cookieSameSite(c.SameSite),
		}
		if c.Expires > 0 {
			p.Expires = proto.TimeSinceEpoch(c.Expires)
		}
		params = append(params, p)
	}
	return browser.SetCookies(params)
}

// DumpCookies reads all cookies from the given domains. Pass empty to read all
// cookies the browser currently knows. The result is in the portable Cookie
// shape (NOT the raw CDP type) so it round-trips cleanly through MarshalCookies.
func DumpCookies(_ context.Context, browser *rod.Browser, domains []string) ([]Cookie, error) {
	if browser == nil {
		return nil, errors.New("browser is nil")
	}
	raw, err := browser.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("get cookies: %w", err)
	}
	out := make([]Cookie, 0, len(raw))
	for _, c := range raw {
		if len(domains) > 0 && !cookieMatchesDomain(c.Domain, domains) {
			continue
		}
		out = append(out, Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  float64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
			SameSite: string(c.SameSite),
		})
	}
	return out, nil
}

func cookieSameSite(v string) proto.NetworkCookieSameSite {
	switch v {
	case "Strict", "strict":
		return proto.NetworkCookieSameSiteStrict
	case "None", "none", "no_restriction":
		return proto.NetworkCookieSameSiteNone
	default:
		return proto.NetworkCookieSameSiteLax
	}
}

// cookieMatchesDomain returns true if cookieDomain matches any entry in
// allow. Matches are suffix-based to handle Facebook's leading-dot convention.
func cookieMatchesDomain(cookieDomain string, allow []string) bool {
	for _, d := range allow {
		if cookieDomain == d {
			return true
		}
		if len(cookieDomain) > 0 && cookieDomain[0] == '.' {
			if cookieDomain[1:] == d {
				return true
			}
		}
	}
	return false
}
