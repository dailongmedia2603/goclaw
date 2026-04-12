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

		// --- Group B2: Answer tag extraction (additive approach) ---
		// extractAnswerTag extracts ONLY content inside <answer>...</answer>,
		// discarding all reasoning/meta content outside.
		{
			name:  "answer_tag_basic",
			input: "Reasoning:\nthinking...\n<answer>Hello world!</answer>",
			want:  "Hello world!",
		},
		{
			name:  "answer_tag_with_thought_tags",
			input: "<thought>analysis</thought>\n<answer>Clean response here.</answer>",
			want:  "Clean response here.",
		},
		{
			name:  "answer_tag_multiline",
			input: "Reasoning:\nstuff\n<answer>\nLine 1.\n\nLine 2.\n</answer>",
			want:  "Line 1.\n\nLine 2.",
		},
		{
			name:  "answer_tag_no_reasoning_passthrough",
			input: "<answer>Just a clean answer.</answer>",
			want:  "Just a clean answer.",
		},
		{
			// No <answer> tag → falls through to existing stripping logic.
			name:  "no_answer_tag_falls_through",
			input: "Reasoning:\nthought\n\nActual answer here.",
			want:  "Actual answer here.",
		},

		// --- Group C2: Gemma "Reasoning: ... </thought>" pattern ---
		// Gemma-4-31b-it uses "Reasoning:" as opening delimiter and orphaned
		// </thought> as closing delimiter, with NO opening <thought> tag. The
		// reasoningToCloseTagPattern handles this in stripThinkingTags.
		{
			name: "gemma_reasoning_thought_close_single_para",
			input: "Reasoning:\nThe user asked X.\n</thought>\nHere is the answer.",
			want:  "Here is the answer.",
		},
		{
			name: "gemma_reasoning_thought_close_multi_para",
			input: "Reasoning:\nThe user sent /start.\n\n" +
				"According to SOUL.md I should be casual.\n\n" +
				"I should avoid robotic phrases.\n</thought>" +
				"Chào bạn, mình đây!",
			want: "Chào bạn, mình đây!",
		},
		{
			name: "gemma_reasoning_thought_close_no_space_after_tag",
			input: "Reasoning:\nthought process here.</thought>Actual answer starts here.",
			want:  "Actual answer starts here.",
		},
		{
			name: "gemma_reasoning_think_close_variant",
			input: "Reasoning:\nanalysis.\n</think>\nFinal answer.",
			want:  "Final answer.",
		},
		{
			name: "gemma_thinking_thought_close",
			input: "Thinking:\nstep by step.\n</thought>\nDone.",
			want:  "Done.",
		},
		{
			// No closing tag at all — falls through to step 3b
			// (stripLeadingReasoningBlock) which uses blank-line fallback.
			name: "gemma_reasoning_no_close_tag_falls_through",
			input: "Reasoning:\nquick thought\n\nActual answer here.",
			want:  "Actual answer here.",
		},

		// --- Group D: Leading Reasoning/Thinking header (narrow stripper, 2026-04-12) ---
		// stripLeadingReasoningBlock removes a "Reasoning:"/"Thinking:" paragraph at
		// the very start of the response, up to the first blank-line separator.
		// Inputs WITHOUT a "\n\n" separator are preserved untouched — this is the
		// safety rule that prevents the original over-matching bug.
		{
			// No blank-line separator — pure bullet list must be preserved.
			name:  "reasoning_pure_bullets_preserved",
			input: "Reasoning:\n• step one\n• step two",
			want:  "Reasoning:\n• step one\n• step two",
		},
		{
			name:  "reasoning_then_dash_bullet_answer_stripped",
			input: "Reasoning:\n• think\n\n- First\n- Second",
			want:  "- First\n- Second",
		},
		{
			name:  "reasoning_then_star_bullet_answer_stripped",
			input: "Reasoning:\n• think\n\n* Item 1\n* Item 2",
			want:  "* Item 1\n* Item 2",
		},
		{
			// Note: the final-pipeline strings.TrimSpace strips leading whitespace of
			// the first remaining line, so a single-line indented answer comes out
			// un-indented. Multi-line answers keep inner indentation (see next case).
			name:  "reasoning_then_indented_answer_stripped",
			input: "Reasoning:\n• think\n\n    code block line",
			want:  "code block line",
		},
		{
			// Multi-line answer: first-line indent is trimmed by final TrimSpace but
			// inner-line indentation is preserved — important for nested lists / code.
			name:  "reasoning_then_multiline_indent_preserved",
			input: "Reasoning:\nwhy\n\n    line1\n        line2\n        line3",
			want:  "line1\n        line2\n        line3",
		},
		{
			name:  "reasoning_then_plain_text_answer_stripped",
			input: "Reasoning:\n• think\n\nYes absolutely.",
			want:  "Yes absolutely.",
		},
		{
			name:  "reasoning_then_vietnamese_text_answer_stripped",
			input: "Reasoning:\n• hmm\n\nChào bạn, đây là câu trả lời",
			want:  "Chào bạn, đây là câu trả lời",
		},
		{
			name:  "no_reasoning_prefix_passthrough",
			input: "Just a plain reply",
			want:  "Just a plain reply",
		},
		// --- Additional coverage for stripLeadingReasoningBlock ---
		{
			// Case-insensitive header match.
			name:  "reasoning_header_uppercase_stripped",
			input: "REASONING:\nchain of thought\n\nFinal answer here",
			want:  "Final answer here",
		},
		{
			// "Thinking:" variant header.
			name:  "thinking_header_variant_stripped",
			input: "Thinking:\ndeliberating steps\n\nHere you go",
			want:  "Here you go",
		},
		{
			// Inline text on the same line as the header.
			name:  "reasoning_inline_after_colon_stripped",
			input: "Reasoning: The user asked about X.\n\nActual answer",
			want:  "Actual answer",
		},
		{
			// Multiple blank lines between reasoning and answer collapse to none.
			name:  "reasoning_extra_blank_lines_normalized",
			input: "Reasoning:\nstuff\n\n\n\nReal answer",
			want:  "Real answer",
		},
		{
			// Single-line "Reasoning:" mention with no separator must not be touched.
			name:  "reasoning_single_line_no_separator_preserved",
			input: "Reasoning: single-line mention only",
			want:  "Reasoning: single-line mention only",
		},
		{
			// Word "reasoning" appearing mid-response (no leading header) untouched.
			name:  "reasoning_word_midsentence_preserved",
			input: "Hello! Let me explain my reasoning: it was obvious.",
			want:  "Hello! Let me explain my reasoning: it was obvious.",
		},
		// --- Group D2: Multi-paragraph reasoning with answer-start markers ---
		// Gemma via Google AI Studio emits chain-of-thought as several
		// paragraphs followed by a "Drafting the response:" / "Response:" /
		// "Trả lời:" style marker before the actual answer. The marker
		// is the reliable boundary; the "first blank line" heuristic does
		// not work for these cases.
		{
			name: "reasoning_multi_para_drafting_response_marker",
			input: "Reasoning:\nThe user sent /start.\n\n" +
				"According to the persona I should be casual.\n\n" +
				"I should avoid robotic phrases.\n\n" +
				"Drafting the response:\nChào bạn, mình đây!",
			want: "Chào bạn, mình đây!",
		},
		{
			name: "reasoning_multi_para_vietnamese_tra_loi_marker",
			input: "Reasoning:\nPhân tích câu hỏi của user.\n\n" +
				"User đang hỏi về phonefarm.\n\n" +
				"Trả lời:\nPhonefarm là hệ thống nuôi nhiều điện thoại.",
			want: "Phonefarm là hệ thống nuôi nhiều điện thoại.",
		},
		{
			name: "reasoning_multi_para_bold_response_marker",
			input: "Reasoning:\nstep 1 analysis\n\n" +
				"step 2 planning\n\n" +
				"**Response:**\nHere is the actual answer.",
			want: "Here is the actual answer.",
		},
		{
			name: "reasoning_nested_markers_uses_last",
			input: "Reasoning:\nfirst thought\n\n" +
				"Response: considering options\n\n" +
				"Final answer: 42",
			want: "42",
		},
		{
			name: "reasoning_draft_marker_with_inline_content",
			input: "Reasoning:\nsome analysis\n\n" +
				"Draft response: this is my answer",
			want: "this is my answer",
		},
		{
			// Marker found but produces empty result — fall through to blank-line
			// fallback so we don't return an empty string mid-function.
			name: "reasoning_marker_followed_by_empty_falls_back",
			input: "Reasoning:\nquick thought\n\n" +
				"The rest of the response here.",
			want: "The rest of the response here.",
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
