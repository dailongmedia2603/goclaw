//go:build !sqliteonly

package fbcloak

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPlanStatus_IsTerminal(t *testing.T) {
	cases := map[PlanStatus]bool{
		PlanStatusPending:      false,
		PlanStatusSent:         true,
		PlanStatusSuperseded:   true,
		PlanStatusCancelled:    true,
		PlanStatusReplanNeeded: false,
		PlanStatusSkipped:      true,
	}
	for s, want := range cases {
		if got := s.IsTerminal(); got != want {
			t.Errorf("%s.IsTerminal() = %v, want %v", s, got, want)
		}
	}
}

func TestPlanStatus_IsActive(t *testing.T) {
	cases := map[PlanStatus]bool{
		PlanStatusPending:      true,
		PlanStatusSent:         false,
		PlanStatusSuperseded:   false,
		PlanStatusCancelled:    false,
		PlanStatusReplanNeeded: true,
		PlanStatusSkipped:      false,
	}
	for s, want := range cases {
		if got := s.IsActive(); got != want {
			t.Errorf("%s.IsActive() = %v, want %v", s, got, want)
		}
	}
}

func TestPlan_JSONRoundtrip(t *testing.T) {
	now := time.Now().UTC().Round(time.Second)
	sentAt := now.Add(time.Hour)
	logID := uuid.New()
	plan := Plan{
		ID:               uuid.New(),
		TenantID:         uuid.New(),
		CredentialID:     uuid.New(),
		PSID:             "100012345",
		Status:           PlanStatusSent,
		ScheduledAt:      now,
		MessageDraft:     "hi",
		Reason:           "test",
		GeneratedByModel: "claude-haiku-4-5",
		GeneratedAt:      now,
		SummaryVersion:   2,
		SentAt:           &sentAt,
		SendLogID:        &logID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	b, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Plan
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Status != PlanStatusSent || decoded.PSID != "100012345" {
		t.Fatalf("roundtrip mismatch: %+v", decoded)
	}
	if decoded.SentAt == nil || !decoded.SentAt.Equal(sentAt) {
		t.Errorf("SentAt lost in roundtrip")
	}
	if decoded.SendLogID == nil || *decoded.SendLogID != logID {
		t.Errorf("SendLogID lost in roundtrip")
	}
}
