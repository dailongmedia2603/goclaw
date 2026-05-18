// Package agent — response sanitization pipeline.
//
// Matching TS sanitization chain:
//
//	extractAssistantText() → per-block:
//	  1. stripMinimaxToolCallXml()        → Go: stripGarbledToolXML()
//	  2. stripDowngradedToolCallText()     → Go: stripDowngradedToolCallText()
//	  3. stripThinkingTagsFromText()       → Go: stripThinkingTags()
//	  then:
//	  4. sanitizeUserFacingText()          → Go: sanitizeUserFacingText()
//	     - stripFinalTagsFromText()        → Go: stripFinalTags()
//	     - collapseConsecutiveDuplicateBlocks()
//
// Additional Go-specific:
//	  5. stripEchoedSystemMessages()       → strip hallucinated [System Message] blocks
//	  6. stripGarbledToolXML()             → strip garbled XML from models like DeepSeek
package agent

import (
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func truncPreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// SanitizeAssistantContent applies the full sanitization pipeline to assistant
// response text before saving to session and sending to user.
// Matching TS extractAssistantText() + sanitizeUserFacingText().
func SanitizeAssistantContent(content string) string {
	if content == "" {
		return content
	}

	original := content

	logDrain := func(step string, before, after string) {
		if len(after) < len(before) {
			slog.Info("sanitize.drain",
				"step", step,
				"before_len", len(before),
				"after_len", len(after),
				"dropped", len(before)-len(after),
				"preview_before", truncPreview(before, 200),
			)
		}
	}

	// 1. Strip garbled tool-call XML (DeepSeek, GLM, Minimax)
	prev := content
	content = stripGarbledToolXML(content)
	logDrain("1_garbled_xml", prev, content)
	if content == "" {
		slog.Warn("sanitize.empty_after_step", "step", "1_garbled_xml", "original_len", len(original), "preview", truncPreview(original, 300))
		return ""
	}

	// 2. Strip downgraded tool call text ([Tool Call: ...], [Tool Result ...])
	prev = content
	content = stripDowngradedToolCallText(content)
	logDrain("2_downgraded_tool", prev, content)

	// 3. Strip thinking/reasoning tags (<think>, <thinking>, <thought>, <antThinking>)
	prev = content
	content = stripThinkingTags(content)
	logDrain("3_thinking_tags", prev, content)

	// 4. Strip <final> tags (keep content inside)
	prev = content
	content = stripFinalTags(content)
	logDrain("4_final_tags", prev, content)

	// 5. Strip echoed [System Message] blocks
	prev = content
	content = stripEchoedSystemMessages(content)
	logDrain("5_system_messages", prev, content)

	// 6. Collapse consecutive duplicate blocks
	prev = content
	content = collapseConsecutiveDuplicateBlocks(content)
	logDrain("6_collapse_dupes", prev, content)

	// 7. Strip MEDIA: paths from LLM output (media delivered separately)
	prev = content
	content = stripMediaPaths(content)
	logDrain("7_media_paths", prev, content)

	// 8. Normalize LaTeX math notation to Unicode (e.g. $\rightarrow$ → →, $\alpha$ → α).
	content = normalizeLatex(content)

	// 9. Strip leading blank lines (preserve indentation)
	content = stripLeadingBlankLines(content)

	content = strings.TrimSpace(content)

	if content == "" && original != "" {
		slog.Warn("sanitize.fully_drained",
			"original_len", len(original),
			"preview", truncPreview(original, 500),
		)
	} else if content != original {
		slog.Debug("sanitized assistant content",
			"original_len", len(original),
			"cleaned_len", len(content),
		)
	}

	return content
}

// --- 1. Garbled tool-call XML ---

// garbledToolXMLPattern matches XML-like tool call artifacts that some models
// (DeepSeek, GLM, etc.) emit as text content instead of proper tool calls.
var garbledToolXMLPattern = regexp.MustCompile(
	`(?s)</?(?:function_calls?|functioninvoke|invoke|invfunction_calls|tool_call|tool_use|parameter|minimax:tool_call)[^>]*>`,
)

var garbledToolXMLIndicators = []string{
	"invfunction_calls",
	"functioninvoke",
	"<parameter name=",
	"</parameter",
	"<function_call",
	"<tool_call",
	"<tool_use",
	"<minimax:tool_call",
}

func stripGarbledToolXML(content string) string {
	hasIndicator := false
	lower := strings.ToLower(content)
	for _, ind := range garbledToolXMLIndicators {
		if strings.Contains(lower, strings.ToLower(ind)) {
			hasIndicator = true
			break
		}
	}
	if !hasIndicator {
		return content
	}

	cleaned := garbledToolXMLPattern.ReplaceAllString(content, "")
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		slog.Warn("stripped entire response as garbled tool XML", "original_len", len(content))
		return ""
	}

	slog.Warn("stripped garbled tool call XML from response",
		"original_len", len(content),
		"remaining_len", len(cleaned),
	)
	return cleaned
}

// --- 2. Downgraded tool call text ---

// stripDowngradedToolCallText removes [Tool Call: ...], [Tool Result ...],
// and [Historical context: ...] blocks that some models emit as text.
// Matching TS stripDowngradedToolCallText().
// Uses line-by-line scanning (Go regexp doesn't support lookahead).
func stripDowngradedToolCallText(content string) string {
	if !strings.Contains(content, "[Tool Call:") &&
		!strings.Contains(content, "[Tool Result") &&
		!strings.Contains(content, "[Historical context:") {
		return content
	}

	lines := strings.Split(content, "\n")
	var result []string
	skipping := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Start skipping on these markers
		if strings.HasPrefix(trimmed, "[Tool Call:") ||
			strings.HasPrefix(trimmed, "[Tool Result") ||
			strings.HasPrefix(trimmed, "[Historical context:") {
			skipping = true
			continue
		}

		// Stop skipping on non-indented, non-empty line that isn't part of the block
		if skipping {
			// Arguments JSON and tool output are typically indented or empty
			if trimmed == "" || strings.HasPrefix(trimmed, "Arguments:") ||
				strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "}") {
				continue
			}
			// Non-tool-block line → stop skipping
			skipping = false
		}

		result = append(result, line)
	}

	return strings.TrimSpace(strings.Join(result, "\n"))
}

// --- 3. Thinking/reasoning tags ---

// Matches TS stripThinkingTagsFromText() with strict mode.
// Strips: <redacted_thinking>...</redacted_thinking>, <think>...</think>,
//         <thinking>...</thinking>, <thought>...</thought>,
//         <antThinking>...</antThinking>
// Go regexp doesn't support backreferences, so we use separate patterns.
var thinkingTagPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<redacted_thinking\b[^>]*>.*?</redacted_thinking\s*>`),
	regexp.MustCompile(`(?is)<thinking\b[^>]*>.*?</thinking\s*>`),
	regexp.MustCompile(`(?is)<think\b[^>]*>.*?</think\s*>`),
	regexp.MustCompile(`(?is)<thought\b[^>]*>.*?</thought\s*>`),
	regexp.MustCompile(`(?is)<antThinking\b[^>]*>.*?</antThinking\s*>`),
	regexp.MustCompile(`(?is)<antthinking\b[^>]*>.*?</antthinking\s*>`),
}

func stripThinkingTags(content string) string {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "<think") && !strings.Contains(lower, "<thought") &&
		!strings.Contains(lower, "<antthinking") && !strings.Contains(lower, "<redacted_thinking") {
		return content
	}
	result := content
	for _, pat := range thinkingTagPatterns {
		result = pat.ReplaceAllString(result, "")
	}
	return strings.TrimSpace(result)
}

// --- 4. <final> tags ---

// Matches TS stripFinalTagsFromText(). Removes <final> and </final> tags
// but keeps the content inside.
var finalTagPattern = regexp.MustCompile(`(?i)<\s*/?\s*final\s*>`)

func stripFinalTags(content string) string {
	if !strings.Contains(strings.ToLower(content), "final") {
		return content
	}
	return finalTagPattern.ReplaceAllString(content, "")
}

// --- 5. Echoed [System Message] ---

// stripEchoedSystemMessages removes "[System Message] ..." blocks that LLMs
// hallucinate/echo in their response text.
// Uses line-based scanning (Go regexp doesn't support lookahead).
func stripEchoedSystemMessages(content string) string {
	if !strings.Contains(content, "[System Message]") {
		return content
	}

	lines := strings.Split(content, "\n")
	var result []string
	skipping := false

	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[System Message]") {
			skipping = true
			continue
		}
		if skipping {
			// Empty line ends the system message block
			if strings.TrimSpace(line) == "" {
				skipping = false
				continue
			}
			// Still part of the system message block (Stats:, reply instructions, etc.)
			continue
		}
		result = append(result, line)
	}

	cleaned := strings.TrimSpace(strings.Join(result, "\n"))

	if cleaned != strings.TrimSpace(content) {
		slog.Warn("stripped echoed [System Message] from assistant response",
			"original_len", len(content),
			"cleaned_len", len(cleaned),
		)
	}

	return cleaned
}

// --- 6. Collapse consecutive duplicate blocks ---

// collapseConsecutiveDuplicateBlocks removes repeated paragraph blocks.
// Matching TS collapseConsecutiveDuplicateBlocks().
func collapseConsecutiveDuplicateBlocks(content string) string {
	blocks := strings.Split(content, "\n\n")
	if len(blocks) <= 1 {
		return content
	}

	var result []string
	for i, block := range blocks {
		trimmed := strings.TrimSpace(block)
		if trimmed == "" {
			continue
		}
		if i > 0 && len(result) > 0 && trimmed == strings.TrimSpace(result[len(result)-1]) {
			continue // skip duplicate
		}
		result = append(result, block)
	}

	collapsed := strings.Join(result, "\n\n")
	if collapsed != content {
		slog.Debug("collapsed duplicate blocks",
			"original_blocks", len(blocks),
			"result_blocks", len(result),
		)
	}
	return collapsed
}

// --- 7. Strip MEDIA: paths ---

// mediaPathPattern matches "MEDIA:" followed by a path (absolute or relative).
var mediaPathPattern = regexp.MustCompile(`MEDIA:\S+`)

// stripMediaPaths removes lines containing MEDIA:/path references from LLM output.
// These are tool result artifacts that should not appear in user-facing text
// (media files are delivered separately via OutboundMessage.Media).
func stripMediaPaths(content string) string {
	if !strings.Contains(content, "MEDIA:") {
		return content
	}
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[[audio_as_voice]]") {
			continue
		}
		// Strip any line containing a MEDIA: path reference, regardless of wrapping format.
		// LLMs echo these in many forms: bare "MEDIA:/path", markdown "![alt](MEDIA:relative/path)",
		// JSON '{"image":"MEDIA:/path"}', etc. Match MEDIA: followed by any non-space path char.
		if mediaPathPattern.MatchString(trimmed) {
			continue
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}

// --- 8. Strip leading blank lines ---

var leadingBlankLinesPattern = regexp.MustCompile(`^(?:[ \t]*\r?\n)+`)

func stripLeadingBlankLines(content string) string {
	return leadingBlankLinesPattern.ReplaceAllString(content, "")
}

// --- 9. Config leak detection (predefined agents) ---

// configLeakFileNames are internal file names that should not appear in user-facing output
// when a predefined agent describes its procedures or configuration.
var configLeakFileNames = []string{
	"SOUL.md", "IDENTITY.md", "AGENTS.md", "BOOTSTRAP.md",
	"internal_config", "system prompt",
}

// Patterns to strip markdown code from content before config leak detection.
// Mentions inside code blocks/inline code are typically architecture docs, not leaks.
var fencedCodeBlockPattern = regexp.MustCompile("(?s)```[^`]*```")
var inlineCodePattern = regexp.MustCompile("`[^`\n]+`")

// stripMarkdownCode removes fenced code blocks and inline code from text.
func stripMarkdownCode(s string) string {
	s = fencedCodeBlockPattern.ReplaceAllString(s, "")
	s = inlineCodePattern.ReplaceAllString(s, "")
	return s
}

// StripConfigLeak detects when a predefined agent dumps its internal configuration
// (e.g. referencing SOUL.md, AGENTS.md, IDENTITY.md) and replaces the entire
// response with a friendly decline.
//
// Only active for predefined agents. Single-gate detection:
// 3+ distinct internal file names mentioned in plain text → replace entire response.
// Mentions inside markdown code blocks and inline code are excluded from counting,
// as they typically appear in architecture explanations rather than actual leaks.
func StripConfigLeak(content, agentType string) string {
	if agentType != store.AgentTypePredefined || content == "" {
		return content
	}

	// Count hits only in plain text (outside code blocks/inline code)
	plain := stripMarkdownCode(content)

	hits := 0
	for _, name := range configLeakFileNames {
		if strings.Contains(plain, name) {
			hits++
		}
	}
	if hits < 3 {
		return content
	}

	slog.Warn("security.config_leak_stripped",
		"file_hits", hits,
		"original_len", len(content),
	)

	return "🔒 Security check not passed."
}

// --- NO_REPLY detection ---

// IsSilentReply checks if the text begins with a NO_REPLY token.
//
// Divergent from TS isSilentReplyText() (exact-match only) — we match broadly:
// decorative wrappers (`NO_REPLY_`, `"NO_REPLY"`, `**NO_REPLY**`) AND trailing
// explanations (`NO_REPLY because offline`, `NO_REPLY: note`) suppress delivery.
// Only requirement: the token is not glued to another word (`NO_REPLYING` is NOT silent).
// Case-insensitive.
//
// Trade-off vs upstream #19537: upstream guards against suppressing substantive
// replies that end in NO_REPLY. We accept that risk because observed model output
// leans toward "NO_REPLY + reason" rather than "real reply ending in NO_REPLY".
func IsSilentReply(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	// Strip decorative wrappers from both ends (quotes, markdown emphasis, punctuation).
	stripped := strings.Trim(trimmed, "_ \t\n\r.,:;!?\"'`*~#>-()[]{}")
	const token = "NO_REPLY"
	if len(stripped) < len(token) {
		return false
	}
	if !strings.EqualFold(stripped[:len(token)], token) {
		return false
	}
	if len(stripped) == len(token) {
		return true
	}
	// Token must not be glued to another word — next rune must be non-alphanumeric.
	next, _ := utf8.DecodeRuneInString(stripped[len(token):])
	return !isAlphaNum(next)
}

func isAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

// --- Normalize LaTeX math notation to Unicode ---

// latexMacroMap translates common LaTeX macros (Greek letters, arrows, relations,
// operators, sets, logic, calculus) into their Unicode equivalents. Extend as needed.
var latexMacroMap = map[string]string{
	// Arrows
	`\rightarrow`:     "→",
	`\Rightarrow`:     "⇒",
	`\leftarrow`:      "←",
	`\Leftarrow`:      "⇐",
	`\leftrightarrow`: "↔",
	`\Leftrightarrow`: "⇔",
	`\to`:             "→",
	`\gets`:           "←",
	`\mapsto`:         "↦",
	`\uparrow`:        "↑",
	`\downarrow`:      "↓",
	`\longrightarrow`: "⟶",
	`\longleftarrow`:  "⟵",

	// Greek lowercase
	`\alpha`: "α", `\beta`: "β", `\gamma`: "γ", `\delta`: "δ",
	`\epsilon`: "ε", `\varepsilon`: "ε", `\zeta`: "ζ", `\eta`: "η",
	`\theta`: "θ", `\vartheta`: "ϑ", `\iota`: "ι", `\kappa`: "κ",
	`\lambda`: "λ", `\mu`: "μ", `\nu`: "ν", `\xi`: "ξ",
	`\pi`: "π", `\varpi`: "ϖ", `\rho`: "ρ", `\varrho`: "ϱ",
	`\sigma`: "σ", `\varsigma`: "ς", `\tau`: "τ", `\upsilon`: "υ",
	`\phi`: "φ", `\varphi`: "ϕ", `\chi`: "χ", `\psi`: "ψ", `\omega`: "ω",

	// Greek uppercase
	`\Gamma`: "Γ", `\Delta`: "Δ", `\Theta`: "Θ", `\Lambda`: "Λ",
	`\Xi`: "Ξ", `\Pi`: "Π", `\Sigma`: "Σ", `\Upsilon`: "Υ",
	`\Phi`: "Φ", `\Psi`: "Ψ", `\Omega`: "Ω",

	// Operators & relations
	`\times`: "×", `\div`: "÷", `\pm`: "±", `\mp`: "∓",
	`\cdot`: "·", `\cdots`: "⋯", `\ldots`: "…", `\dots`: "…",
	`\leq`: "≤", `\le`: "≤", `\geq`: "≥", `\ge`: "≥",
	`\neq`: "≠", `\ne`: "≠", `\approx`: "≈", `\equiv`: "≡",
	`\sim`: "∼", `\simeq`: "≃", `\cong`: "≅", `\propto`: "∝",
	`\ll`: "≪", `\gg`: "≫",

	// Sets & logic
	`\in`: "∈", `\notin`: "∉", `\ni`: "∋",
	`\subset`: "⊂", `\supset`: "⊃", `\subseteq`: "⊆", `\supseteq`: "⊇",
	`\cup`: "∪", `\cap`: "∩", `\setminus`: "∖",
	`\emptyset`: "∅", `\varnothing`: "∅",
	`\forall`: "∀", `\exists`: "∃", `\nexists`: "∄",
	`\land`: "∧", `\wedge`: "∧", `\lor`: "∨", `\vee`: "∨",
	`\neg`: "¬", `\lnot`: "¬",

	// Calculus & misc
	`\infty`: "∞", `\partial`: "∂", `\nabla`: "∇",
	`\sum`: "∑", `\prod`: "∏", `\int`: "∫", `\oint`: "∮",
	`\sqrt`: "√",
}

var (
	latexMacroPattern        = regexp.MustCompile(`\\[a-zA-Z]+`)
	latexDisplayMathPattern  = regexp.MustCompile(`(?s)\$\$(.+?)\$\$`)
	latexInlineMathPattern   = regexp.MustCompile(`\$([^$\n]+?)\$`)
	latexParenMathPattern    = regexp.MustCompile(`(?s)\\\((.+?)\\\)`)
	latexBracketMathPattern  = regexp.MustCompile(`(?s)\\\[(.+?)\\\]`)
	latexProtectCodeBlockRe  = regexp.MustCompile("(?s)```.*?```")
	latexProtectInlineCodeRe = regexp.MustCompile("`[^`\n]+`")
	latexProtectPlaceholder  = regexp.MustCompile("\x00LTX(\\d+)\x00")
)

func replaceLatexMacros(s string) string {
	return latexMacroPattern.ReplaceAllStringFunc(s, func(m string) string {
		if repl, ok := latexMacroMap[m]; ok {
			return repl
		}
		return m
	})
}

// normalizeLatex strips common LaTeX math delimiters ($...$, $$...$$, \(...\), \[...\])
// and replaces LaTeX macros inside with Unicode equivalents. For $...$ and $$...$$,
// the delimiters are only stripped when the content contains at least one backslash —
// this preserves currency mentions like "$50 and $100" as plain prose.
func normalizeLatex(content string) string {
	if !strings.Contains(content, `\`) {
		return content
	}

	// Protect fenced + inline code so LaTeX discussions inside code stay literal.
	var protected []string
	stash := func(m string) string {
		idx := len(protected)
		protected = append(protected, m)
		return "\x00LTX" + strconv.Itoa(idx) + "\x00"
	}
	content = latexProtectCodeBlockRe.ReplaceAllStringFunc(content, stash)
	content = latexProtectInlineCodeRe.ReplaceAllStringFunc(content, stash)

	content = latexDisplayMathPattern.ReplaceAllStringFunc(content, func(m string) string {
		inner := m[2 : len(m)-2]
		if !strings.Contains(inner, `\`) {
			return m
		}
		return replaceLatexMacros(inner)
	})
	content = latexInlineMathPattern.ReplaceAllStringFunc(content, func(m string) string {
		inner := m[1 : len(m)-1]
		if !strings.Contains(inner, `\`) {
			return m
		}
		return replaceLatexMacros(inner)
	})
	content = latexParenMathPattern.ReplaceAllStringFunc(content, func(m string) string {
		return replaceLatexMacros(m[2 : len(m)-2])
	})
	content = latexBracketMathPattern.ReplaceAllStringFunc(content, func(m string) string {
		return replaceLatexMacros(m[2 : len(m)-2])
	})

	// Restore protected code blocks.
	content = latexProtectPlaceholder.ReplaceAllStringFunc(content, func(m string) string {
		idx, err := strconv.Atoi(m[len("\x00LTX") : len(m)-1])
		if err != nil || idx < 0 || idx >= len(protected) {
			return m
		}
		return protected[idx]
	})

	return content
}

// --- Message Directives ([[name:value]]) ---

// messageDirectivePattern matches structured routing tags: [[word]] or [[word:value]].
// Single-line only (no (?s) dotall). Does NOT match arbitrary [[...]] content.
var messageDirectivePattern = regexp.MustCompile(`\[\[\w+(?::[^\]\n]+)?\]\]`)

// StripMessageDirectives removes internal [[...]] routing tags from user-facing text,
// preserving [[tts...]] tags needed by the TTS auto-apply pipeline.
func StripMessageDirectives(content string) string {
	if !strings.Contains(content, "[[") {
		return content
	}
	result := messageDirectivePattern.ReplaceAllStringFunc(content, func(match string) string {
		inner := match[2 : len(match)-2] // strip [[ and ]]
		if strings.HasPrefix(inner, "tts") {
			return match // preserve for TTS AutoTagged mode
		}
		return ""
	})
	return strings.TrimSpace(result)
}
