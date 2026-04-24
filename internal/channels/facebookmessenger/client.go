package facebookmessenger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// sidecarDefaultTimeout is the HTTP per-request timeout for calls to the sidecar.
// Long enough to accommodate media upload round-trips, short enough that we
// don't hang the outbound dispatcher indefinitely.
const sidecarDefaultTimeout = 30 * time.Second

// sidecarClient is the minimal HTTP client used to talk to the sidecar shim.
// All methods accept a context so upstream cancellation propagates.
type sidecarClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newSidecarClient(baseURL, token string) *sidecarClient {
	return &sidecarClient{
		baseURL: baseURL,
		token:   token,
		http: &http.Client{
			Timeout: sidecarDefaultTimeout,
			// Default transport with its connection pool is fine — no tuning needed at this scale.
		},
	}
}

// sendRequest is the body GoClaw POSTs to sidecar /send.
type sendRequest struct {
	ChatID  string        `json:"chat_id"`
	Content string        `json:"content,omitempty"`
	Media   []mediaUpload `json:"media,omitempty"`
	ReplyTo string        `json:"reply_to,omitempty"`
}

type mediaUpload struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	Caption     string `json:"caption,omitempty"`
}

type sendResponse struct {
	MessageID string `json:"message_id"`
	Timestamp int64  `json:"timestamp"`
}

// Send posts a message to the sidecar. Returns the remote message ID on success.
func (c *sidecarClient) Send(ctx context.Context, req sendRequest) (*sendResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal send request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/send", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrSidecarTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrSidecarUnreachable, err)
	}
	defer resp.Body.Close()

	// Cap response size to avoid OOM from a malicious/faulty sidecar.
	rawBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status=%d body=%s", ErrSidecarBadStatus, resp.StatusCode, string(rawBody))
	}

	var out sendResponse
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return nil, fmt.Errorf("decode send response: %w", err)
	}
	return &out, nil
}

// Health pings the sidecar's /healthz endpoint. Cheap; safe to call on a ticker.
func (c *sidecarClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrSidecarTimeout
		}
		return fmt.Errorf("%w: %v", ErrSidecarUnreachable, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: status=%d", ErrSidecarBadStatus, resp.StatusCode)
	}
	return nil
}
