package browser

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateFBCookies_AllPresent(t *testing.T) {
	cookies := []Cookie{
		{Name: "c_user", Value: "100"},
		{Name: "xs", Value: "abc"},
		{Name: "datr", Value: "xyz"},
		{Name: "fr", Value: "ignored"},
	}
	if err := ValidateFBCookies(cookies, FBRequiredCookieNames); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
}

func TestValidateFBCookies_MissingXs(t *testing.T) {
	cookies := []Cookie{
		{Name: "c_user", Value: "100"},
		{Name: "datr", Value: "xyz"},
	}
	err := ValidateFBCookies(cookies, FBRequiredCookieNames)
	if err == nil {
		t.Fatal("expected error for missing xs")
	}
	if !errors.Is(err, ErrMissingRequiredCookies) {
		t.Errorf("expected ErrMissingRequiredCookies, got %v", err)
	}
	if !strings.Contains(err.Error(), "xs") {
		t.Errorf("error should mention 'xs', got %q", err.Error())
	}
}

func TestValidateFBCookies_AllMissing(t *testing.T) {
	err := ValidateFBCookies([]Cookie{{Name: "fr"}}, FBRequiredCookieNames)
	if err == nil {
		t.Fatal("expected error for all required missing")
	}
	for _, name := range FBRequiredCookieNames {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should list %q, got %q", name, err.Error())
		}
	}
}

func TestMarshalUnmarshalCookies_Roundtrip(t *testing.T) {
	in := []Cookie{
		{Name: "c_user", Value: "100", Domain: ".facebook.com", Path: "/", Secure: true, HTTPOnly: true, SameSite: "Lax"},
		{Name: "xs", Value: "abc%3Adef", Domain: ".facebook.com", Path: "/", Secure: true, HTTPOnly: true, SameSite: "Lax", Expires: 1700000000},
	}
	encoded, err := MarshalCookies(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out, err := UnmarshalCookies(encoded)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("length mismatch: got %d, want %d", len(out), len(in))
	}
	for i := range in {
		if out[i] != in[i] {
			t.Errorf("cookie %d mismatch:\n got: %+v\nwant: %+v", i, out[i], in[i])
		}
	}
}

func TestUnmarshalCookies_Empty(t *testing.T) {
	out, err := UnmarshalCookies("")
	if err != nil {
		t.Fatalf("unmarshal empty: %v", err)
	}
	if out != nil {
		t.Errorf("expected nil for empty input, got %v", out)
	}
}

func TestUnmarshalCookies_InvalidJSON(t *testing.T) {
	_, err := UnmarshalCookies("not-json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCookieMatchesDomain(t *testing.T) {
	cases := []struct {
		cookieDomain string
		allow        []string
		want         bool
	}{
		{".facebook.com", []string{"facebook.com"}, true},
		{"facebook.com", []string{"facebook.com"}, true},
		{".messenger.com", []string{"facebook.com"}, false},
		{"x.facebook.com", []string{"facebook.com"}, false}, // strict suffix-with-leading-dot only
		{".facebook.com", []string{"messenger.com", "facebook.com"}, true},
	}
	for _, tc := range cases {
		got := cookieMatchesDomain(tc.cookieDomain, tc.allow)
		if got != tc.want {
			t.Errorf("cookieMatchesDomain(%q, %v) = %v, want %v", tc.cookieDomain, tc.allow, got, tc.want)
		}
	}
}

func TestCookieSameSite(t *testing.T) {
	cases := map[string]string{
		"Lax":            "Lax",
		"lax":            "Lax",
		"Strict":         "Strict",
		"strict":         "Strict",
		"None":           "None",
		"no_restriction": "None",
		"":               "Lax", // default
	}
	for input, want := range cases {
		got := string(cookieSameSite(input))
		if got != want {
			t.Errorf("cookieSameSite(%q) = %q, want %q", input, got, want)
		}
	}
}
