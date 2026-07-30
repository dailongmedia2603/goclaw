//go:build !sqliteonly

package fbcloak

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// ScreenshotKind enumerates the capture phases. The kind is part of the
// filename so the audit chain pre/post/checkpoint is self-describing.
type ScreenshotKind string

const (
	ScreenshotPre        ScreenshotKind = "pre"
	ScreenshotPostKind   ScreenshotKind = "post"
	ScreenshotCheckpoint ScreenshotKind = "checkpoint"
)

// IsValid reports whether kind is one of the recognised constants.
func (k ScreenshotKind) IsValid() bool {
	switch k {
	case ScreenshotPre, ScreenshotPostKind, ScreenshotCheckpoint:
		return true
	}
	return false
}

// ScreenshotWriter persists raw PNG bytes to a tenant-scoped path. The
// production wire-up is rod-driven (Phase 3c) — this layer owns ONLY path
// resolution + safety, not the capture itself.
type ScreenshotWriter struct {
	// BaseDir is the on-disk root. Resolved at construction; must be an
	// absolute path. Per-tenant subdirectories are created lazily.
	BaseDir string

	// Retention controls how old PNGs may live before PurgeOlderThan
	// removes them. Zero → 30 days.
	Retention time.Duration

	// nowFn is overridable in tests — production uses time.Now.
	nowFn func() time.Time
}

// DefaultScreenshotRetention is the fallback when config leaves the value
// at zero. 30 days is the audit-friendly minimum; longer wastes disk.
const DefaultScreenshotRetention = 30 * 24 * time.Hour

// NewScreenshotWriter builds a writer whose BaseDir is `<dataDir>/fbcloak/screenshots`
// when override is empty. Returns an error when the resolved path is not
// absolute — guards against ".." escape from a relative config value.
func NewScreenshotWriter(dataDir, override string, retention time.Duration) (*ScreenshotWriter, error) {
	base := override
	if base == "" {
		if dataDir == "" {
			return nil, fmt.Errorf("screenshot: dataDir required when override empty")
		}
		base = filepath.Join(dataDir, "fbcloak", "screenshots")
	}
	if !filepath.IsAbs(base) {
		return nil, fmt.Errorf("screenshot: BaseDir must be absolute, got %q", base)
	}
	if retention <= 0 {
		retention = DefaultScreenshotRetention
	}
	return &ScreenshotWriter{
		BaseDir:   filepath.Clean(base),
		Retention: retention,
		nowFn:     time.Now,
	}, nil
}

// PathFor computes the deterministic on-disk path for a (tenant, job, sendLog,
// kind) tuple WITHOUT writing. Returns an error if any UUID is nil — guards
// against accidentally writing to the BaseDir root. The kind is validated
// against the enum so user-controlled strings cannot land in the filename.
func (w *ScreenshotWriter) PathFor(tenantID, jobID, sendLogID uuid.UUID, kind ScreenshotKind) (string, error) {
	if tenantID == uuid.Nil || jobID == uuid.Nil || sendLogID == uuid.Nil {
		return "", fmt.Errorf("screenshot: tenantID/jobID/sendLogID required")
	}
	if !kind.IsValid() {
		return "", fmt.Errorf("screenshot: invalid kind %q", kind)
	}
	now := w.now()
	// {base}/{tenant_id}/{job_id}/{sendLogID}_{kind}_{ts}.png
	dir := filepath.Join(w.BaseDir, tenantID.String(), jobID.String())
	name := fmt.Sprintf("%s_%s_%d.png", sendLogID.String(), kind, now.UnixNano())
	full := filepath.Clean(filepath.Join(dir, name))
	// Defence-in-depth: confirm the cleaned path stays within BaseDir.
	// Without the trailing separator, a sibling dir like /base-evil could
	// match the prefix and bypass the boundary.
	if !pathWithin(w.BaseDir, full) {
		return "", fmt.Errorf("screenshot: resolved path escapes BaseDir: %s", full)
	}
	return full, nil
}

// Save writes raw PNG bytes at the path computed by PathFor and ensures the
// per-tenant/job directory exists. Returns the absolute path so callers can
// store it on the SendLog row.
func (w *ScreenshotWriter) Save(ctx context.Context, tenantID, jobID, sendLogID uuid.UUID, kind ScreenshotKind, png []byte) (string, error) {
	if len(png) == 0 {
		return "", fmt.Errorf("screenshot: empty PNG bytes")
	}
	full, err := w.PathFor(tenantID, jobID, sendLogID, kind)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", fmt.Errorf("screenshot: mkdir: %w", err)
	}
	// 0o600: only the gateway process should read raw screenshots; UI
	// reads via signed URL via the file handler which runs as the same
	// user. Tighter perms surface bugs faster than 0o644.
	if err := os.WriteFile(full, png, 0o600); err != nil {
		return "", fmt.Errorf("screenshot: write: %w", err)
	}
	return full, nil
}

// PurgeOlderThan removes files modified before now-retention from BaseDir.
// Returns the number of files removed. Errors on individual files are
// logged via the caller's logger (we return only the walk error) so a
// single permission glitch doesn't abort the sweep.
func (w *ScreenshotWriter) PurgeOlderThan(ctx context.Context, retention time.Duration) (int, error) {
	if retention <= 0 {
		retention = w.Retention
	}
	cutoff := w.now().Add(-retention)
	removed := 0
	err := filepath.Walk(w.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// BaseDir not yet created → no-op.
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if rmErr := os.Remove(path); rmErr == nil {
				removed++
			}
		}
		return nil
	})
	return removed, err
}

func (w *ScreenshotWriter) now() time.Time {
	if w.nowFn != nil {
		return w.nowFn()
	}
	return time.Now()
}

// pathWithin returns true when `target` lives strictly inside `base` after
// path cleaning. Mirrors the boundary check used in internal/http/files.go.
func pathWithin(base, target string) bool {
	if base == "" {
		return false
	}
	cb := filepath.Clean(base)
	ct := filepath.Clean(target)
	if cb == ct {
		return false // BaseDir itself isn't a valid screenshot file
	}
	rel, err := filepath.Rel(cb, ct)
	if err != nil {
		return false
	}
	if rel == "." || rel == "" {
		return false
	}
	// rel starting with ".." means escape.
	if len(rel) >= 2 && rel[:2] == ".." {
		return false
	}
	return true
}
