//go:build !sqliteonly

package fbcloak

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewScreenshotWriter_RejectsRelative(t *testing.T) {
	if _, err := NewScreenshotWriter("", "relative/path", 0); err == nil {
		t.Fatal("expected error for relative override")
	}
	if _, err := NewScreenshotWriter("", "", 0); err == nil {
		t.Fatal("expected error when both dataDir and override empty")
	}
}

func TestNewScreenshotWriter_DefaultsAndOverride(t *testing.T) {
	dataDir := t.TempDir()
	w, err := NewScreenshotWriter(dataDir, "", 0)
	if err != nil {
		t.Fatalf("NewScreenshotWriter: %v", err)
	}
	wantBase := filepath.Join(dataDir, "fbcloak", "screenshots")
	if w.BaseDir != wantBase {
		t.Errorf("BaseDir: got %q, want %q", w.BaseDir, wantBase)
	}
	if w.Retention != DefaultScreenshotRetention {
		t.Errorf("Retention default: got %v, want %v", w.Retention, DefaultScreenshotRetention)
	}

	override := filepath.Join(dataDir, "custom")
	w2, err := NewScreenshotWriter("", override, time.Hour)
	if err != nil {
		t.Fatalf("NewScreenshotWriter override: %v", err)
	}
	if w2.BaseDir != override {
		t.Errorf("override BaseDir: got %q, want %q", w2.BaseDir, override)
	}
	if w2.Retention != time.Hour {
		t.Errorf("override retention: got %v, want %v", w2.Retention, time.Hour)
	}
}

func TestScreenshotWriter_PathFor_TenantScope(t *testing.T) {
	w, err := NewScreenshotWriter(t.TempDir(), "", 0)
	if err != nil {
		t.Fatalf("NewScreenshotWriter: %v", err)
	}

	tenantID := uuid.New()
	jobID := uuid.New()
	sendID := uuid.New()
	got, err := w.PathFor(tenantID, jobID, sendID, ScreenshotPre)
	if err != nil {
		t.Fatalf("PathFor: %v", err)
	}
	if !strings.Contains(got, tenantID.String()) {
		t.Errorf("path missing tenantID: %s", got)
	}
	if !strings.Contains(got, jobID.String()) {
		t.Errorf("path missing jobID: %s", got)
	}
	if !strings.Contains(got, sendID.String()) {
		t.Errorf("path missing sendLogID: %s", got)
	}
	if !strings.HasPrefix(got, w.BaseDir) {
		t.Errorf("path escapes BaseDir: %s vs %s", got, w.BaseDir)
	}
	if !strings.HasSuffix(got, ".png") {
		t.Errorf("path missing .png suffix: %s", got)
	}
}

func TestScreenshotWriter_PathFor_RejectsNilUUID(t *testing.T) {
	w, _ := NewScreenshotWriter(t.TempDir(), "", 0)
	cases := []struct {
		name             string
		tenant, job, send uuid.UUID
	}{
		{"nil tenant", uuid.Nil, uuid.New(), uuid.New()},
		{"nil job", uuid.New(), uuid.Nil, uuid.New()},
		{"nil send", uuid.New(), uuid.New(), uuid.Nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := w.PathFor(c.tenant, c.job, c.send, ScreenshotPre); err == nil {
				t.Errorf("expected error for %s", c.name)
			}
		})
	}
}

func TestScreenshotWriter_PathFor_RejectsInvalidKind(t *testing.T) {
	w, _ := NewScreenshotWriter(t.TempDir(), "", 0)
	if _, err := w.PathFor(uuid.New(), uuid.New(), uuid.New(), ScreenshotKind("../../../etc/passwd")); err == nil {
		t.Fatal("expected error for invalid kind (path traversal attempt)")
	}
	if _, err := w.PathFor(uuid.New(), uuid.New(), uuid.New(), ScreenshotKind("")); err == nil {
		t.Fatal("expected error for empty kind")
	}
}

func TestScreenshotWriter_Save_RoundTrip(t *testing.T) {
	w, _ := NewScreenshotWriter(t.TempDir(), "", 0)
	tenantID := uuid.New()
	jobID := uuid.New()
	sendID := uuid.New()
	want := []byte("\x89PNG\r\n\x1a\nfake")
	got, err := w.Save(context.Background(), tenantID, jobID, sendID, ScreenshotPre, want)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	read, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(read) != string(want) {
		t.Errorf("content mismatch: got %q, want %q", read, want)
	}
	info, _ := os.Stat(got)
	// On macOS/Linux the perm we set is 0600. We don't assert on Windows.
	if info.Mode().Perm()&0o077 != 0 {
		t.Errorf("perm too loose: %v", info.Mode().Perm())
	}
}

func TestScreenshotWriter_Save_RejectsEmpty(t *testing.T) {
	w, _ := NewScreenshotWriter(t.TempDir(), "", 0)
	if _, err := w.Save(context.Background(), uuid.New(), uuid.New(), uuid.New(), ScreenshotPre, nil); err == nil {
		t.Fatal("expected error for empty PNG")
	}
}

func TestScreenshotWriter_PurgeOlderThan(t *testing.T) {
	w, _ := NewScreenshotWriter(t.TempDir(), "", 0)
	tenantID := uuid.New()
	jobID := uuid.New()

	// Write two files: one fresh, one stale.
	freshID := uuid.New()
	staleID := uuid.New()
	_, err := w.Save(context.Background(), tenantID, jobID, freshID, ScreenshotPre, []byte("fresh"))
	if err != nil {
		t.Fatalf("save fresh: %v", err)
	}
	stalePath, err := w.Save(context.Background(), tenantID, jobID, staleID, ScreenshotPre, []byte("stale"))
	if err != nil {
		t.Fatalf("save stale: %v", err)
	}
	// Backdate the stale file.
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(stalePath, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	removed, err := w.PurgeOlderThan(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if removed != 1 {
		t.Errorf("purged: got %d, want 1", removed)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Errorf("stale file still exists: err=%v", err)
	}
}

func TestPathWithin(t *testing.T) {
	cases := []struct {
		base, target string
		want         bool
	}{
		{"/tmp/base", "/tmp/base/a.png", true},
		{"/tmp/base", "/tmp/base/sub/a.png", true},
		{"/tmp/base", "/tmp/base", false},     // base itself
		{"/tmp/base", "/tmp/base/../x", false},
		{"/tmp/base", "/tmp/base-sibling/a", false},
		{"", "/tmp/base/a.png", false},
		{"/tmp/base", "/etc/passwd", false},
	}
	for _, c := range cases {
		got := pathWithin(c.base, c.target)
		if got != c.want {
			t.Errorf("pathWithin(%q, %q): got %v, want %v", c.base, c.target, got, c.want)
		}
	}
}
