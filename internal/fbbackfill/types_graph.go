package fbbackfill

import (
	"errors"
	"time"
)

// Sentinel errors classifying Graph API failures. Callers (job runner) use
// errors.Is to branch on these rather than parsing strings.
var (
	// ErrAuthExpired indicates the Page Access Token is invalid or revoked
	// (Graph error codes 190, 102, 10). Not retryable — the job must transition
	// to failed and prompt the user to re-connect the channel.
	ErrAuthExpired = errors.New("fbbackfill: page access token expired or invalid")

	// ErrRateLimit indicates the Graph API platform or BUC limit was hit
	// (codes 4, 17, 32, 613, 80001). The job should transition to paused and
	// auto-resume after estimated_time_to_regain_access minutes.
	ErrRateLimit = errors.New("fbbackfill: graph api rate limit")

	// ErrBadRequest indicates a 4xx response that is not auth- or rate-related
	// (e.g., #100 invalid parameter, #200 permission). Not retryable.
	ErrBadRequest = errors.New("fbbackfill: graph api bad request")

	// ErrTransient indicates a 5xx or network error that failed after retries.
	// Callers may retry later; the job runner will mark progress and continue
	// with the next conversation rather than failing the whole job.
	ErrTransient = errors.New("fbbackfill: graph api transient failure")

	// ErrNotFound indicates the requested resource does not exist (404).
	ErrNotFound = errors.New("fbbackfill: graph api resource not found")
)

// ConversationParticipant is one side of a Messenger conversation.
type ConversationParticipant struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Conversation represents one Messenger thread between a Page and a user.
type Conversation struct {
	ID           string    `json:"id"`
	UpdatedTime  time.Time `json:"-"` // parsed from raw string
	UpdatedRaw   string    `json:"updated_time"`
	MessageCount int       `json:"message_count"`
	Participants struct {
		Data []ConversationParticipant `json:"data"`
	} `json:"participants"`
}

// ListConversationsPage is one page of conversations with the cursor for
// the next page (empty = end of list).
type ListConversationsPage struct {
	Data []Conversation `json:"data"`
	Next string         `json:"-"` // extracted from paging.cursors.after
}

// GraphPaging is the canonical paging block returned by Graph API edges
// that support cursor-based pagination.
type GraphPaging struct {
	Cursors struct {
		After  string `json:"after,omitempty"`
		Before string `json:"before,omitempty"`
	} `json:"cursors"`
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
}

type rawConversationsResponse struct {
	Data   []Conversation `json:"data"`
	Paging GraphPaging    `json:"paging"`
}

// Attachment describes a non-text part of a message. Backfill does not
// download attachments; it only records type/mime/name so the summary can
// mention "sent 3 images" etc.
type Attachment struct {
	ID       string `json:"id,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Name     string `json:"name,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// Message represents one Messenger message within a conversation.
type Message struct {
	ID          string    `json:"id"`
	CreatedTime time.Time `json:"-"` // parsed from raw string
	CreatedRaw  string    `json:"created_time"`
	From        ConversationParticipant `json:"from"`
	To          struct {
		Data []ConversationParticipant `json:"data"`
	} `json:"to,omitempty"`
	Message     string `json:"message,omitempty"`
	Attachments struct {
		Data []Attachment `json:"data"`
	} `json:"attachments,omitempty"`
}

// ListMessagesPage is one page of messages with a cursor for the next page.
type ListMessagesPage struct {
	Data []Message `json:"data"`
	Next string    `json:"-"`
}

type rawMessagesResponse struct {
	Data   []Message   `json:"data"`
	Paging GraphPaging `json:"paging"`
}

// graphAPIError is the envelope returned by Graph on 4xx/5xx.
type graphAPIError struct {
	Error struct {
		Message      string `json:"message"`
		Type         string `json:"type"`
		Code         int    `json:"code"`
		ErrorSubcode int    `json:"error_subcode,omitempty"`
		FBTraceID    string `json:"fbtrace_id,omitempty"`
	} `json:"error"`
}

// parseGraphTime parses the ISO 8601 timestamps returned by the Graph API,
// tolerating both "2006-01-02T15:04:05+0000" and "2006-01-02T15:04:05Z".
func parseGraphTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// RFC3339 handles the Z form and colon-separated offsets; the no-colon
	// +0000 form needs the fallback layout.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05-0700", s); err == nil {
		return t
	}
	return time.Time{}
}
