//go:build !sqliteonly

package fbcloak

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedTimestamp is the result of attempting to parse a "X ago" string into
// an absolute time. Source records which tier produced the value so the job
// runner can log fall-back patterns.
type ParsedTimestamp struct {
	At         time.Time
	Source     string  // "ax" | "react" | "regex"
	Confidence float64 // 0..1 — Tier 1 = 1.0, Tier 2 = 0.95, Tier 3 = 0.6
}

// ErrTimestampUnparsable indicates none of the three tiers could extract a
// timestamp; the caller should fall back to a screenshot + skip this entry.
var ErrTimestampUnparsable = errors.New("fbcloak: timestamp unparsable")

// vnRelativePattern captures Vietnamese relative-time expressions seen in
// Meta Business Suite ("3 ngày", "1 tuần", "2 tháng", etc.). The English
// alternates are kept because Meta sometimes A/B-tests UI copy.
var vnRelativePattern = regexp.MustCompile(
	`(?i)(\d+)\s*(giây|phút|giờ|ngày|tuần|tháng|năm|s|min|h|d|w|mo|y)`,
)

// ParseTier1AX extracts a timestamp from an accessibility tree node "name"
// attribute. The string typically looks like "Jane Doe • 3 ngày" — we grab
// the relative phrase and convert.
func ParseTier1AX(axName string, now time.Time) (ParsedTimestamp, error) {
	if axName == "" {
		return ParsedTimestamp{}, ErrTimestampUnparsable
	}
	at, ok := parseRelativeVN(axName, now)
	if !ok {
		return ParsedTimestamp{}, ErrTimestampUnparsable
	}
	return ParsedTimestamp{At: at, Source: "ax", Confidence: 1.0}, nil
}

// ParseTier2React expects a JSON-ish blob (whatever React props expose) and
// looks for a "lastActivityTimestampMs" key. We deliberately do NOT use
// encoding/json to parse — the structure is unstable; substring extraction
// is more resilient.
func ParseTier2React(reactDump string, now time.Time) (ParsedTimestamp, error) {
	if reactDump == "" {
		return ParsedTimestamp{}, ErrTimestampUnparsable
	}
	const key = "lastActivityTimestampMs"
	_, after, ok := strings.Cut(reactDump, key)
	if !ok {
		return ParsedTimestamp{}, ErrTimestampUnparsable
	}
	tail := after
	// Skip non-digit characters until we hit the number.
	digits := strings.Builder{}
	for _, r := range tail {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		if digits.Len() > 0 {
			break
		}
	}
	if digits.Len() == 0 {
		return ParsedTimestamp{}, ErrTimestampUnparsable
	}
	ms, err := strconv.ParseInt(digits.String(), 10, 64)
	if err != nil {
		return ParsedTimestamp{}, fmt.Errorf("react ms: %w", err)
	}
	_ = now // present in signature for symmetry with the other tiers
	return ParsedTimestamp{
		At:         time.UnixMilli(ms).UTC(),
		Source:     "react",
		Confidence: 0.95,
	}, nil
}

// ParseTier3Regex is the last-resort regex on the listitem's plain text.
// Lower confidence — strings like "3 ngày" can come from message previews
// rather than the activity stamp itself.
func ParseTier3Regex(rawText string, now time.Time) (ParsedTimestamp, error) {
	if rawText == "" {
		return ParsedTimestamp{}, ErrTimestampUnparsable
	}
	at, ok := parseRelativeVN(rawText, now)
	if !ok {
		return ParsedTimestamp{}, ErrTimestampUnparsable
	}
	return ParsedTimestamp{At: at, Source: "regex", Confidence: 0.6}, nil
}

// ParseLastActivity tries the three tiers in order and returns the first
// success. axName / reactDump may be empty; rawText is the safety-net.
func ParseLastActivity(axName, reactDump, rawText string, now time.Time) (ParsedTimestamp, error) {
	if t, err := ParseTier1AX(axName, now); err == nil {
		return t, nil
	}
	if t, err := ParseTier2React(reactDump, now); err == nil {
		return t, nil
	}
	if t, err := ParseTier3Regex(rawText, now); err == nil {
		return t, nil
	}
	return ParsedTimestamp{}, ErrTimestampUnparsable
}

// parseRelativeVN converts the first "X <unit>" match in the input into an
// absolute time relative to now. Unit aliases handle both VN and the few
// English short-forms Meta sometimes ships.
func parseRelativeVN(s string, now time.Time) (time.Time, bool) {
	m := vnRelativePattern.FindStringSubmatch(s)
	if len(m) != 3 {
		return time.Time{}, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return time.Time{}, false
	}
	unit := strings.ToLower(m[2])
	var dur time.Duration
	switch unit {
	case "giây", "s":
		dur = time.Duration(n) * time.Second
	case "phút", "min":
		dur = time.Duration(n) * time.Minute
	case "giờ", "h":
		dur = time.Duration(n) * time.Hour
	case "ngày", "d":
		dur = time.Duration(n) * 24 * time.Hour
	case "tuần", "w":
		dur = time.Duration(n) * 7 * 24 * time.Hour
	case "tháng", "mo":
		dur = time.Duration(n) * 30 * 24 * time.Hour
	case "năm", "y":
		dur = time.Duration(n) * 365 * 24 * time.Hour
	default:
		return time.Time{}, false
	}
	return now.Add(-dur), true
}
