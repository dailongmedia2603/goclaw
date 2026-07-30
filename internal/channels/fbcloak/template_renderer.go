//go:build !sqliteonly

package fbcloak

import (
	"context"
	"strings"
)

// SimpleTemplateRenderer is a placeholder-based renderer that does NOT call
// an LLM. Phase 4 will swap in a skill-aware renderer that may optionally
// invoke a provider for {last_topic}-style enrichment.
//
// Supported placeholders:
//   - {customer_name}   → Target.RecipientName ("bạn" if empty)
//   - {fanpage_name}    → Credential.FanpageName
//   - {greeting}        → time-of-day appropriate greeting
type SimpleTemplateRenderer struct {
	Body            string // template literal (e.g. "Chào {customer_name}, {fanpage_name} chào bạn quay lại!")
	DefaultCustomer string // used when Target.RecipientName is empty (default "bạn")
}

// Compile-time guard.
var _ TemplateRenderer = (*SimpleTemplateRenderer)(nil)

func (r *SimpleTemplateRenderer) Render(_ context.Context, _ Job, t Target, c Credential) (string, error) {
	customer := t.RecipientName
	if customer == "" {
		customer = r.DefaultCustomer
		if customer == "" {
			customer = "bạn"
		}
	}
	out := r.Body
	out = strings.ReplaceAll(out, "{customer_name}", customer)
	out = strings.ReplaceAll(out, "{fanpage_name}", c.FanpageName)
	return strings.TrimSpace(out), nil
}
