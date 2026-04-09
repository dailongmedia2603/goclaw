package agent

import "testing"

func TestRedactSensitiveTerms(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		terms       []string
		replacement string
		want        string
	}{
		{
			name:    "no terms - passthrough",
			content: "Hello GoClaw world",
			terms:   nil,
			want:    "Hello GoClaw world",
		},
		{
			name:    "empty content",
			content: "",
			terms:   []string{"GoClaw"},
			want:    "",
		},
		{
			name:        "exact match",
			content:     "Framework của em là GoClaw.",
			terms:       []string{"GoClaw"},
			replacement: "hệ thống AI",
			want:        "Framework của em là hệ thống AI.",
		},
		{
			name:        "case insensitive",
			content:     "Em chạy trên GOCLAW và goclaw rất tốt.",
			terms:       []string{"GoClaw"},
			replacement: "hệ thống AI",
			want:        "Em chạy trên hệ thống AI và hệ thống AI rất tốt.",
		},
		{
			name:        "multiple terms",
			content:     "GoClaw là fork của OpenClaw framework.",
			terms:       []string{"GoClaw", "OpenClaw"},
			replacement: "hệ thống AI",
			want:        "hệ thống AI là fork của hệ thống AI framework.",
		},
		{
			name:        "empty replacement - strips term",
			content:     "Em dùng GoClaw để chạy.",
			terms:       []string{"GoClaw"},
			replacement: "",
			want:        "Em dùng  để chạy.",
		},
		{
			name:        "no match - passthrough",
			content:     "Em là trợ lý AI thông minh.",
			terms:       []string{"GoClaw", "OpenClaw"},
			replacement: "xxx",
			want:        "Em là trợ lý AI thông minh.",
		},
		{
			name:        "term in markdown bold",
			content:     "Em vận hành trên **GoClaw** framework.",
			terms:       []string{"GoClaw"},
			replacement: "hệ thống",
			want:        "Em vận hành trên **hệ thống** framework.",
		},
		{
			name:        "term in code block",
			content:     "```\nGoClaw server running\n```",
			terms:       []string{"GoClaw"},
			replacement: "",
			want:        "```\n server running\n```",
		},
		{
			name:        "overlapping replacements",
			content:     "GoClaw GoClaw GoClaw",
			terms:       []string{"GoClaw"},
			replacement: "X",
			want:        "X X X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactSensitiveTerms(tt.content, tt.terms, tt.replacement)
			if got != tt.want {
				t.Errorf("RedactSensitiveTerms() = %q, want %q", got, tt.want)
			}
		})
	}
}
