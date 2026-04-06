package browser

import "testing"

func TestMatchDomain(t *testing.T) {
	tests := []struct {
		host, pattern string
		want          bool
	}{
		// Exact match
		{"shopee.vn", "shopee.vn", true},
		{"google.com", "google.com", true},
		{"google.com", "shopee.vn", false},

		// Suffix match
		{"www.shopee.vn", "shopee.vn", true},
		{"m.shopee.vn", "shopee.vn", true},
		{"notshopee.vn", "shopee.vn", false},

		// Wildcard: *.shopee.* means "subdomain.shopee.tld"
		{"www.shopee.com", "*.shopee.*", true},
		{"m.shopee.vn", "*.shopee.*", true},
		{"shopee.co.th", "*.shopee.*", false}, // no subdomain prefix → use "shopee.*" instead
		{"shopee.vn", "*.shopee.*", false},    // no subdomain prefix
		{"google.com", "*.shopee.*", false},

		// Wildcard: shopee.* means "shopee.<anything>"
		{"shopee.vn", "shopee.*", true},
		{"shopee.co.th", "shopee.*", true},
		{"www.shopee.com", "shopee.*", false}, // has subdomain

		// Case insensitive
		{"Shopee.VN", "shopee.vn", true},
		{"WWW.SHOPEE.VN", "*.shopee.*", true},

		// Empty
		{"", "shopee.vn", false},
		{"shopee.vn", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.host+"_"+tt.pattern, func(t *testing.T) {
			got := matchDomain(tt.host, tt.pattern)
			if got != tt.want {
				t.Errorf("matchDomain(%q, %q) = %v, want %v", tt.host, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		rawURL, want string
	}{
		{"https://shopee.vn/search?q=test", "shopee.vn"},
		{"http://www.shopee.co.th/product/123", "www.shopee.co.th"},
		{"shopee.vn", "shopee.vn"},
		{"https://GOOGLE.COM/path", "google.com"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.rawURL, func(t *testing.T) {
			got := extractHost(tt.rawURL)
			if got != tt.want {
				t.Errorf("extractHost(%q) = %q, want %q", tt.rawURL, got, tt.want)
			}
		})
	}
}

func TestProfileRegistryResolve(t *testing.T) {
	reg := NewProfileRegistry("default")

	defaultMgr := New()
	shopeeMgr := New(WithShared(true))

	reg.Register(&Profile{
		Name:    "default",
		Manager: defaultMgr,
	})
	reg.Register(&Profile{
		Name:    "shopee",
		Manager: shopeeMgr,
		Shared:  true,
		Domains: []string{"shopee.*", "*.shopee.*"},
	})

	// Explicit profile
	p := reg.Resolve("shopee", "")
	if p == nil || p.Name != "shopee" {
		t.Fatal("explicit 'shopee' should resolve to shopee profile")
	}

	// Domain match
	p = reg.Resolve("", "https://shopee.vn/search?q=test")
	if p == nil || p.Name != "shopee" {
		t.Fatal("shopee.vn URL should auto-route to shopee profile")
	}

	p = reg.Resolve("", "https://www.shopee.co.th/product/123")
	if p == nil || p.Name != "shopee" {
		t.Fatal("shopee.co.th URL should auto-route to shopee profile")
	}

	// Fallback to default
	p = reg.Resolve("", "https://google.com")
	if p == nil || p.Name != "default" {
		t.Fatal("google.com URL should fall back to default profile")
	}

	// Invalid explicit falls through to default
	p = reg.Resolve("nonexistent", "https://google.com")
	if p == nil || p.Name != "default" {
		t.Fatal("nonexistent profile should fall back to default")
	}

	// No URL, no profile name → default
	p = reg.Resolve("", "")
	if p == nil || p.Name != "default" {
		t.Fatal("empty resolve should return default profile")
	}
}

func TestProfileRegistryAll(t *testing.T) {
	reg := NewProfileRegistry("default")
	reg.Register(&Profile{Name: "default", Manager: New()})
	reg.Register(&Profile{Name: "shopee", Manager: New()})
	reg.Register(&Profile{Name: "facebook", Manager: New()})

	names := reg.All()
	if len(names) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(names))
	}
	// Should be sorted
	if names[0] != "default" || names[1] != "facebook" || names[2] != "shopee" {
		t.Fatalf("expected sorted names [default facebook shopee], got %v", names)
	}
}

func TestProfileRegistryClose(t *testing.T) {
	reg := NewProfileRegistry("default")
	reg.Register(&Profile{Name: "default", Manager: New()})
	reg.Register(&Profile{Name: "shopee", Manager: New()})

	// Close should not panic
	reg.Close()
}
