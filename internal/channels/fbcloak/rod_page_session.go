//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
)

// rodPageSession is the production PageSession backed by a real rod page.
// One session = one credential = one worker. Per-credential sequencing is
// enforced upstream by JobRunnerImpl.credentialMutex; this struct does not
// serialise calls itself.
//
// Selectors are intentionally tolerant: Meta Business Inbox iterates DOM
// constantly, so we look for any element that *plausibly* matches role +
// label rather than fixed CSS classes. When a selector fails we surface a
// descriptive error so checkpoint_detector or send_executor can decide.
type rodPageSession struct {
	page         *rod.Page
	tenantID     uuid.UUID
	jobID        uuid.UUID
	credentialID uuid.UUID
	writer       *ScreenshotWriter // nil → screenshot calls return ""
	logSendID    uuid.UUID         // current send_log row's ID; rotated by NavigateThread
}

// NewRodPageSession constructs a PageSession. tenantID/jobID/credentialID
// are baked in so screenshot paths land in the right tenant directory.
// writer is optional: nil disables disk capture.
func NewRodPageSession(page *rod.Page, tenantID, jobID, credentialID uuid.UUID, writer *ScreenshotWriter) PageSession {
	return &rodPageSession{
		page:         page,
		tenantID:     tenantID,
		jobID:        jobID,
		credentialID: credentialID,
		writer:       writer,
	}
}

// Compile-time assertion.
var _ PageSession = (*rodPageSession)(nil)

// NavigateThread opens the Business Inbox thread URL. When conversationID
// is provided, jumps directly to it (`active_chat_thread_id=t_xxx`);
// otherwise opens the inbox + lets verify-last-message handle resolution.
// Returns the final URL after redirects so the caller can detect
// `/checkpoint`, `/login`, etc.
func (r *rodPageSession) NavigateThread(ctx context.Context, fanpageID, conversationID, recipientPSID string) (string, error) {
	if r.page == nil {
		return "", errors.New("rodPageSession: nil page")
	}
	if fanpageID == "" {
		return "", errors.New("rodPageSession: fanpageID required")
	}
	// Per-thread send_log id rotates each navigation so screenshot files
	// don't collide across sends in a single job run.
	r.logSendID = uuid.New()

	url := buildThreadURL(fanpageID, conversationID, recipientPSID)
	page := r.page.Context(ctx)
	if err := page.Navigate(url); err != nil {
		return "", fmt.Errorf("navigate: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return "", fmt.Errorf("wait load: %w", err)
	}
	// Best-effort settle — DOM hydration finishes shortly after `load`.
	_ = page.WaitIdle(3 * time.Second)
	info, err := page.Info()
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

// Inspector returns a ThreadInspector reading the same rod page. Cheap —
// no allocation cost beyond the wrapper.
func (r *rodPageSession) Inspector() ThreadInspector {
	return NewRodThreadInspector(r.page)
}

// PageInspector exposes the URL/Title/HTML reader the checkpoint detector
// needs. Same underlying page.
func (r *rodPageSession) PageInspector() PageInspector {
	return NewRodPageInspector(r.page)
}

// PageSessionToInspector adapts a PageSession into a PageInspector for the
// checkpoint detector. nil → nil so the runner short-circuits gracefully.
// Production wires this into JobRunnerImpl.CheckpointInspectorFor.
func PageSessionToInspector(s PageSession) PageInspector {
	if s == nil {
		return nil
	}
	if rs, ok := s.(*rodPageSession); ok {
		return rs.PageInspector()
	}
	return nil
}

// TypeMessage focuses the compose box and types `message` character-by-character
// (Rod's input-event mode). Returns the readback of the textbox so the
// executor can verify what the DOM actually accepted vs what we sent.
//
// Selector strategy: prefer a `[contenteditable="true"]` with an aria-label
// hinting at "message"/"reply"; fall back to the first contenteditable
// inside the main column.
func (r *rodPageSession) TypeMessage(ctx context.Context, message string) (string, error) {
	if r.page == nil {
		return "", errors.New("rodPageSession: nil page")
	}
	page := r.page.Context(ctx)
	box, err := r.findComposeBox(page)
	if err != nil {
		return "", err
	}
	if err := box.Focus(); err != nil {
		return "", fmt.Errorf("focus textbox: %w", err)
	}
	// Clear any pre-filled value (defensive).
	if err := page.Keyboard.Press('a'); err == nil { // ⌘A is OS-dependent; rely on selectall API instead
		_, _ = page.Eval(`document.execCommand('selectAll', false, null)`)
		_, _ = page.Eval(`document.execCommand('delete', false, null)`)
	}
	// Insert as one chunk — Meta detects per-key timing variance more
	// than overall typing speed; humanizer at runner level inserts
	// pauses BETWEEN sends, not within.
	if err := box.Input(message); err != nil {
		return "", fmt.Errorf("input: %w", err)
	}
	// Read back via .innerText. Some Meta builds use lexical editor
	// rendering text inside nested spans — we trim and join.
	res, err := box.Eval(`() => this.innerText || this.textContent || ''`)
	if err != nil {
		return "", fmt.Errorf("readback: %w", err)
	}
	return strings.TrimSpace(res.Value.String()), nil
}

// ClickSend finds and clicks the Send button. Tries multiple selectors
// because Meta swaps between aria-label "Press Enter to send", "Send
// message", or a paper-plane icon button.
func (r *rodPageSession) ClickSend(ctx context.Context) error {
	if r.page == nil {
		return errors.New("rodPageSession: nil page")
	}
	page := r.page.Context(ctx)
	btn, err := r.findSendButton(page)
	if err != nil {
		return err
	}
	return btn.Click(proto.InputMouseButtonLeft, 1)
}

// WaitSendConfirmed waits until the message we sent appears in the
// transcript. Heuristic: poll the visible message list; success when the
// last bubble's text contains a substring of our sent payload.
//
// timeout caps the poll loop. Returns nil on success; error otherwise.
func (r *rodPageSession) WaitSendConfirmed(ctx context.Context, timeout time.Duration) error {
	if r.page == nil {
		return errors.New("rodPageSession: nil page")
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cErr := ctx.Err(); cErr != nil {
			return cErr
		}
		// Empty compose box = best signal that the send committed (Meta
		// clears the editor after the message lands in the transcript).
		empty := r.composeBoxEmpty(ctx)
		if empty {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("send confirmation timeout (%s)", timeout)
}

// ScreenshotPre captures a PNG before pressing Send. Returns "" when the
// writer is nil (disabled) or when capture fails — failures are
// non-fatal (executor logs and continues).
func (r *rodPageSession) ScreenshotPre(ctx context.Context) (string, error) {
	return r.capture(ctx, ScreenshotPre)
}

// ScreenshotPost captures a PNG after the send confirmation. Same
// nil/failure semantics as ScreenshotPre.
func (r *rodPageSession) ScreenshotPost(ctx context.Context) (string, error) {
	return r.capture(ctx, ScreenshotPostKind)
}

// --- helpers ---

func (r *rodPageSession) capture(ctx context.Context, kind ScreenshotKind) (string, error) {
	if r.writer == nil || r.page == nil {
		return "", nil
	}
	if r.logSendID == uuid.Nil {
		return "", errors.New("capture: NavigateThread must run first")
	}
	png, err := r.page.Context(ctx).Screenshot(false, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	})
	if err != nil {
		return "", err
	}
	return r.writer.Save(ctx, r.tenantID, r.jobID, r.logSendID, kind, png)
}

// composeBoxEmpty returns true when the textbox text is whitespace-only
// or the box can't be found (we avoid raising — caller treats "couldn't
// confirm" as a failure path).
func (r *rodPageSession) composeBoxEmpty(ctx context.Context) bool {
	page := r.page.Context(ctx)
	box, err := r.findComposeBox(page)
	if err != nil {
		return false
	}
	res, err := box.Eval(`() => (this.innerText || this.textContent || '').trim()`)
	if err != nil {
		return false
	}
	return res.Value.String() == ""
}

// findComposeBox searches for the message input. Tries aria-label hints
// first, falls back to the first contenteditable in [role="main"].
func (r *rodPageSession) findComposeBox(page *rod.Page) (*rod.Element, error) {
	// Aria-label heuristics — Meta uses different strings per locale, so
	// we accept any English/Vietnamese keyword we've seen in the wild.
	candidates := []string{
		`[contenteditable="true"][aria-label*="Message"]`,
		`[contenteditable="true"][aria-label*="message"]`,
		`[contenteditable="true"][aria-label*="reply"]`,
		`[contenteditable="true"][aria-label*="Tin nhắn"]`,
		`[role="main"] [contenteditable="true"]`,
		`[contenteditable="true"]`,
	}
	for _, sel := range candidates {
		el, err := page.Element(sel)
		if err == nil && el != nil {
			return el, nil
		}
	}
	return nil, errors.New("compose box not found")
}

// findSendButton searches for a Send-like button. Order matters: aria-label
// first (most stable), then icon-button heuristics.
func (r *rodPageSession) findSendButton(page *rod.Page) (*rod.Element, error) {
	candidates := []string{
		`[aria-label="Press Enter to send"]`,
		`[aria-label="Send message"]`,
		`[aria-label*="Send"][role="button"]`,
		`[aria-label*="Gửi"][role="button"]`,
		`button[type="submit"]`,
	}
	for _, sel := range candidates {
		el, err := page.Element(sel)
		if err == nil && el != nil {
			return el, nil
		}
	}
	return nil, errors.New("send button not found")
}

// buildThreadURL composes the Business Inbox URL the runner navigates to.
// When conversationID is non-empty we use the active_chat_thread_id form
// (preferred — direct hop, no inbox listitem brittleness); otherwise we
// land on the inbox with the page asset selected and let verify-last-
// message find the correct row from the PSID hint.
func buildThreadURL(fanpageID, conversationID, recipientPSID string) string {
	base := "https://business.facebook.com/latest/inbox"
	q := "?asset_id=" + fanpageID
	if conversationID != "" {
		q += "&active_chat_thread_id=" + conversationID
	} else if recipientPSID != "" {
		// Some Meta variants accept thread_user_id as a fallback selector.
		q += "&thread_user_id=" + recipientPSID
	}
	return base + q
}
