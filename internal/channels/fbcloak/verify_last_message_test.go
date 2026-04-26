//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubInspector struct {
	ax, react, raw string
	err            error
}

func (s *stubInspector) LastMessageMarkers(_ context.Context) (string, string, string, error) {
	return s.ax, s.react, s.raw, s.err
}

func TestVerify_HappyPath(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	expected := Target{LastMessageAt: now.Add(-10 * 24 * time.Hour)} // DB says 10 days ago
	ins := &stubInspector{ax: "Jane • 10 ngày"}
	v, err := VerifyLastMessage(t.Context(), ins, expected, VerifyConfig{
		Tolerance: 2 * 24 * time.Hour,
		MinIdle:   7 * 24 * time.Hour,
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !v.OK {
		t.Errorf("expected OK=true, got %+v", v)
	}
}

func TestVerify_CustomerRepliedRecently(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	// DB says 30 days ago; actual UI shows 1 day ago → customer replied.
	expected := Target{LastMessageAt: now.Add(-30 * 24 * time.Hour)}
	ins := &stubInspector{ax: "Jane • 1 ngày"}
	v, err := VerifyLastMessage(t.Context(), ins, expected, VerifyConfig{
		Tolerance: 2 * 24 * time.Hour,
		MinIdle:   7 * 24 * time.Hour,
		Now:       func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.OK {
		t.Errorf("expected OK=false (customer replied), got %+v", v)
	}
	if v.Mismatch != "customer_replied_recently" {
		t.Errorf("mismatch: got %s, want customer_replied_recently", v.Mismatch)
	}
}

func TestVerify_NoMessages(t *testing.T) {
	ins := &stubInspector{}
	v, err := VerifyLastMessage(t.Context(), ins, Target{}, VerifyConfig{Now: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	if v.OK {
		t.Error("empty thread should not be OK")
	}
	if v.Mismatch != "no_messages" {
		t.Errorf("mismatch: got %s, want no_messages", v.Mismatch)
	}
}

func TestVerify_ParseFailed(t *testing.T) {
	ins := &stubInspector{ax: "no timestamp here at all"}
	v, _ := VerifyLastMessage(t.Context(), ins, Target{}, VerifyConfig{Now: time.Now})
	if v.OK {
		t.Error("unparsable should not be OK")
	}
	if v.Mismatch != "parse_failed" {
		t.Errorf("mismatch: got %s, want parse_failed", v.Mismatch)
	}
}

func TestVerify_InspectorError(t *testing.T) {
	wantErr := errors.New("dom gone")
	ins := &stubInspector{err: wantErr}
	v, err := VerifyLastMessage(t.Context(), ins, Target{}, VerifyConfig{Now: time.Now})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wantErr propagation, got %v", err)
	}
	if v.OK {
		t.Error("inspector error should not be OK")
	}
}

func TestVerify_NilInspector(t *testing.T) {
	_, err := VerifyLastMessage(t.Context(), nil, Target{}, VerifyConfig{})
	if err == nil {
		t.Error("expected error for nil inspector")
	}
}
