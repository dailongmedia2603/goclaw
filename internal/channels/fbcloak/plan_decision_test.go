//go:build !sqliteonly

package fbcloak

import (
	"strings"
	"testing"
	"time"
)

func TestParseDecision_PlainJSON(t *testing.T) {
	in := `{"should_send":true,"send_after_days":7,"message":"hi","reason":"test"}`
	d, err := ParseDecision(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !d.ShouldSend || d.SendAfterDays != 7 || d.Message != "hi" {
		t.Errorf("got %+v", d)
	}
}

func TestParseDecision_MarkdownFence(t *testing.T) {
	in := "```json\n{\"should_send\":true,\"send_after_days\":3,\"message\":\"hi\",\"reason\":\"x\"}\n```"
	d, err := ParseDecision(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !d.ShouldSend || d.SendAfterDays != 3 {
		t.Errorf("got %+v", d)
	}
}

func TestParseDecision_PreambleProse(t *testing.T) {
	in := "Sure, here's my decision:\n\n{\"should_send\":false,\"skip_reason\":\"too_recent\",\"reason\":\"khach moi nhan\"}"
	d, err := ParseDecision(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if d.ShouldSend || d.SkipReason != "too_recent" {
		t.Errorf("got %+v", d)
	}
}

func TestParseDecision_TrailingProse(t *testing.T) {
	in := `{"should_send":true,"send_after_days":5,"message":"hi","reason":"x"} -- end`
	d, err := ParseDecision(in)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !d.ShouldSend {
		t.Errorf("got %+v", d)
	}
}

func TestParseDecision_Malformed(t *testing.T) {
	cases := []string{
		``,
		`not json`,
		`{`,
		`{"should_send":"yes"}`, // wrong type
	}
	for _, in := range cases {
		if _, err := ParseDecision(in); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestPlanDecision_Valid_SendOK(t *testing.T) {
	d := PlanDecision{ShouldSend: true, SendAfterDays: 7, Message: "hi", Reason: "x"}
	if err := d.Valid(); err != nil {
		t.Errorf("valid send: %v", err)
	}
}

func TestPlanDecision_Valid_SendMissingMessage(t *testing.T) {
	d := PlanDecision{ShouldSend: true, SendAfterDays: 7, Reason: "x"}
	if err := d.Valid(); err == nil {
		t.Error("expected error for empty message")
	}
}

func TestPlanDecision_Valid_SendBadDays(t *testing.T) {
	for _, days := range []int{0, -1, 31, 100} {
		d := PlanDecision{ShouldSend: true, SendAfterDays: days, Message: "hi"}
		if err := d.Valid(); err == nil {
			t.Errorf("expected error for days=%d", days)
		}
	}
}

func TestPlanDecision_Valid_SendTooLong(t *testing.T) {
	d := PlanDecision{ShouldSend: true, SendAfterDays: 7, Message: strings.Repeat("a", 600)}
	if err := d.Valid(); err == nil {
		t.Error("expected error for >500 chars")
	}
}

func TestPlanDecision_Valid_SkipMissingReason(t *testing.T) {
	d := PlanDecision{ShouldSend: false}
	if err := d.Valid(); err == nil {
		t.Error("expected error for missing skip_reason")
	}
}

func TestPlanDecision_ScheduleAt(t *testing.T) {
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	d := PlanDecision{SendAfterDays: 7}
	got := d.ScheduleAt(now, 0)
	want := now.Add(7 * 24 * time.Hour)
	if !got.Equal(want) {
		t.Errorf("ScheduleAt = %v, want %v", got, want)
	}
	got = d.ScheduleAt(now, time.Minute*10)
	if got.Sub(want) != 10*time.Minute {
		t.Errorf("jitter not applied: diff = %v", got.Sub(want))
	}
}
