package channels

import "testing"

// TestSplitThinkTags_StructuralRemainsIntact verifies that the <think> XML
// tag parser (used as fallback for providers that don't natively separate
// reasoning) works correctly. This path is the structural fallback that
// remains after removal of the text-based "Reasoning:" heuristic.
func TestSplitThinkTags_StructuralRemainsIntact(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		wantThink   string
		wantAns     string
		wantPartial bool
	}{
		{
			name:      "no_tags",
			in:        "plain answer",
			wantThink: "",
			wantAns:   "plain answer",
		},
		{
			name:      "empty_input",
			in:        "",
			wantThink: "",
			wantAns:   "",
		},
		{
			name:      "complete_think_tag",
			in:        "<think>inner</think>answer",
			wantThink: "inner",
			wantAns:   "answer",
		},
		{
			name:      "complete_think_tag_with_newline",
			in:        "<think>\nstep 1\nstep 2\n</think>\nThe answer",
			wantThink: "\nstep 1\nstep 2\n",
			wantAns:   "\nThe answer",
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
		{
			name:      "antthinking_variant",
			in:        "<antThinking>x</antThinking>y",
			wantThink: "x",
			wantAns:   "y",
		},
		{
			name:      "case_insensitive_tags",
			in:        "<THINK>x</THINK>y",
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
