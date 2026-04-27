//go:build !sqliteonly

package fbcloak

import "errors"

var (
	// ErrPlanNotFound — Get / GetActiveForRecipient miss.
	ErrPlanNotFound = errors.New("fbcloak: plan not found")

	// ErrActiveConflict — Create attempted while another active plan exists for same (credential, psid).
	ErrActiveConflict = errors.New("fbcloak: active plan already exists for this recipient")

	// ErrPlanTerminal — caller tried to MarkSent/MarkSuperseded on a row already in terminal status.
	ErrPlanTerminal = errors.New("fbcloak: plan in terminal status, transition not allowed")

	// ErrInvalidTenant — tenantID required but uuid.Nil supplied.
	ErrInvalidTenant = errors.New("fbcloak: tenantID required")

	// ErrScheduleTooFar — Create rejected because scheduled_at > now+90d.
	ErrScheduleTooFar = errors.New("fbcloak: scheduled_at > 90 days, rejected")
)
