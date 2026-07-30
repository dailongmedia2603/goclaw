//go:build !sqliteonly

package fbcloak

import (
	"time"

	"github.com/google/uuid"
)

// CredentialStatus enumerates the operational state of a stored credential.
type CredentialStatus string

const (
	StatusActive     CredentialStatus = "active"
	StatusExpired    CredentialStatus = "expired"
	StatusCheckpoint CredentialStatus = "checkpoint"
	StatusDisabled   CredentialStatus = "disabled"
)

// Credential is a stored set of cookies + proxy + UA for one fanpage admin
// session. Tenant-scoped. Encrypted at rest (cookies, proxy URL).
type Credential struct {
	ID            uuid.UUID        `db:"id" json:"id"`
	TenantID      uuid.UUID        `db:"tenant_id" json:"tenantId"`
	FanpageID     string           `db:"fanpage_id" json:"fanpageId"`
	FanpageName   string           `db:"fanpage_name" json:"fanpageName"`
	Cookies       string           `db:"-" json:"-"`             // decrypted JSON string in memory only
	CookiesEnc    string           `db:"cookies_enc" json:"-"`   // encrypted column
	ProxyURL      string           `db:"-" json:"-"`             // decrypted in memory only
	ProxyURLEnc   string           `db:"proxy_url_enc" json:"-"` // encrypted column
	UserAgent     string           `db:"user_agent" json:"userAgent"`
	ViewportW     int              `db:"viewport_w" json:"viewportW"`
	ViewportH     int              `db:"viewport_h" json:"viewportH"`
	Timezone      string           `db:"timezone" json:"timezone"`
	Status        CredentialStatus `db:"status" json:"status"`
	LastLoginAt   *time.Time       `db:"last_login_at" json:"lastLoginAt,omitempty"`
	LastCheckAt   *time.Time       `db:"last_check_at" json:"lastCheckAt,omitempty"`
	CreatedAt     time.Time        `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time        `db:"updated_at" json:"updatedAt"`
}

// CreateCredentialInput is the user-supplied data when adding a new credential.
type CreateCredentialInput struct {
	FanpageID   string `json:"fanpageId"`
	FanpageName string `json:"fanpageName"`
	Cookies     string `json:"cookies"`             // raw JSON, will be validated + encrypted
	ProxyURL    string `json:"proxyUrl,omitempty"`
	UserAgent   string `json:"userAgent,omitempty"` // empty → server default
	ViewportW   int    `json:"viewportW,omitempty"`
	ViewportH   int    `json:"viewportH,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
}

// JobStatus is the result of the most recent job run.
type JobStatus string

const (
	JobStatusOK      JobStatus = "ok"
	JobStatusPartial JobStatus = "partial"
	JobStatusFailed  JobStatus = "fail"
	JobStatusKilled  JobStatus = "killed"
)

// WorkingHours describes when a job is allowed to send within local TZ.
type WorkingHours struct {
	Start string `json:"start"` // "HH:MM"
	End   string `json:"end"`   // "HH:MM"
	TZ    string `json:"tz"`
}

// Job is a scheduled re-engagement task tied to one credential.
type Job struct {
	ID                  uuid.UUID    `db:"id" json:"id"`
	TenantID            uuid.UUID    `db:"tenant_id" json:"tenantId"`
	CredentialID        uuid.UUID    `db:"credential_id" json:"credentialId"`
	Name                string       `db:"name" json:"name"`
	TemplateID          *uuid.UUID   `db:"template_id" json:"templateId,omitempty"`
	TargetMinIdle       time.Duration `db:"-" json:"targetMinIdleSec"`
	TargetMaxIdle       time.Duration `db:"-" json:"targetMaxIdleSec"`
	DailyCap            int          `db:"daily_cap" json:"dailyCap"`
	WorkingHours        WorkingHours `db:"-" json:"workingHours"`
	WorkingHoursJSON    string       `db:"working_hours" json:"-"`
	CronExpr            string       `db:"cron_expr" json:"cronExpr"`
	Enabled             bool         `db:"enabled" json:"enabled"`
	DryRun              bool         `db:"dry_run" json:"dryRun"`
	UseScannerFallback  bool         `db:"use_scanner_fallback" json:"useScannerFallback"`
	NextRunAt           *time.Time   `db:"next_run_at" json:"nextRunAt,omitempty"`
	LastRunAt           *time.Time   `db:"last_run_at" json:"lastRunAt,omitempty"`
	LastRunStatus       *JobStatus   `db:"last_run_status" json:"lastRunStatus,omitempty"`
	CreatedAt           time.Time    `db:"created_at" json:"createdAt"`
	UpdatedAt           time.Time    `db:"updated_at" json:"updatedAt"`
}

// SendStatus enumerates outcomes of a single send attempt.
type SendStatus string

const (
	SendStatusSent    SendStatus = "sent"
	SendStatusDryRun  SendStatus = "dry_run"
	SendStatusSkipped SendStatus = "skipped"
	SendStatusFailed  SendStatus = "failed"
)

// SendLog is one row in fbcloak_send_log — the audit trail for every attempt.
type SendLog struct {
	ID             uuid.UUID  `db:"id" json:"id"`
	TenantID       uuid.UUID  `db:"tenant_id" json:"tenantId"`
	JobID          uuid.UUID  `db:"job_id" json:"jobId"`
	CredentialID   uuid.UUID  `db:"credential_id" json:"credentialId"`
	FanpageID      string     `db:"fanpage_id" json:"fanpageId"`
	ConversationID string     `db:"conversation_id" json:"conversationId"`
	RecipientPSID  *string    `db:"recipient_psid" json:"recipientPsid,omitempty"`
	RecipientName  *string    `db:"recipient_name" json:"recipientName,omitempty"`
	LastInboundAt  *time.Time `db:"last_inbound_at" json:"lastInboundAt,omitempty"`
	MessageText    string     `db:"message_text" json:"messageText"`
	Status         SendStatus `db:"status" json:"status"`
	SkipReason     *string    `db:"skip_reason" json:"skipReason,omitempty"`
	Error          *string    `db:"error" json:"error,omitempty"`
	ScreenshotPre  *string    `db:"screenshot_pre" json:"screenshotPre,omitempty"`
	ScreenshotPost *string    `db:"screenshot_post" json:"screenshotPost,omitempty"`
	SentAt         time.Time  `db:"sent_at" json:"sentAt"`
}

// SendLogFilter narrows the result set for ListSendLogFiltered. Zero
// values mean "no filter on that dimension". Status accepts a single
// SendStatus or empty (all). FromDate/ToDate are inclusive bounds on
// sent_at; both zero → unbounded.
type SendLogFilter struct {
	JobID    *uuid.UUID
	Status   SendStatus
	FromDate time.Time
	ToDate   time.Time
	Limit    int // 0 → 100, capped at 500
	Offset   int
}

// ProbeResult summarizes a credential health check (cookie validity + checkpoint state).
type ProbeResult struct {
	OK     bool             `json:"ok"`
	Status CredentialStatus `json:"status"`
	Detail string           `json:"detail,omitempty"`
	UserID string           `json:"userId,omitempty"` // c_user value confirmed by /me redirect
}
