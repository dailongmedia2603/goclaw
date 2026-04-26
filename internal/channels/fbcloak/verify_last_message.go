//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"time"
)

// VerifyVerdict captures the outcome of comparing what's currently displayed
// in a thread to what fbcloak's database says was the last contact.
type VerifyVerdict struct {
	OK       bool
	Mismatch string    // "customer_replied_recently" | "no_messages" | "parse_failed"
	ActualAt time.Time // best-effort parsed timestamp; zero if unparsable
}

// ThreadInspector is the subset of browser surface area VerifyLastMessage
// needs. send_executor wires the rod-based implementation in production;
// tests can stub it without spinning up Chrome.
type ThreadInspector interface {
	// LastMessageMarkers returns whatever metadata strings are visible at
	// the bottom of an open thread: the AX node name, the JSON-ish React
	// dump (may be empty if unsupported), and the raw text of the message
	// timestamp area.
	LastMessageMarkers(ctx context.Context) (axName, reactDump, rawText string, err error)
}

// VerifyConfig carries the tolerance window. Default = 2 days; if the actual
// last message is newer than DB timestamp by more than this, we treat it as
// "customer replied recently" and skip the send.
type VerifyConfig struct {
	Tolerance time.Duration
	MinIdle   time.Duration // mirror of Job.TargetMinIdle so we can guard "recent reply within window"
	Now       func() time.Time
}

// VerifyLastMessage opens the inspector, parses the last-activity strings,
// and decides whether to proceed with the send.
func VerifyLastMessage(ctx context.Context, ins ThreadInspector, expected Target, cfg VerifyConfig) (VerifyVerdict, error) {
	if ins == nil {
		return VerifyVerdict{}, errors.New("verify: nil ThreadInspector")
	}
	now := time.Now
	if cfg.Now != nil {
		now = cfg.Now
	}
	tolerance := cfg.Tolerance
	if tolerance == 0 {
		tolerance = 2 * 24 * time.Hour
	}

	axName, reactDump, rawText, err := ins.LastMessageMarkers(ctx)
	if err != nil {
		return VerifyVerdict{Mismatch: "parse_failed"}, err
	}
	if axName == "" && reactDump == "" && rawText == "" {
		return VerifyVerdict{Mismatch: "no_messages"}, nil
	}

	parsed, err := ParseLastActivity(axName, reactDump, rawText, now())
	if err != nil {
		return VerifyVerdict{Mismatch: "parse_failed"}, nil
	}

	delta := parsed.At.Sub(expected.LastMessageAt)
	// Conservative: customer replied more recently than DB by > tolerance
	// AND the new actual is inside the MinIdle window → skip.
	if delta > tolerance && now().Sub(parsed.At) < cfg.MinIdle {
		return VerifyVerdict{Mismatch: "customer_replied_recently", ActualAt: parsed.At}, nil
	}
	return VerifyVerdict{OK: true, ActualAt: parsed.At}, nil
}
