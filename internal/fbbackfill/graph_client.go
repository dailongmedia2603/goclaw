package fbbackfill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Graph API version pinned for this client. Matches the upstream
// internal/channels/facebook/graph_api.go version at the time of writing
// (v25.0). Bump both together when the upstream is updated.
const (
	graphAPIBaseDefault = "https://graph.facebook.com"
	graphAPIVersion     = "v25.0"
)

// BackfillClient is a fork-local HTTP client for the two Graph API edges
// the backfill job needs: /{page-id}/conversations and
// /{conversation-id}/messages. It is not a replacement for upstream
// GraphClient — the runtime webhook channel continues to use that. This
// client exists separately so the fork does not have to extend upstream
// files.
type BackfillClient struct {
	http             *http.Client
	baseURL          string
	apiVersion       string
	pageAccessToken  string
	pageID           string
	bucTracker       *bucTracker
	maxRetries       int
	maxRetryAfterSec int
	perCallTimeout   time.Duration
	clock            func() time.Time
	sleep            func(time.Duration)
}

// Option configures a BackfillClient at construction time.
type Option func(*BackfillClient)

// WithBaseURL overrides the Graph API base URL. Used in tests to point at
// a local httptest.Server.
func WithBaseURL(u string) Option { return func(c *BackfillClient) { c.baseURL = strings.TrimRight(u, "/") } }

// WithHTTPClient swaps the underlying http.Client (useful for timeouts
// and transport customization).
func WithHTTPClient(h *http.Client) Option { return func(c *BackfillClient) { c.http = h } }

// WithMaxRetries sets the per-request retry cap. Default 3.
func WithMaxRetries(n int) Option { return func(c *BackfillClient) { c.maxRetries = n } }

// WithPerCallTimeout sets the deadline for a single HTTP round trip.
// Default 30s. The retry loop multiplies this by MaxRetries for the total.
func WithPerCallTimeout(d time.Duration) Option { return func(c *BackfillClient) { c.perCallTimeout = d } }

// WithClock overrides time.Now (used in tests for deterministic backoff).
func WithClock(now func() time.Time) Option { return func(c *BackfillClient) { c.clock = now } }

// WithSleep overrides time.Sleep (used in tests to skip backoff waits).
func WithSleep(s func(time.Duration)) Option { return func(c *BackfillClient) { c.sleep = s } }

// NewBackfillClient constructs a client for a specific Page. The token
// is held in memory for the lifetime of the client — callers must not
// log it. One client per Page because each has its own BUC tracker state.
func NewBackfillClient(pageAccessToken, pageID string, opts ...Option) *BackfillClient {
	c := &BackfillClient{
		http:             &http.Client{Timeout: 60 * time.Second},
		baseURL:          graphAPIBaseDefault,
		apiVersion:       graphAPIVersion,
		pageAccessToken:  pageAccessToken,
		pageID:           pageID,
		bucTracker:       &bucTracker{},
		maxRetries:       3,
		maxRetryAfterSec: 120,
		perCallTimeout:   30 * time.Second,
		clock:            time.Now,
		sleep:            time.Sleep,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// BUCTracker exposes the rate-limit tracker so the job runner can make
// pause/resume decisions based on the most recent reading.
func (c *BackfillClient) BUCTracker() *bucTracker { return c.bucTracker }

// ListConversations fetches one page of Messenger conversations for the
// Page. Pass an empty cursor to start; the returned Next is the cursor
// for the next page (empty = end of list).
func (c *BackfillClient) ListConversations(ctx context.Context, cursor string) (*ListConversationsPage, error) {
	q := url.Values{}
	q.Set("fields", "updated_time,message_count,participants")
	q.Set("limit", "100")
	q.Set("platform", "MESSENGER")
	if cursor != "" {
		q.Set("after", cursor)
	}
	path := fmt.Sprintf("/%s/conversations", c.pageID)
	body, err := c.doRequest(ctx, path, q)
	if err != nil {
		return nil, err
	}
	var raw rawConversationsResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("fbbackfill: decode conversations: %w", err)
	}
	for i := range raw.Data {
		raw.Data[i].UpdatedTime = parseGraphTime(raw.Data[i].UpdatedRaw)
	}
	return &ListConversationsPage{
		Data: raw.Data,
		Next: raw.Paging.Cursors.After,
	}, nil
}

// ListMessages fetches one page of messages from a conversation. Messages
// are returned in reverse chronological order by the Graph API — callers
// should reverse the slice if they need chronological order.
func (c *BackfillClient) ListMessages(ctx context.Context, conversationID, cursor string) (*ListMessagesPage, error) {
	if conversationID == "" {
		return nil, fmt.Errorf("fbbackfill: list_messages: empty conversation id")
	}
	q := url.Values{}
	q.Set("fields", "id,message,from,to,created_time,attachments")
	q.Set("limit", "100")
	if cursor != "" {
		q.Set("after", cursor)
	}
	path := "/" + conversationID + "/messages"
	body, err := c.doRequest(ctx, path, q)
	if err != nil {
		return nil, err
	}
	var raw rawMessagesResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("fbbackfill: decode messages: %w", err)
	}
	for i := range raw.Data {
		raw.Data[i].CreatedTime = parseGraphTime(raw.Data[i].CreatedRaw)
	}
	return &ListMessagesPage{
		Data: raw.Data,
		Next: raw.Paging.Cursors.After,
	}, nil
}

// doRequest builds the URL, executes the request, applies pacing based on
// BUC state, retries on transient and rate-limit errors, and maps 4xx
// responses to sentinel errors.
func (c *BackfillClient) doRequest(ctx context.Context, path string, q url.Values) ([]byte, error) {
	// Respect pacing advice from the last BUC reading before making any call.
	if pause := c.bucTracker.ShouldPauseFor(); pause > 0 {
		c.sleep(pause)
	}

	// Token is injected as a query param (Graph API convention); do not log the URL with token.
	q.Set("access_token", c.pageAccessToken)
	reqURL := c.baseURL + "/" + c.apiVersion + path + "?" + q.Encode()

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		callCtx, cancel := context.WithTimeout(ctx, c.perCallTimeout)
		req, err := http.NewRequestWithContext(callCtx, http.MethodGet, reqURL, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("fbbackfill: build request: %w", err)
		}
		start := c.clock()
		resp, err := c.http.Do(req)
		callDur := time.Since(start)
		if err != nil {
			cancel()
			// network failure — classify as transient and retry
			lastErr = err
			slog.Debug("fb_backfill.client.call_err", "attempt", attempt, "err", err, "path", path)
			if attempt < c.maxRetries {
				c.sleep(backoff(attempt))
				continue
			}
			return nil, fmt.Errorf("%w: %v", ErrTransient, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if readErr != nil {
			lastErr = readErr
			if attempt < c.maxRetries {
				c.sleep(backoff(attempt))
				continue
			}
			return nil, fmt.Errorf("%w: read body: %v", ErrTransient, readErr)
		}

		// Update BUC tracker regardless of status — the header is always set.
		c.bucTracker.ParseHeader(resp.Header.Get("X-Business-Use-Case-Usage"))

		slog.Debug("fb_backfill.client.call",
			"path", path, "status", resp.StatusCode,
			"duration_ms", callDur.Milliseconds(),
			"buc_pct", c.bucTracker.PeakPercent())

		if resp.StatusCode == http.StatusOK {
			return body, nil
		}

		// Parse Graph error envelope.
		var ge graphAPIError
		_ = json.Unmarshal(body, &ge)
		code := ge.Error.Code

		switch {
		case isAuthCode(code):
			slog.Warn("fb_backfill.client.auth_expired",
				"fb_error_code", code, "fb_error_msg", ge.Error.Message)
			return nil, fmt.Errorf("%w: code=%d: %s", ErrAuthExpired, code, ge.Error.Message)

		case isRateLimitCode(code) || resp.StatusCode == http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), c.maxRetryAfterSec)
			if retryAfter == 0 && c.bucTracker.IsSaturated() {
				// Saturated BUC + no Retry-After → bubble up so job runner can pause.
				return nil, fmt.Errorf("%w: code=%d saturated", ErrRateLimit, code)
			}
			if attempt < c.maxRetries {
				slog.Warn("fb_backfill.client.rate_limit_retry",
					"code", code, "retry_after_sec", int(retryAfter/time.Second), "attempt", attempt)
				c.sleep(retryAfter)
				continue
			}
			return nil, fmt.Errorf("%w: code=%d: %s", ErrRateLimit, code, ge.Error.Message)

		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("%w: %s", ErrNotFound, ge.Error.Message)

		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			// Non-auth, non-rate-limit 4xx — not retryable.
			return nil, fmt.Errorf("%w: code=%d status=%d: %s",
				ErrBadRequest, code, resp.StatusCode, ge.Error.Message)

		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("%w: status=%d code=%d", ErrTransient, resp.StatusCode, code)
			if attempt < c.maxRetries {
				c.sleep(backoff(attempt))
				continue
			}
			return nil, lastErr

		default:
			// Unknown — treat as transient.
			lastErr = fmt.Errorf("%w: unexpected status=%d", ErrTransient, resp.StatusCode)
			if attempt < c.maxRetries {
				c.sleep(backoff(attempt))
				continue
			}
			return nil, lastErr
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("fbbackfill: retries exhausted")
}

// backoff returns the sleep duration before the given retry attempt.
// Exponential: 1s, 2s, 4s, capped at 30s.
func backoff(attempt int) time.Duration {
	switch attempt {
	case 0:
		return time.Second
	case 1:
		return 2 * time.Second
	case 2:
		return 4 * time.Second
	default:
		return 30 * time.Second
	}
}

// parseRetryAfter parses the Retry-After header (seconds form) with a cap.
// Returns 0 if header is empty or unparseable.
func parseRetryAfter(v string, capSec int) time.Duration {
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0
	}
	if capSec > 0 && n > capSec {
		n = capSec
	}
	return time.Duration(n) * time.Second
}

// isAuthCode returns true for Graph error codes that indicate an expired
// or invalid access token. Non-retryable.
func isAuthCode(code int) bool {
	switch code {
	case 10, 102, 190, 200, 458, 459, 460, 463, 464, 467:
		return true
	}
	return false
}

// isRateLimitCode returns true for Graph error codes that indicate the
// caller should back off and retry.
func isRateLimitCode(code int) bool {
	switch code {
	case 4, 17, 32, 613:
		return true
	case 80001, 80002, 80003, 80004, 80005, 80006, 80008, 80014:
		// BUC-specific codes (pages=80001).
		return true
	}
	return false
}
