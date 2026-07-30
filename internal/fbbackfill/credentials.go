package fbbackfill

import (
	"encoding/json"
	"errors"
)

// facebookCreds mirrors the upstream internal/channels/facebook/types.go
// facebookCreds struct. Duplicated here (rather than imported) because
// the upstream type is unexported and the fork-safety contract forbids
// modifying the upstream package.
type facebookCreds struct {
	PageAccessToken string `json:"page_access_token"`
	AppSecret       string `json:"app_secret"`
	VerifyToken     string `json:"verify_token"`
}

// facebookConfigPartial is a subset of the upstream facebookInstanceConfig
// carrying only fields the backfill job needs (PageID) plus any
// fork-specific keys (_backfill). Everything else is ignored on parse.
type facebookConfigPartial struct {
	PageID            string `json:"page_id"`
	BackfillOnCreate  bool   `json:"backfill_on_create,omitempty"`
}

// ErrMissingPageID indicates the channel instance config lacks a page_id,
// which is required for Graph API calls.
var ErrMissingPageID = errors.New("fbbackfill: channel config missing page_id")

// ErrMissingAccessToken indicates the credentials blob lacks
// page_access_token. The channel cannot be used until re-connected.
var ErrMissingAccessToken = errors.New("fbbackfill: credentials missing page_access_token")

// parseCreds decodes an instance's credentials JSON into a facebookCreds.
// Callers receive the decrypted bytes from upstream store.ChannelInstanceStore.
func parseCreds(raw []byte) (*facebookCreds, error) {
	if len(raw) == 0 {
		return nil, ErrMissingAccessToken
	}
	var c facebookCreds
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, errors.New("fbbackfill: parse credentials: " + err.Error())
	}
	if c.PageAccessToken == "" {
		return nil, ErrMissingAccessToken
	}
	return &c, nil
}

// parseConfig decodes an instance's config JSON and extracts the page_id
// plus any fork-relevant fields.
func parseConfig(raw json.RawMessage) (*facebookConfigPartial, error) {
	if len(raw) == 0 {
		return nil, ErrMissingPageID
	}
	var c facebookConfigPartial
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, errors.New("fbbackfill: parse config: " + err.Error())
	}
	if c.PageID == "" {
		return nil, ErrMissingPageID
	}
	return &c, nil
}
