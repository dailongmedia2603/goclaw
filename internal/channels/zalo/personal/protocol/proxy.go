package protocol

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/proxy"
)

// ParseProxyURL normalizes a proxy spec into a *url.URL.
//
// Accepted forms:
//   - Full URL:        "http://user:pass@host:port", "socks5://host:port"
//   - host:port:user:pass  (bare colon form sold by most VN static-IP proxies)
//   - host:port            (no auth)
//
// A bare colon form defaults to the http scheme. Returns (nil, nil) for an
// empty spec so callers can treat "no proxy" as a non-error.
func ParseProxyURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	if strings.Contains(raw, "://") {
		u, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("zalo_personal: invalid proxy url: %w", err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("zalo_personal: proxy url missing host")
		}
		return u, nil
	}

	parts := strings.Split(raw, ":")
	switch len(parts) {
	case 2: // host:port
		return &url.URL{Scheme: "http", Host: parts[0] + ":" + parts[1]}, nil
	case 4: // host:port:user:pass
		return &url.URL{
			Scheme: "http",
			Host:   parts[0] + ":" + parts[1],
			User:   url.UserPassword(parts[2], parts[3]),
		}, nil
	default:
		return nil, fmt.Errorf("zalo_personal: proxy must be host:port, host:port:user:pass, or a full URL")
	}
}

// proxyHTTPTransport builds an *http.Transport that routes through pu.
func proxyHTTPTransport(pu *url.URL) (*http.Transport, error) {
	tr := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	switch strings.ToLower(pu.Scheme) {
	case "http", "https":
		tr.Proxy = http.ProxyURL(pu)
	case "socks5", "socks5h":
		cd, err := socksContextDialer(pu)
		if err != nil {
			return nil, err
		}
		tr.DialContext = cd.DialContext
	default:
		return nil, fmt.Errorf("zalo_personal: unsupported proxy scheme %q", pu.Scheme)
	}
	return tr, nil
}

// applyProxyToWSDialer routes a websocket.Dialer through pu. HTTP proxies use
// the CONNECT method (Dialer.Proxy); SOCKS5 proxies use a custom NetDialContext.
func applyProxyToWSDialer(d *websocket.Dialer, pu *url.URL) error {
	switch strings.ToLower(pu.Scheme) {
	case "http", "https":
		d.Proxy = http.ProxyURL(pu)
	case "socks5", "socks5h":
		cd, err := socksContextDialer(pu)
		if err != nil {
			return err
		}
		d.NetDialContext = cd.DialContext
	default:
		return fmt.Errorf("zalo_personal: unsupported proxy scheme %q", pu.Scheme)
	}
	return nil
}

// socksContextDialer builds a SOCKS5 context dialer (with optional auth) from pu.
func socksContextDialer(pu *url.URL) (proxy.ContextDialer, error) {
	var auth *proxy.Auth
	if pu.User != nil {
		pass, _ := pu.User.Password()
		auth = &proxy.Auth{User: pu.User.Username(), Password: pass}
	}
	d, err := proxy.SOCKS5("tcp", pu.Host, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("zalo_personal: socks5 dialer: %w", err)
	}
	cd, ok := d.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("zalo_personal: socks5 dialer has no DialContext")
	}
	return cd, nil
}

// MaskProxyForLog returns a log-safe rendering that hides any credentials.
func MaskProxyForLog(raw string) string {
	pu, err := ParseProxyURL(raw)
	if err != nil || pu == nil {
		return "(invalid)"
	}
	scheme := strings.ToLower(pu.Scheme)
	if pu.User != nil {
		return scheme + "://***@" + pu.Host
	}
	return scheme + "://" + pu.Host
}
