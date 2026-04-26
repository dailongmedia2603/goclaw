//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/edition"
)

// fakeCredentialStore is an in-memory CredentialStore used by service tests.
type fakeCredentialStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]Credential
}

func newFakeCredentialStore() *fakeCredentialStore {
	return &fakeCredentialStore{byID: map[uuid.UUID]Credential{}}
}

func (f *fakeCredentialStore) Create(_ context.Context, c Credential) (Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	c.CookiesEnc = "aes-gcm:fake"
	if c.ProxyURL != "" {
		c.ProxyURLEnc = "aes-gcm:fake-proxy"
	}
	f.byID[c.ID] = c
	return c, nil
}

func (f *fakeCredentialStore) Get(_ context.Context, tenantID, id uuid.UUID) (Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok || c.TenantID != tenantID {
		return Credential{}, ErrCredentialNotFound
	}
	return c, nil
}

func (f *fakeCredentialStore) GetByFanpage(_ context.Context, tenantID uuid.UUID, fanpageID string) (Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.byID {
		if c.TenantID == tenantID && c.FanpageID == fanpageID {
			return c, nil
		}
	}
	return Credential{}, ErrCredentialNotFound
}

func (f *fakeCredentialStore) ListByTenant(_ context.Context, tenantID uuid.UUID) ([]Credential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Credential
	for _, c := range f.byID {
		if c.TenantID == tenantID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeCredentialStore) UpdateStatus(_ context.Context, tenantID, id uuid.UUID, st CredentialStatus) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok || c.TenantID != tenantID {
		return ErrCredentialNotFound
	}
	c.Status = st
	f.byID[id] = c
	return nil
}

func (f *fakeCredentialStore) UpdateCookies(_ context.Context, tenantID, id uuid.UUID, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok || c.TenantID != tenantID {
		return ErrCredentialNotFound
	}
	return nil
}

func (f *fakeCredentialStore) UpdateLastCheck(_ context.Context, tenantID, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok || c.TenantID != tenantID {
		return ErrCredentialNotFound
	}
	return nil
}

func (f *fakeCredentialStore) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.byID[id]
	if !ok || c.TenantID != tenantID {
		return ErrCredentialNotFound
	}
	delete(f.byID, id)
	return nil
}

// --- tests ---

func newSvc(t *testing.T) (*Service, *fakeCredentialStore) {
	t.Helper()
	edition.SetCurrent(edition.Standard) // ensure FBCloakEnabled
	store := newFakeCredentialStore()
	svc, err := NewService(Deps{CredentialStore: store})
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, store
}

const validCookieJSON = `[
  {"name":"c_user","value":"100","domain":".facebook.com"},
  {"name":"xs","value":"abc","domain":".facebook.com"},
  {"name":"datr","value":"xyz","domain":".facebook.com"}
]`

func TestAddCredential_HappyPath(t *testing.T) {
	svc, _ := newSvc(t)
	tenant := uuid.New()
	got, err := svc.AddCredential(t.Context(), tenant, CreateCredentialInput{
		FanpageID:   "12345",
		FanpageName: "Test Page",
		Cookies:     validCookieJSON,
	})
	if err != nil {
		t.Fatalf("AddCredential: %v", err)
	}
	if got.Cookies != "" {
		t.Error("plaintext Cookies leaked in response")
	}
	if got.CookiesEnc == "" {
		t.Error("CookiesEnc empty in response")
	}
	if got.UserAgent == "" {
		t.Error("default UserAgent not applied")
	}
}

func TestAddCredential_RejectsMissingCookies(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.AddCredential(t.Context(), uuid.New(), CreateCredentialInput{
		FanpageID:   "12345",
		FanpageName: "P",
		Cookies:     `[{"name":"c_user","value":"1","domain":".facebook.com"}]`,
	})
	if err == nil {
		t.Fatal("expected error when xs/datr missing")
	}
}

func TestAddCredential_RejectsBadJSON(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.AddCredential(t.Context(), uuid.New(), CreateCredentialInput{
		FanpageID:   "12345",
		FanpageName: "P",
		Cookies:     `not-json`,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid cookies JSON") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
}

func TestAddCredential_RejectsBadProxy(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.AddCredential(t.Context(), uuid.New(), CreateCredentialInput{
		FanpageID:   "12345",
		FanpageName: "P",
		Cookies:     validCookieJSON,
		ProxyURL:    "not-a-url",
	})
	if !errors.Is(err, ErrInvalidProxyURL) {
		t.Fatalf("expected ErrInvalidProxyURL, got %v", err)
	}
}

func TestAddCredential_AcceptsSocks5(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.AddCredential(t.Context(), uuid.New(), CreateCredentialInput{
		FanpageID:   "12345",
		FanpageName: "P",
		Cookies:     validCookieJSON,
		ProxyURL:    "socks5://user:pass@host:1080",
	})
	if err != nil {
		t.Fatalf("expected acceptance of socks5 URL, got %v", err)
	}
}

func TestListCredentials_TenantIsolation(t *testing.T) {
	svc, _ := newSvc(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	if _, err := svc.AddCredential(t.Context(), tenantA, CreateCredentialInput{FanpageID: "A", FanpageName: "A", Cookies: validCookieJSON}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddCredential(t.Context(), tenantB, CreateCredentialInput{FanpageID: "B", FanpageName: "B", Cookies: validCookieJSON}); err != nil {
		t.Fatal(err)
	}

	listA, _ := svc.ListCredentials(t.Context(), tenantA)
	listB, _ := svc.ListCredentials(t.Context(), tenantB)
	if len(listA) != 1 || listA[0].FanpageID != "A" {
		t.Errorf("tenant A should see only A, got %+v", listA)
	}
	if len(listB) != 1 || listB[0].FanpageID != "B" {
		t.Errorf("tenant B should see only B, got %+v", listB)
	}
	for _, c := range listA {
		if c.Cookies != "" {
			t.Errorf("plaintext cookies leaked from List: %+v", c)
		}
	}
}

func TestService_KillswitchBlocks(t *testing.T) {
	svc, _ := newSvc(t)
	svc.SetKillswitch(true)
	_, err := svc.AddCredential(t.Context(), uuid.New(), CreateCredentialInput{FanpageID: "X", FanpageName: "X", Cookies: validCookieJSON})
	if !errors.Is(err, ErrKillswitchActive) {
		t.Fatalf("expected ErrKillswitchActive, got %v", err)
	}
}

func TestService_EditionGate(t *testing.T) {
	defer edition.SetCurrent(edition.Standard) // restore after the lite swap
	store := newFakeCredentialStore()
	svc, _ := NewService(Deps{CredentialStore: store})
	edition.SetCurrent(edition.Lite)
	_, err := svc.AddCredential(t.Context(), uuid.New(), CreateCredentialInput{FanpageID: "X", FanpageName: "X", Cookies: validCookieJSON})
	if !errors.Is(err, ErrFeatureDisabled) {
		t.Fatalf("expected ErrFeatureDisabled on Lite, got %v", err)
	}
}

func TestNewService_RequiresStore(t *testing.T) {
	_, err := NewService(Deps{})
	if err == nil {
		t.Error("expected error when CredentialStore nil")
	}
}

func TestRedact(t *testing.T) {
	c := Credential{Cookies: "secret", ProxyURL: "socks5://u:p@h:1", CookiesEnc: "aes:enc"}
	r := redact(c)
	if r.Cookies != "" || r.ProxyURL != "" {
		t.Errorf("redact failed: %+v", r)
	}
	if r.CookiesEnc != "aes:enc" {
		t.Error("redact must keep encrypted column")
	}
}
