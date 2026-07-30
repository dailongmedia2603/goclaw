# Multi-Profile Browser Architecture

> Research Report — April 2026

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Problem Statement](#problem-statement)
3. [Current Architecture](#current-architecture)
4. [Architecture Design](#architecture-design)
5. [Configuration Design](#configuration-design)
6. [Routing Strategy](#routing-strategy)
7. [Code Changes](#code-changes)
8. [Deployment: Cloak Browser + VNC](#deployment)
9. [Security Considerations](#security-considerations)
10. [Implementation Roadmap](#implementation-roadmap)

---

## Executive Summary

GoClaw currently supports a single browser instance (local Chrome or remote CDP sidecar). This works for general web browsing but fails for anti-bot platforms like Shopee that detect automated browsers.

**Solution:** Introduce a **Profile Registry** pattern — a pool of named browser profiles, each backed by its own `Manager` instance with independent CDP endpoint, isolation mode, and lifecycle. The LLM agent selects profiles via an explicit `profile` parameter or automatic domain-based routing.

Key design decisions:
- **Profile Registry wraps existing Manager** — zero changes to core Manager logic
- **Shared vs Isolated mode** — shared profiles keep cookies/sessions (for manual login), isolated profiles use incognito per tenant (current behavior)
- **Domain-based auto-routing** — URLs matching profile tags auto-select the right profile
- **Backward compatible** — flat config (no `profiles` key) creates a single "default" profile

---

## Problem Statement

| Scenario | Current | Needed |
|----------|---------|--------|
| General web check | Chrome headless ✅ | Same |
| Shopee scraping | Detected as bot ❌ | Cloak Browser + manual login session |
| Future: Facebook, Lazada | N/A | Additional anti-detect profiles |
| Multiple concurrent browsers | Not supported ❌ | Independent profiles with own CDP |

**Core tension:** Anti-bot sites require (1) anti-detect browser fingerprinting and (2) persistent authenticated sessions from manual login. Current incognito-per-tenant isolation destroys both.

---

## Current Architecture

```
┌─────────────────┐
│   BrowserTool   │  (tools.Tool interface)
│   tool.go       │
└────────┬────────┘
         │ single manager
┌────────▼────────┐
│    Manager      │  (browser.go)
│  - 1 browser    │
│  - tenantCtxs   │  incognito per tenant
│  - refs/pages   │
└────────┬────────┘
         │ CDP WebSocket
┌────────▼────────┐
│  Chrome/Remote  │  single instance
└─────────────────┘
```

**Limitation:** One Manager → one browser → one CDP endpoint. All tenants share the same browser instance with incognito isolation.

---

## Architecture Design

### Option Analysis

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A. Multi-Manager Pool** | ProfileRegistry holds map of Manager instances | Clean separation, each profile independent, existing Manager unchanged | Slightly more memory per profile |
| B. Single Manager Multi-Browser | Extend Manager to hold multiple `*rod.Browser` | Less structs | Manager becomes complex, harder to isolate failures |
| C. CDP Proxy | External proxy routes to different browsers | Language agnostic | Extra infra, latency, debugging harder |

**Chosen: Option A — Profile Registry** (simplest, cleanest, KISS)

### Architecture Diagram

```
┌──────────────────────────────────────────────────────┐
│                    BrowserTool                        │
│  tool.go — adds "profile" param to all actions       │
└──────────────┬───────────────────────────────────────┘
               │ selects profile
┌──────────────▼───────────────────────────────────────┐
│              ProfileRegistry                          │
│  registry.go — map[string]*Profile                   │
│                                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │
│  │  "default"  │  │  "shopee"   │  │  "facebook"  │  │
│  │  Manager    │  │  Manager    │  │  Manager     │  │
│  │  headless   │  │  remote CDP │  │  remote CDP  │  │
│  │  isolated   │  │  shared     │  │  shared      │  │
│  └──────┬──────┘  └──────┬──────┘  └──────┬───────┘  │
└─────────┼────────────────┼────────────────┼──────────┘
          │                │                │
    ┌─────▼─────┐   ┌─────▼──────┐   ┌─────▼──────┐
    │  Chrome   │   │   Cloak    │   │  GoLogin   │
    │  headless │   │  Browser   │   │  profile   │
    │  (local)  │   │  (VNC)     │   │  (remote)  │
    └───────────┘   └────────────┘   └────────────┘
```

### Profile Structure

```go
// Profile represents a named browser configuration.
type Profile struct {
    Name     string   // unique identifier: "default", "shopee", etc.
    Manager  *Manager // owns its own browser connection, refs, pages
    Shared   bool     // true = skip incognito, use main browser context (keeps cookies)
    Domains  []string // auto-route: ["shopee.vn", "*.shopee.*"]
    VNCURL   string   // optional: "http://host:6080" for manual login
}

// ProfileRegistry manages named browser profiles.
type ProfileRegistry struct {
    mu       sync.RWMutex
    profiles map[string]*Profile
    default_ string // fallback profile name
}
```

### Shared vs Isolated Mode

```
Isolated (current default):
  Tenant A → Incognito Context A → fresh cookies
  Tenant B → Incognito Context B → fresh cookies
  ✅ Tenant isolation
  ❌ No persistent session

Shared (new):
  Manual login → Main browser context → cookies saved
  Tenant A → Main browser context → uses saved cookies
  Tenant B → Main browser context → uses saved cookies
  ✅ Persistent authenticated session
  ❌ No tenant isolation (acceptable for dedicated-purpose profiles)
```

**Implementation:** Single change in `tenantBrowserLocked()`:

```go
func (m *Manager) tenantBrowserLocked(tenantID string) (*rod.Browser, error) {
    if m.browser == nil {
        return nil, fmt.Errorf("browser not running")
    }
    // Shared mode: always use main browser (preserves cookies/session)
    if m.shared {
        return m.browser, nil
    }
    // Master tenant or no tenant: use main browser
    if tenantID == "" || tenantID == MasterTenantID {
        return m.browser, nil
    }
    // ... existing incognito logic
}
```

---

## Configuration Design

### New Config Structure

```go
// BrowserToolConfig controls the browser automation tool.
type BrowserToolConfig struct {
    Enabled        bool                        `json:"enabled"`
    DefaultProfile string                      `json:"default_profile,omitempty"` // default: "default"
    Profiles       map[string]BrowserProfile   `json:"profiles,omitempty"`

    // Legacy flat fields — used as "default" profile when Profiles is empty
    Headless        bool   `json:"headless,omitempty"`
    RemoteURL       string `json:"remote_url,omitempty"`
    ActionTimeoutMs int    `json:"action_timeout_ms,omitempty"`
    IdleTimeoutMs   int    `json:"idle_timeout_ms,omitempty"`
    MaxPages        int    `json:"max_pages,omitempty"`
}

// BrowserProfile configures a single named browser profile.
type BrowserProfile struct {
    RemoteURL       string   `json:"remote_url,omitempty"`       // CDP endpoint
    Headless        bool     `json:"headless,omitempty"`         // local Chrome only
    Shared          bool     `json:"shared,omitempty"`           // skip incognito isolation
    Domains         []string `json:"domains,omitempty"`          // auto-route domains
    VNCURL          string   `json:"vnc_url,omitempty"`          // noVNC URL for manual login
    ActionTimeoutMs int      `json:"action_timeout_ms,omitempty"`
    IdleTimeoutMs   int      `json:"idle_timeout_ms,omitempty"`
    MaxPages        int      `json:"max_pages,omitempty"`
}
```

### Backward Compatibility

When `profiles` is empty/nil, the flat fields are used to create a single "default" profile:

```go
func (c *BrowserToolConfig) ResolvedProfiles() map[string]BrowserProfile {
    if len(c.Profiles) > 0 {
        return c.Profiles
    }
    // Legacy: flat fields → single "default" profile
    return map[string]BrowserProfile{
        "default": {
            RemoteURL:       c.RemoteURL,
            Headless:        c.Headless,
            ActionTimeoutMs: c.ActionTimeoutMs,
            IdleTimeoutMs:   c.IdleTimeoutMs,
            MaxPages:        c.MaxPages,
        },
    }
}
```

### Example config.json5

```json5
{
  "tools": {
    "browser": {
      "enabled": true,
      "default_profile": "default",
      "profiles": {
        // General web browsing — headless Chrome, incognito per tenant
        "default": {
          "headless": true,
          "action_timeout_ms": 30000,
          "idle_timeout_ms": 600000,
          "max_pages": 5
        },
        // Shopee — Cloak Browser, shared session, manual login via VNC
        "shopee": {
          "remote_url": "ws://cloak-shopee:9222",
          "shared": true,
          "domains": ["shopee.vn", "shopee.co.th", "shopee.com"],
          "vnc_url": "http://103.97.126.134:6080/?path=vnc/shopee",
          "action_timeout_ms": 60000,
          "idle_timeout_ms": -1,
          "max_pages": 3
        }
      }
    }
  }
}
```

### Environment Variables

```bash
# Legacy (backward compat) — creates "default" profile
GOCLAW_BROWSER_REMOTE_URL="ws://chrome:9222"

# Multi-profile via env (comma-separated profiles, colon-separated fields)
# Not recommended — use config.json5 for multi-profile
```

---

## Routing Strategy

### Three-Layer Resolution

```
1. Explicit:  profile="shopee"           → use "shopee" profile
2. Domain:    targetUrl contains shopee.vn → match "shopee" profile domains
3. Fallback:  no match                    → use default_profile
```

### Implementation

```go
// ResolveProfile picks the right profile for a request.
func (r *ProfileRegistry) ResolveProfile(explicit string, targetURL string) *Profile {
    // 1. Explicit profile selection
    if explicit != "" {
        if p, ok := r.profiles[explicit]; ok {
            return p
        }
    }

    // 2. Domain-based routing
    if targetURL != "" {
        host := extractHost(targetURL)
        for _, p := range r.profiles {
            for _, domain := range p.Domains {
                if matchDomain(host, domain) {
                    return p
                }
            }
        }
    }

    // 3. Fallback to default
    return r.profiles[r.default_]
}
```

### Tool Parameter Addition

```go
// New parameter in BrowserTool.Parameters():
"profile": {
    "type":        "string",
    "description": "Browser profile to use (e.g. 'default', 'shopee'). Auto-selected by URL if omitted.",
}
```

The LLM sees available profiles in the tool description and can explicitly select one, or let the router auto-select based on the URL.

---

## Code Changes

### Files to Create

| File | Purpose |
|------|---------|
| `pkg/browser/registry.go` | `ProfileRegistry` — manages named profiles |

### Files to Modify

| File | Change |
|------|--------|
| `pkg/browser/browser.go` | Add `shared bool` field + `WithShared()` option |
| `pkg/browser/browser.go` | `tenantBrowserLocked()` — return main browser when `shared=true` |
| `pkg/browser/tool.go` | Replace `*Manager` with `*ProfileRegistry`, add `profile` param |
| `pkg/browser/tool.go` | Route each action to resolved profile's Manager |
| `internal/config/config_channels.go` | Add `BrowserProfile`, `Profiles`, `DefaultProfile` |
| `internal/config/config_load.go` | Parse profiles, env var overlay |
| `cmd/gateway_setup.go` | Build ProfileRegistry from config, register tool |

### Estimated Scope

- **~200 lines new** (registry.go, domain matcher)
- **~80 lines modified** (tool.go routing, config, setup)
- **~5 lines modified** (Manager shared mode)
- **Zero breaking changes** — flat config continues to work

---

## Deployment

### Cloak Browser + noVNC on VPS

```
┌─────────────────────────────────────────────────┐
│  VPS 103.97.126.134                             │
│                                                  │
│  ┌──────────────┐  ┌──────────────────────────┐ │
│  │  GoClaw      │  │  cloak-shopee container  │ │
│  │  Gateway     │──│  Xvfb + Cloak Browser    │ │
│  │              │  │  CDP :9222               │ │
│  └──────────────┘  │  VNC :5900               │ │
│                     │  noVNC :6080             │ │
│  ┌──────────────┐  └──────────────────────────┘ │
│  │  chrome      │                                │
│  │  (headless)  │  ← default profile            │
│  │  CDP :9223   │                                │
│  └──────────────┘                                │
└─────────────────────────────────────────────────┘
```

### Docker Compose Addition

```yaml
services:
  cloak-shopee:
    image: kasmweb/chrome:1.16.0  # or custom Cloak Browser image
    environment:
      - VNC_PW=secure_password
      - DISPLAY=:1
    ports:
      - "6080:6080"   # noVNC web interface
      - "9222:9222"   # CDP endpoint
    volumes:
      - cloak_shopee_data:/home/user/.config  # persist browser profile
    command: >
      bash -c "
        Xvfb :1 -screen 0 1920x1080x24 &
        x11vnc -display :1 -forever -nopw &
        /opt/noVNC/utils/novnc_proxy --vnc localhost:5900 --listen 6080 &
        /usr/bin/google-chrome
          --remote-debugging-port=9222
          --remote-debugging-address=0.0.0.0
          --user-data-dir=/home/user/.config/cloak-shopee
          --no-first-run
          --disable-gpu
          --window-size=1920,1080
      "
    networks:
      - goclaw

  # Existing headless Chrome for general browsing
  chrome:
    image: chromedp/headless-shell:latest
    ports:
      - "9223:9222"
    networks:
      - goclaw

volumes:
  cloak_shopee_data:
```

### Manual Login Workflow

```
1. User opens: http://103.97.126.134:6080/?path=vnc/shopee
2. Sees Cloak Browser desktop via noVNC
3. Navigates to shopee.vn, logs in manually
4. Session cookies persist in browser profile volume
5. Bot uses the same browser via CDP → authenticated requests
```

### Using Actual Cloak Browser

If using the real Cloak Browser app (not generic Chrome):

1. Install Cloak Browser on a Linux VM with desktop
2. Create a profile in Cloak Browser GUI
3. Launch with `--remote-debugging-port=9222`
4. Set up VNC for remote desktop access
5. Configure GoClaw: `"remote_url": "ws://<cloak-vm-ip>:9222"`

---

## Security Considerations

| Risk | Mitigation |
|------|------------|
| Shared session = all tenants see Shopee cookies | Shared profiles should be purpose-specific; one profile per service |
| VNC exposed to internet | Password-protect noVNC + restrict to admin IPs via firewall |
| CDP port exposed | Only expose on internal Docker network, never to public |
| Profile cookie theft | Volume encryption, restrict container access |
| Session expiry | Periodic re-login reminder via bot status check |

---

## Implementation Roadmap

### Phase 1: Core (MVP)

1. Add `shared` field to Manager + `WithShared()` option
2. Create `ProfileRegistry` (registry.go)
3. Update `BrowserToolConfig` with profiles support
4. Update `BrowserTool` to use registry + `profile` param
5. Update `gateway_setup.go` to build registry from config

### Phase 2: Deployment

6. Set up Cloak Browser Docker container with VNC
7. Configure multi-profile in config.json5
8. Test manual login + bot automation flow

### Phase 3: Polish

9. Add `browser status` action to show all profiles
10. Add session health check (is Shopee still logged in?)
11. Document in CLAUDE.md

---

## Unresolved Questions

1. **Cloak Browser in Docker?** — Real Cloak Browser is a desktop app. Docker alternative: use Kasm Chrome image with stealth patches, or run Cloak Browser on a separate VM with VNC.
2. **Session expiry handling** — How to detect and notify when Shopee session expires? Could add a periodic health check in the reaper goroutine.
3. **Profile hot-reload** — Should profiles be addable at runtime without restart? YAGNI for now — restart is fine.
