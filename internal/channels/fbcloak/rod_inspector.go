//go:build !sqliteonly

package fbcloak

import (
	"context"
	"errors"
	"strings"

	"github.com/go-rod/rod"
)

// rodPageInspector adapts a *rod.Page to the PageInspector interface used
// by checkpoint_detector. Methods are best-effort: a panic from rod
// surfaces as an error rather than crashing the runner.
type rodPageInspector struct {
	page *rod.Page
}

// NewRodPageInspector returns a PageInspector backed by the given rod page.
// nil page yields an inspector whose methods all return errors — useful in
// degraded paths where the caller wants the detector to no-op gracefully.
func NewRodPageInspector(page *rod.Page) PageInspector {
	return &rodPageInspector{page: page}
}

func (r *rodPageInspector) URL(_ context.Context) (string, error) {
	if r.page == nil {
		return "", errors.New("rodPageInspector: nil page")
	}
	info, err := r.page.Info()
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

func (r *rodPageInspector) Title(_ context.Context) (string, error) {
	if r.page == nil {
		return "", errors.New("rodPageInspector: nil page")
	}
	info, err := r.page.Info()
	if err != nil {
		return "", err
	}
	return info.Title, nil
}

func (r *rodPageInspector) HTML(_ context.Context) (string, error) {
	if r.page == nil {
		return "", errors.New("rodPageInspector: nil page")
	}
	html, err := r.page.HTML()
	if err != nil {
		return "", err
	}
	return html, nil
}

// rodThreadInspector adapts a *rod.Page to the ThreadInspector contract.
// Extracts last-message metadata from a couple of known DOM hooks; falls
// back to parsing visible text when the structured hooks fail. Returning
// empty strings is acceptable — VerifyLastMessage will treat that as
// "parse_failed" and skip without an error.
type rodThreadInspector struct {
	page *rod.Page
}

// NewRodThreadInspector returns a ThreadInspector backed by the given page.
func NewRodThreadInspector(page *rod.Page) ThreadInspector {
	return &rodThreadInspector{page: page}
}

func (r *rodThreadInspector) LastMessageMarkers(ctx context.Context) (axName, reactDump, rawText string, err error) {
	if r.page == nil {
		return "", "", "", errors.New("rodThreadInspector: nil page")
	}
	if cErr := ctx.Err(); cErr != nil {
		return "", "", "", cErr
	}
	// 1) AX node name — last visible row in the conversation list. Meta
	//    Business Inbox renders the chat list as a `[role="grid"]` with
	//    `[role="row"]` items; the most recent message bubble usually
	//    has an `aria-label` with the timestamp.
	axName = r.evalString(`
		(() => {
			const rows = document.querySelectorAll('[role="row"], [data-testid="message-bubble"], [aria-label]');
			let last = null;
			rows.forEach(el => { last = el; });
			if (!last) return '';
			return last.getAttribute('aria-label') || last.innerText || '';
		})()
	`)
	// 2) raw text near the bottom — fallback when AX label is missing.
	rawText = r.evalString(`
		(() => {
			const sel = '[role="main"], [aria-label*="Messenger"], [aria-label*="messages"], main';
			const root = document.querySelector(sel) || document.body;
			const text = (root.innerText || '').trim();
			const lines = text.split('\n').filter(l => l.trim().length > 0);
			return lines.slice(-12).join('\n');
		})()
	`)
	// 3) reactDump intentionally left empty — Meta scrambles React
	//    internals; trying to read them yields false positives more
	//    often than it helps. parse_timestamp.go already handles
	//    text-based timestamps.
	return axName, "", rawText, nil
}

// evalString runs a JS expression and returns the string result, or "" on
// any error. Centralised so each marker call is a one-liner.
func (r *rodThreadInspector) evalString(js string) string {
	res, err := r.page.Eval(js)
	if err != nil || res == nil {
		return ""
	}
	v := res.Value.String()
	return strings.TrimSpace(v)
}
