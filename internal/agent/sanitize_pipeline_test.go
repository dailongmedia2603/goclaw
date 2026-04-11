package agent

import (
	"strings"
	"testing"
)

// pipelineTestCase describes a single golden test case for SanitizeAssistantContent.
//
// bugFlagged = true means the expected "want" value currently reflects a known bug
// (stripPlainTextReasoning drains non-empty content to empty). Phase 3 of the
// "remove-reasoning-text-heuristic" plan will flip these cases to correct output.
type pipelineTestCase struct {
	name       string
	input      string
	want       string
	bugFlagged bool
}

// TestSanitizeAssistantContent_Pipeline locks the behavior of the full
// SanitizeAssistantContent pipeline. Each case exercises one or more transformers.
// Phase 1 (golden tests) must pass BEFORE any code removal in later phases.
func TestSanitizeAssistantContent_Pipeline(t *testing.T) {
	cases := []pipelineTestCase{
		// --- Group A: Happy path (no transformation) ---
		{
			name:  "happy_plain_text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "happy_markdown_bullet_list",
			input: "- A\n- B\n- C",
			want:  "- A\n- B\n- C",
		},
		{
			name:  "happy_numbered_list",
			input: "1. first\n2. second\n3. third",
			want:  "1. first\n2. second\n3. third",
		},
		{
			name:  "happy_vietnamese_with_emoji",
			input: "Chào bạn 👋 đây là câu trả lời",
			want:  "Chào bạn 👋 đây là câu trả lời",
		},
		{
			name:  "happy_code_block",
			input: "Here's the code:\n```go\nfmt.Println(\"hi\")\n```",
			want:  "Here's the code:\n```go\nfmt.Println(\"hi\")\n```",
		},
		{
			name:  "happy_multiline_paragraphs",
			input: "First para.\n\nSecond para.\n\nThird para.",
			want:  "First para.\n\nSecond para.\n\nThird para.",
		},

		// --- Group B: Tool call stripping ---
		{
			name:  "tool_garbled_xml_tags_only",
			input: "<function_calls><invoke name=\"read_file\"></invoke></function_calls>",
			want:  "",
		},
		{
			name:  "tool_garbled_xml_keeps_text_between_tags",
			input: "<function_calls>some text<parameter name=\"x\">leak</parameter></function_calls>",
			want:  "some textleak",
		},
		{
			name:  "tool_downgraded_text_block",
			input: "[Tool Call: read_file]\nArguments: {\"path\":\"x\"}\n\nHere is the answer",
			want:  "Here is the answer",
		},
		{
			name:  "tool_bare_function_call",
			input: "take_snapshot(targetId=\"abc\")",
			want:  "",
		},

		// --- Group C: Thinking tag stripping (STRUCTURAL — must be preserved) ---
		{
			name:  "think_tag_basic",
			input: "<think>inner reasoning</think>final answer",
			want:  "final answer",
		},
		{
			name:  "think_tag_thinking_variant",
			input: "<thinking>deliberating</thinking>\n\nOK done",
			want:  "OK done",
		},
		{
			name:  "think_tag_multiline",
			input: "<think>\nstep 1\nstep 2\n</think>\nThe answer is 42",
			want:  "The answer is 42",
		},
		{
			name:  "think_tag_orphan_close",
			input: "some content </thought>",
			want:  "some content",
		},

		// --- Group D: Reasoning text prefix (BUG AREA — Phase 3 will flip these) ---
		{
			name:       "reasoning_pure_bullets_bug",
			input:      "Reasoning:\n• step one\n• step two",
			want:       "",
			bugFlagged: true,
		},
		{
			name:       "reasoning_then_dash_bullet_answer_bug",
			input:      "Reasoning:\n• think\n\n- First\n- Second",
			want:       "",
			bugFlagged: true,
		},
		{
			name:       "reasoning_then_star_bullet_answer_bug",
			input:      "Reasoning:\n• think\n\n* Item 1\n* Item 2",
			want:       "",
			bugFlagged: true,
		},
		{
			name:       "reasoning_then_indented_answer_bug",
			input:      "Reasoning:\n• think\n\n    code block line",
			want:       "",
			bugFlagged: true,
		},
		{
			name:  "reasoning_then_plain_text_answer",
			input: "Reasoning:\n• think\n\nYes absolutely.",
			want:  "Yes absolutely.",
		},
		{
			name:  "reasoning_then_vietnamese_text_answer",
			input: "Reasoning:\n• hmm\n\nChào bạn, đây là câu trả lời",
			want:  "Chào bạn, đây là câu trả lời",
		},
		{
			name:  "no_reasoning_prefix_passthrough",
			input: "Just a plain reply",
			want:  "Just a plain reply",
		},

		// --- Group E: Final tags + system message echo ---
		{
			name:  "final_tag_keeps_content",
			input: "<final>this is the answer</final>",
			want:  "this is the answer",
		},
		{
			name:  "system_message_echo_stripped",
			input: "[System Message]\nStats: 10 tokens\n\nReal answer here",
			want:  "Real answer here",
		},

		// --- Group F: Media + collapse + LaTeX ---
		{
			name:  "media_path_stripped_with_text_before",
			input: "Here you go:\nMEDIA:/tmp/abc.png",
			want:  "Here you go:",
		},
		{
			name:  "collapse_duplicate_blocks",
			input: "A\n\nB\n\nB\n\nC",
			want:  "A\n\nB\n\nC",
		},
		{
			name:  "inline_latex_alpha",
			input: "angle is $\\alpha$",
			want:  "angle is α",
		},
		{
			name:  "inline_latex_rightarrow",
			input: "A $\\rightarrow$ B",
			want:  "A → B",
		},

		// --- Group G: Empty / edge cases ---
		{
			name:  "empty_input",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace_only",
			input: "   \n\n  ",
			want:  "",
		},
		{
			name:  "only_think_tag_becomes_empty",
			input: "<think>only reasoning here</think>",
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeAssistantContent(tc.input)
			if got != tc.want {
				if tc.bugFlagged {
					t.Logf("[BUG-FLAGGED, will be fixed in Phase 3]\ninput=%q\nwant=%q\ngot=%q",
						tc.input, tc.want, got)
					// Do NOT fail — bug-flagged cases are expected to match current buggy behavior
					if got != tc.want {
						t.Errorf("bug-flagged case no longer matches current behavior — update plan")
					}
				} else {
					t.Errorf("input=%q\nwant=%q\ngot=%q", tc.input, tc.want, got)
				}
			}
		})
	}
}

// TestSanitizeAssistantContent_IdempotentForPlain verifies that applying the
// pipeline twice to plain content yields the same result (no accidental
// repeated-transformation side effects).
func TestSanitizeAssistantContent_IdempotentForPlain(t *testing.T) {
	inputs := []string{
		"Hello world",
		"- A\n- B\n- C",
		"Chào bạn 👋",
		"1. first\n2. second",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			once := SanitizeAssistantContent(in)
			twice := SanitizeAssistantContent(once)
			if once != twice {
				t.Errorf("pipeline not idempotent for %q:\nonce=%q\ntwice=%q", in, once, twice)
			}
		})
	}
}

// TestSanitizeAssistantContent_NoReplyDetection verifies IsSilentReply
// still recognizes the NO_REPLY sentinel after sanitization.
func TestSanitizeAssistantContent_NoReplyDetection(t *testing.T) {
	cases := []struct {
		input  string
		silent bool
	}{
		{"NO_REPLY", true},
		{"NO_REPLY.", true},
		{"NO_REPLY trailing comment", true},
		{"leading comment NO_REPLY", true},
		{"Hello world", false},
		{"", false},
		{"NOREPLY", false}, // underscore matters
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			got := IsSilentReply(c.input)
			if got != c.silent {
				t.Errorf("IsSilentReply(%q) = %v, want %v", c.input, got, c.silent)
			}
		})
	}
}

// TestStripPlainTextReasoning_CurrentBehavior locks the current (buggy)
// behavior of stripPlainTextReasoning at the unit level. This test will be
// DELETED in Phase 3 when the function is removed.
func TestStripPlainTextReasoning_CurrentBehavior(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"no_prefix_passthrough", "normal text", "normal text"},
		{"pure_reasoning_bullets", "Reasoning:\n• step", ""},
		{"reasoning_then_plain_text", "Reasoning:\n• x\n\nYes.", "Yes."},
		{"reasoning_then_dash_bullets_BUG", "Reasoning:\n• x\n\n- ans", ""},
		{"reasoning_then_star_bullets_BUG", "Reasoning:\n• x\n\n* ans", ""},
		{"reasoning_incomplete_BUG", "Reasoning:\n• analyzing", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripPlainTextReasoning(c.in)
			if got != c.want {
				t.Errorf("stripPlainTextReasoning(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestSanitizeWithEmptyGuard verifies the "never silently drain to empty"
// safety net added in Phase 2.
func TestSanitizeWithEmptyGuard(t *testing.T) {
	drainer := func(s string) string { return "" }
	upper := func(s string) string { return strings.ToUpper(s) }
	passthrough := func(s string) string { return s }

	t.Run("non_empty_drained_returns_original", func(t *testing.T) {
		got := sanitizeWithEmptyGuard("drainer", "hello", drainer)
		if got != "hello" {
			t.Errorf("guard should keep original when drained: got %q", got)
		}
	})

	t.Run("non_empty_drained_multiline_kept", func(t *testing.T) {
		in := "line 1\nline 2"
		got := sanitizeWithEmptyGuard("drainer", in, drainer)
		if got != in {
			t.Errorf("guard should keep multiline original: got %q", got)
		}
	})

	t.Run("empty_input_passes_through", func(t *testing.T) {
		got := sanitizeWithEmptyGuard("drainer", "", drainer)
		if got != "" {
			t.Errorf("empty input should pass: got %q", got)
		}
	})

	t.Run("whitespace_only_passes_through", func(t *testing.T) {
		got := sanitizeWithEmptyGuard("drainer", "  \n  ", drainer)
		if got != "" {
			t.Errorf("whitespace should pass: got %q", got)
		}
	})

	t.Run("normal_transform_not_affected", func(t *testing.T) {
		got := sanitizeWithEmptyGuard("upper", "hi", upper)
		if got != "HI" {
			t.Errorf("normal transform should work: got %q", got)
		}
	})

	t.Run("passthrough_preserved", func(t *testing.T) {
		got := sanitizeWithEmptyGuard("passthrough", "abc", passthrough)
		if got != "abc" {
			t.Errorf("passthrough failed: got %q", got)
		}
	})
}
