package fbbackfill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// LLMClient is the narrow slice of providers.Provider that summarizer_impl
// needs. Exposed as an interface so tests can swap a deterministic fake
// without wiring an actual provider registry.
type LLMClient interface {
	Chat(ctx context.Context, req providers.ChatRequest) (*providers.ChatResponse, error)
	DefaultModel() string
}

// LLMResolver produces an LLM client for a tenant. A nil return means
// "LLM unavailable" and the summarizer will fall back to the concat path.
type LLMResolver interface {
	Resolve(ctx context.Context, tenantID uuid.UUID) (LLMClient, string)
}

// noopLLMResolver always returns (nil, "") — forces concat path. Used as
// safe default when the wiring layer does not supply a resolver.
type noopLLMResolver struct{}

func (noopLLMResolver) Resolve(_ context.Context, _ uuid.UUID) (LLMClient, string) { return nil, "" }

// NewNoopLLMResolver returns a resolver that disables LLM summarization
// entirely. Useful for environments without provider wiring or for tests.
func NewNoopLLMResolver() LLMResolver { return noopLLMResolver{} }

// SummarizerConfig parameterizes the summarizer.
type SummarizerConfig struct {
	// ShortPathThreshold — conversations with this many or fewer messages
	// use the concat path (no LLM call). Default 20.
	ShortPathThreshold int

	// MaxTranscriptMessages — cap on messages sent to the LLM to bound
	// input token cost. Default 200 (keeps ~8k tokens typical).
	MaxTranscriptMessages int

	// EpisodicExpiry — retention for backfilled episodic summaries.
	// Default 180 days. Longer than the regular 90d default because
	// historical context is useful for longer.
	EpisodicExpiry time.Duration
}

func (c *SummarizerConfig) defaults() {
	if c.ShortPathThreshold <= 0 {
		c.ShortPathThreshold = 20
	}
	if c.MaxTranscriptMessages <= 0 {
		c.MaxTranscriptMessages = 200
	}
	if c.EpisodicExpiry <= 0 {
		c.EpisodicExpiry = 180 * 24 * time.Hour
	}
}

// summarizerImpl is the production Summarizer.
type summarizerImpl struct {
	episodic store.EpisodicStore
	llm      LLMResolver
	cfg      SummarizerConfig
	nowFn    func() time.Time
}

// NewSummarizer constructs a Summarizer wired to the upstream EpisodicStore.
// llm may be nil — it will fall back to NewNoopLLMResolver().
func NewSummarizer(ep store.EpisodicStore, llm LLMResolver, cfg SummarizerConfig) Summarizer {
	cfg.defaults()
	if llm == nil {
		llm = NewNoopLLMResolver()
	}
	return &summarizerImpl{
		episodic: ep,
		llm:      llm,
		cfg:      cfg,
		nowFn:    func() time.Time { return time.Now().UTC() },
	}
}

// AlreadySummarized reports whether an episodic summary with the given
// SourceID exists for this (agent, psid) pair.
func (s *summarizerImpl) AlreadySummarized(ctx context.Context, agentID uuid.UUID, psid, sourceID string) (bool, error) {
	return s.episodic.ExistsBySourceID(ctx, agentID.String(), psid, sourceID)
}

// Summarize writes one EpisodicSummary for the given PSID, idempotent on
// SourceID. Skips if an entry exists (unless ForceRecreate).
func (s *summarizerImpl) Summarize(ctx context.Context, in SummarizeInput) error {
	if len(in.Messages) == 0 {
		slog.Debug("fb_backfill.summarize.empty", "source_id", in.SourceID)
		return nil
	}

	// Idempotency check — also handles the ForceRecreate path.
	exists, err := s.episodic.ExistsBySourceID(ctx, in.AgentID.String(), in.PSID, in.SourceID)
	if err != nil {
		return fmt.Errorf("fbbackfill: exists-by-source-id: %w", err)
	}
	if exists {
		if !in.ForceRecreate {
			slog.Debug("fb_backfill.summarize.skip_existing", "source_id", in.SourceID)
			return nil
		}
		// ForceRecreate: delete the existing entry so we can write a fresh one.
		// We do not know the UUID directly; list by agent/user and match by SourceID.
		if err := s.deleteExistingBySourceID(ctx, in); err != nil {
			slog.Warn("fb_backfill.summarize.delete_existing_failed",
				"source_id", in.SourceID, "err", err)
			// Fall through — Create may fail with duplicate, but try anyway.
		}
	}

	// Messages must be chronological for the LLM. Caller is responsible
	// but sort here defensively.
	msgs := make([]Message, len(in.Messages))
	copy(msgs, in.Messages)
	sort.SliceStable(msgs, func(i, j int) bool {
		return msgs[i].CreatedTime.Before(msgs[j].CreatedTime)
	})

	var summary, l0 string
	var topics []string
	var tokenCount int
	var path string

	if len(msgs) <= s.cfg.ShortPathThreshold {
		summary, l0, topics = s.shortPath(msgs, in.PageID)
		path = "short"
	} else {
		if res, err := s.llmSummarize(ctx, in.TenantID, msgs, in.PageID); err == nil && res != nil {
			summary, l0, topics, tokenCount = res.Summary, res.L0, res.Topics, res.TokenCount
			path = "llm"
		} else {
			if err != nil {
				slog.Warn("fb_backfill.summarize.llm_failed",
					"source_id", in.SourceID, "err", err, "fallback", "concat")
			}
			summary, l0, topics = s.shortPath(msgs, in.PageID)
			path = "short_fallback"
		}
	}
	if summary == "" {
		summary = "(empty conversation)"
	}
	if l0 == "" {
		l0 = "Historical Messenger thread"
	}

	now := s.nowFn()
	expires := now.Add(s.cfg.EpisodicExpiry)
	ep := &store.EpisodicSummary{
		ID:         uuid.New(),
		TenantID:   in.TenantID,
		AgentID:    in.AgentID,
		UserID:     in.PSID,
		SessionKey: in.PSID, // must match webhook runtime convention: chatID = senderID = PSID
		Summary:    summary,
		KeyTopics:  topics,
		L0Abstract: l0,
		SourceType: "fb_backfill",
		SourceID:   in.SourceID,
		TurnCount:  len(msgs),
		TokenCount: tokenCount,
		CreatedAt:  now,
		ExpiresAt:  &expires,
	}
	if err := s.episodic.Create(ctx, ep); err != nil {
		return fmt.Errorf("fbbackfill: create episodic: %w", err)
	}
	slog.Info("fb_backfill.summarize.created",
		"source_id", in.SourceID, "turn_count", ep.TurnCount,
		"l0_len", len(l0), "tokens_used", tokenCount, "path", path)
	return nil
}

// deleteExistingBySourceID finds the single episodic entry matching
// SourceID and deletes it. EpisodicStore.Delete takes an ID, not a
// SourceID, so we page through the user's entries looking for the match.
// Backfill SourceIDs are unique per (page_id, psid) so the search is
// bounded.
func (s *summarizerImpl) deleteExistingBySourceID(ctx context.Context, in SummarizeInput) error {
	const batchSize = 50
	offset := 0
	for {
		rows, err := s.episodic.List(ctx, in.AgentID.String(), in.PSID, batchSize, offset)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return errors.New("fbbackfill: no existing episodic matched")
		}
		for _, r := range rows {
			if r.SourceID == in.SourceID {
				return s.episodic.Delete(ctx, r.ID.String())
			}
		}
		if len(rows) < batchSize {
			return errors.New("fbbackfill: no existing episodic matched")
		}
		offset += batchSize
	}
}

// ---- Short path (no LLM) ----

// shortPath builds a summary, L0 abstract, and topic list directly from
// message text. Zero LLM cost. Used for short conversations and as a
// fallback when the LLM is unavailable or fails.
func (s *summarizerImpl) shortPath(msgs []Message, pageID string) (summary, l0 string, topics []string) {
	var b strings.Builder
	for _, m := range msgs {
		who := "customer"
		if m.From.ID == pageID {
			who = "page"
		}
		ts := ""
		if !m.CreatedTime.IsZero() {
			ts = m.CreatedTime.Format("2006-01-02 15:04") + " "
		}
		text := m.Message
		if text == "" && len(m.Attachments.Data) > 0 {
			text = fmt.Sprintf("(sent %d attachment(s))", len(m.Attachments.Data))
		}
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[%s%s] %s\n", ts, who, text))
	}
	summary = strings.TrimSpace(b.String())
	l0 = summarizeL0(summary, msgs, pageID)
	topics = extractTopics(msgs)
	return summary, l0, topics
}

// summarizeL0 composes a 1-2 sentence L0 abstract (~50 tokens).
func summarizeL0(summary string, msgs []Message, pageID string) string {
	// Count customer vs page messages
	var customerMsgs, pageMsgs int
	var firstCustomerText string
	for _, m := range msgs {
		if m.From.ID == pageID {
			pageMsgs++
		} else {
			customerMsgs++
			if firstCustomerText == "" && m.Message != "" {
				firstCustomerText = m.Message
			}
		}
	}
	snippet := firstCustomerText
	if len(snippet) > 140 {
		snippet = snippet[:137] + "..."
	}
	if snippet == "" {
		return fmt.Sprintf("Historical conversation: %d customer msgs, %d page replies.",
			customerMsgs, pageMsgs)
	}
	return fmt.Sprintf("Customer: %q (%d msgs / %d replies).", snippet, customerMsgs, pageMsgs)
}

// extractTopics picks up to 5 distinct frequently-used words from the
// conversation as lightweight topic tags. This is intentionally crude —
// it is only a fallback when the LLM does not produce topics.
func extractTopics(msgs []Message) []string {
	counts := map[string]int{}
	for _, m := range msgs {
		for _, w := range tokenizeWords(m.Message) {
			counts[w]++
		}
	}
	type pair struct {
		word  string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for w, c := range counts {
		if c < 2 || len(w) < 4 || isStopword(w) {
			continue
		}
		pairs = append(pairs, pair{w, c})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	out := make([]string, 0, 5)
	for i, p := range pairs {
		if i >= 5 {
			break
		}
		out = append(out, p.word)
	}
	return out
}

func tokenizeWords(s string) []string {
	var out []string
	var cur strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r >= 0x80 {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

var stopwords = map[string]struct{}{
	"have": {}, "this": {}, "that": {}, "with": {}, "from": {}, "your": {},
	"been": {}, "were": {}, "they": {}, "them": {}, "what": {}, "when": {},
	"which": {}, "about": {}, "there": {}, "their": {}, "would": {},
	"please": {}, "thank": {}, "thanks": {}, "hello": {},
	"được":  {}, "không": {}, "mình": {}, "shop":  {}, "chào":  {}, "nhé":   {},
	"với":   {}, "vậy":   {}, "rồi":   {}, "nhưng": {}, "hoặc":  {},
}

func isStopword(w string) bool { _, ok := stopwords[w]; return ok }

// ---- LLM path ----

// llmSummaryResult is the JSON schema we expect from the LLM.
type llmSummaryResult struct {
	Summary    string   `json:"summary"`
	L0         string   `json:"l0"`
	Topics     []string `json:"topics"`
	TokenCount int      `json:"-"`
}

// llmSummarize calls the tenant's background provider with a transcript.
// Returns nil result (no error) when no provider is available.
func (s *summarizerImpl) llmSummarize(ctx context.Context, tenantID uuid.UUID, msgs []Message, pageID string) (*llmSummaryResult, error) {
	client, model := s.llm.Resolve(ctx, tenantID)
	if client == nil {
		return nil, nil
	}
	transcript := buildTranscript(msgs, pageID, s.cfg.MaxTranscriptMessages)
	req := providers.ChatRequest{
		Model: model,
		Messages: []providers.Message{
			{Role: "system", Content: summarizerSystemPrompt},
			{Role: "user", Content: transcript},
		},
	}
	resp, err := client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil || strings.TrimSpace(resp.Content) == "" {
		return nil, errors.New("fbbackfill: empty LLM response")
	}
	result, err := parseLLMSummary(resp.Content)
	if err != nil {
		return nil, err
	}
	if resp.Usage != nil {
		result.TokenCount = resp.Usage.CompletionTokens
	}
	return result, nil
}

const summarizerSystemPrompt = `You are summarizing a past conversation between a Facebook Page and a customer.
The transcript contains timestamped messages. Produce a JSON object with exactly:
1. "summary": 100-200 words capturing the customer's intent, the main topics discussed, any commitments made, and the last known state of the conversation.
2. "l0": a single sentence of at most 30 words that the AI agent can glance at to recall this customer on next contact.
3. "topics": an array of 3-5 short tags (1-2 words each) for the key subjects.

Write "summary" and "l0" in the language the customer predominantly used. Respond with ONLY the JSON object, no preamble.`

func buildTranscript(msgs []Message, pageID string, maxMsgs int) string {
	// When truncating, keep the most recent messages (tail) since they are
	// more relevant to future context.
	start := 0
	if len(msgs) > maxMsgs {
		start = len(msgs) - maxMsgs
	}
	var b strings.Builder
	for _, m := range msgs[start:] {
		who := "customer"
		if m.From.ID == pageID {
			who = "page"
		}
		ts := ""
		if !m.CreatedTime.IsZero() {
			ts = m.CreatedTime.Format("2006-01-02 15:04") + " "
		}
		text := m.Message
		if text == "" && len(m.Attachments.Data) > 0 {
			text = fmt.Sprintf("(attachment: %d file(s))", len(m.Attachments.Data))
		}
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("[%s%s] %s\n", ts, who, text))
	}
	if start > 0 {
		return fmt.Sprintf("(Earlier %d messages omitted)\n\n%s", start, b.String())
	}
	return b.String()
}

// parseLLMSummary tolerates common formatting artifacts (prose preamble,
// code fences) around the JSON object.
func parseLLMSummary(raw string) (*llmSummaryResult, error) {
	s := strings.TrimSpace(raw)
	// Strip markdown code fences if present.
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	// Find first '{' and last '}' in case model wrapped with prose.
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i >= 0 && j > i {
		s = s[i : j+1]
	}
	var r llmSummaryResult
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return nil, fmt.Errorf("fbbackfill: parse llm summary: %w", err)
	}
	if r.Summary == "" {
		return nil, errors.New("fbbackfill: LLM summary missing 'summary'")
	}
	if r.L0 == "" {
		r.L0 = truncate(r.Summary, 200)
	}
	return &r, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
