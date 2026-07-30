//go:build !sqliteonly

package fbcloak

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// PlanDecision is the LLM's parsed output. Matches the JSON schema in
// bundled-skills/fbcloak/orchestrate.md.
type PlanDecision struct {
	ShouldSend    bool   `json:"should_send"`
	SendAfterDays int    `json:"send_after_days,omitempty"`
	Message       string `json:"message,omitempty"`
	Reason        string `json:"reason,omitempty"`
	SkipReason    string `json:"skip_reason,omitempty"`
}

// Valid validates the parsed decision against the contract.
func (d PlanDecision) Valid() error {
	if d.ShouldSend {
		if d.SendAfterDays < 1 || d.SendAfterDays > 30 {
			return fmt.Errorf("send_after_days must be 1-30, got %d", d.SendAfterDays)
		}
		if strings.TrimSpace(d.Message) == "" {
			return errors.New("message empty when should_send=true")
		}
		if len([]rune(d.Message)) > 500 {
			return fmt.Errorf("message too long (%d chars, max 500)", len([]rune(d.Message)))
		}
	} else {
		if d.SkipReason == "" {
			return errors.New("skip_reason empty when should_send=false")
		}
	}
	return nil
}

// ScheduleAt returns the absolute UTC time the plan should fire, computed
// from `now + send_after_days + jitter`. Jitter ±15min to prevent thundering
// herd of plans firing exactly on the same hour.
func (d PlanDecision) ScheduleAt(now time.Time, jitter time.Duration) time.Time {
	base := now.Add(time.Duration(d.SendAfterDays) * 24 * time.Hour)
	return base.Add(jitter)
}

// fenceRe strips ```json ... ``` markdown wrapping that some LLMs add
// despite "JSON only" instruction. We tolerate this rather than retry.
var fenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(\\{.*?\\})\\s*```")

// ParseDecision parses LLM raw output. Tolerates markdown fence wrap and
// prose preamble before the JSON body.
func ParseDecision(raw string) (PlanDecision, error) {
	raw = strings.TrimSpace(raw)
	if m := fenceRe.FindStringSubmatch(raw); len(m) == 2 {
		raw = m[1]
	}
	if !strings.HasPrefix(raw, "{") {
		if i := strings.Index(raw, "{"); i >= 0 {
			raw = raw[i:]
		}
	}
	// Trim trailing prose after JSON body
	if i := strings.LastIndex(raw, "}"); i >= 0 && i < len(raw)-1 {
		raw = raw[:i+1]
	}
	var d PlanDecision
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		return PlanDecision{}, fmt.Errorf("parse decision JSON: %w", err)
	}
	if err := d.Valid(); err != nil {
		return PlanDecision{}, fmt.Errorf("invalid decision: %w", err)
	}
	return d, nil
}
