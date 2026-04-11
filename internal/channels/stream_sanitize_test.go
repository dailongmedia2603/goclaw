package channels

import "testing"

// streamTestCase describes a case for sanitizeStreamBuffer.
//
// bugFlagged = true means the expected value reflects a known bug in the
// current text-based Reasoning: stripping heuristic. Phase 4 of the
// "remove-reasoning-text-heuristic" plan removes sanitizeStreamBuffer entirely;
// this test will be deleted then.
type streamTestCase struct {
	name       string
	in         string
	want       string
	bugFlagged bool
}

// TestSanitizeStreamBuffer_CurrentBehavior locks the current behavior of
// sanitizeStreamBuffer before it is removed in Phase 4. All cases must pass
// against the current implementation — bug-flagged cases document the
// behavior that will change after removal.
func TestSanitizeStreamBuffer_CurrentBehavior(t *testing.T) {
	cases := []streamTestCase{
		// Happy path — no "Reasoning:" prefix, passthrough
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "whitespace_only",
			in:   "   \n\n  ",
			want: "",
		},
		{
			name: "plain_text",
			in:   "Hello world",
			want: "Hello world",
		},
		{
			name: "markdown_bullets_no_prefix",
			in:   "- A\n- B\n- C",
			want: "- A\n- B\n- C",
		},
		{
			name: "numbered_list_no_prefix",
			in:   "1. one\n2. two",
			want: "1. one\n2. two",
		},
		{
			name: "vietnamese_text",
			in:   "Chào bạn, đây là câu trả lời đầy đủ",
			want: "Chào bạn, đây là câu trả lời đầy đủ",
		},

		// Reasoning prefix cases
		{
			name: "reasoning_pure_bullets_drops_to_empty",
			in:   "Reasoning:\n• step one\n• step two",
			want: "",
		},
		{
			name: "reasoning_then_plain_text_ok",
			in:   "Reasoning:\n• think\n\nYes absolutely.",
			want: "Yes absolutely.",
		},
		{
			name:       "reasoning_then_dash_bullets_BUG",
			in:         "Reasoning:\n• think\n\n- First\n- Second",
			want:       "",
			bugFlagged: true,
		},
		{
			name:       "reasoning_then_star_bullets_BUG",
			in:         "Reasoning:\n• think\n\n* Item 1\n* Item 2",
			want:       "",
			bugFlagged: true,
		},
		{
			name:       "reasoning_incomplete_partial_stream_BUG",
			in:         "Reasoning:\n• analyzing",
			want:       "",
			bugFlagged: true,
		},
		{
			name: "reasoning_then_vietnamese_text_ok",
			in:   "Reasoning:\n• hmm\n\nChào bạn, đây là câu trả lời",
			want: "Chào bạn, đây là câu trả lời",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeStreamBuffer(tc.in)
			if got != tc.want {
				if tc.bugFlagged {
					t.Errorf("[BUG-FLAGGED] sanitizeStreamBuffer(%q)\nwant=%q\ngot=%q\n(Phase 4 will remove this function)",
						tc.in, tc.want, got)
				} else {
					t.Errorf("sanitizeStreamBuffer(%q)\nwant=%q\ngot=%q", tc.in, tc.want, got)
				}
			}
		})
	}
}

// TestSplitThinkTags_StructuralRemainsIntact verifies that the <think> XML
// tag parser (used as fallback for providers that don't natively separate
// reasoning) continues to work correctly. This path is NOT removed in Phase 4.
func TestSplitThinkTags_StructuralRemainsIntact(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantThink string
		wantAns  string
		wantPartial bool
	}{
		{
			name:      "no_tags",
			in:        "plain answer",
			wantThink: "",
			wantAns:   "plain answer",
		},
		{
			name:      "complete_think_tag",
			in:        "<think>inner</think>answer",
			wantThink: "inner",
			wantAns:   "answer",
		},
		{
			name:        "partial_unclosed_think",
			in:          "<think>still thinking",
			wantThink:   "still thinking",
			wantAns:     "",
			wantPartial: true,
		},
		{
			name:      "thinking_variant",
			in:        "<thinking>x</thinking>y",
			wantThink: "x",
			wantAns:   "y",
		},
		{
			name:      "thought_variant",
			in:        "<thought>x</thought>y",
			wantThink: "x",
			wantAns:   "y",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitThinkTags(tc.in)
			if got.Thinking != tc.wantThink {
				t.Errorf("Thinking = %q, want %q", got.Thinking, tc.wantThink)
			}
			if got.Answer != tc.wantAns {
				t.Errorf("Answer = %q, want %q", got.Answer, tc.wantAns)
			}
			if got.Partial != tc.wantPartial {
				t.Errorf("Partial = %v, want %v", got.Partial, tc.wantPartial)
			}
		})
	}
}
