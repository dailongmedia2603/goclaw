package browser

import (
	"net/url"
	"sort"
	"strings"
	"sync"
)

// Profile represents a named browser configuration.
type Profile struct {
	Name    string   // unique identifier: "default", "shopee"
	Manager *Manager // owns its own browser connection, refs, pages
	Shared  bool     // true = skip incognito, use main browser context (keeps cookies)
	Domains []string // auto-route: ["shopee.vn", "*.shopee.*"]
	VNCURL  string   // optional: noVNC URL for manual login
}

// ProfileRegistry manages named browser profiles.
type ProfileRegistry struct {
	mu          sync.RWMutex
	profiles    map[string]*Profile
	defaultName string
}

// NewProfileRegistry creates a registry with a default profile name.
func NewProfileRegistry(defaultName string) *ProfileRegistry {
	if defaultName == "" {
		defaultName = "default"
	}
	return &ProfileRegistry{
		profiles:    make(map[string]*Profile),
		defaultName: defaultName,
	}
}

// Register adds a profile to the registry.
func (r *ProfileRegistry) Register(p *Profile) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.profiles[p.Name] = p
}

// Resolve picks the right profile for a request.
// Priority: explicit name > domain match > default.
func (r *ProfileRegistry) Resolve(profileName, targetURL string) *Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Explicit profile selection
	if profileName != "" {
		if p, ok := r.profiles[profileName]; ok {
			return p
		}
	}

	// 2. Domain-based routing
	if targetURL != "" {
		host := extractHost(targetURL)
		if host != "" {
			for _, p := range r.profiles {
				for _, domain := range p.Domains {
					if matchDomain(host, domain) {
						return p
					}
				}
			}
		}
	}

	// 3. Fallback to default
	return r.profiles[r.defaultName]
}

// Default returns the default profile.
func (r *ProfileRegistry) Default() *Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.profiles[r.defaultName]
}

// All returns all profile names sorted alphabetically.
func (r *ProfileRegistry) All() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.profiles))
	for name := range r.profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Profiles returns all profiles.
func (r *ProfileRegistry) Profiles() []*Profile {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Profile, 0, len(r.profiles))
	for _, p := range r.profiles {
		out = append(out, p)
	}
	return out
}

// Close stops all profile managers.
func (r *ProfileRegistry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.profiles {
		p.Manager.Close()
	}
}

// extractHost extracts hostname from a URL string.
func extractHost(rawURL string) string {
	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// matchDomain checks if a host matches a domain pattern.
// Supports exact match ("shopee.vn"), suffix match ("shopee.vn" matches "www.shopee.vn"),
// and wildcard glob ("*.shopee.*", "shopee.*").
func matchDomain(host, pattern string) bool {
	host = strings.ToLower(host)
	pattern = strings.ToLower(pattern)

	if !strings.Contains(pattern, "*") {
		return host == pattern || strings.HasSuffix(host, "."+pattern)
	}

	return globMatch(host, pattern)
}

// globMatch implements wildcard matching where * matches any sequence of characters.
func globMatch(s, p string) bool {
	n, m := len(s), len(p)
	// dp[i][j] = true if s[:i] matches p[:j]
	dp := make([][]bool, n+1)
	for i := range dp {
		dp[i] = make([]bool, m+1)
	}
	dp[0][0] = true
	// Leading *'s can match empty string
	for j := 1; j <= m; j++ {
		if p[j-1] == '*' {
			dp[0][j] = dp[0][j-1]
		}
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if p[j-1] == '*' {
				// * matches zero chars (dp[i][j-1]) or one more char (dp[i-1][j])
				dp[i][j] = dp[i][j-1] || dp[i-1][j]
			} else if p[j-1] == s[i-1] {
				dp[i][j] = dp[i-1][j-1]
			}
		}
	}
	return dp[n][m]
}
