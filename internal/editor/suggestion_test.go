package editor

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSuggestionBlock(t *testing.T) {
	tests := []struct {
		name   string
		quoted []string
		want   string
	}{
		{
			"new source", []string{" context", "-old", "+new"},
			"```suggestion\ncontext\nnew\n```",
		},
		{"deletions only", []string{"-old"}, "```suggestion\n```"},
		{"blank lines", []string{"", "+", " "}, "```suggestion\n\n\n```"},
		{
			"deleted fence", []string{"-`````", "+new"},
			"```suggestion\nnew\n```",
		},
		{
			"embedded fences", []string{"+a``b```c", " ````"},
			"`````suggestion\na``b```c\n````\n`````",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := slices.Clone(tt.quoted)
			testutil.RequireEqual(t, suggestionBlock(tt.quoted), tt.want,
				"suggestion block mismatch")
			testutil.RequireEqual(t, slices.Equal(tt.quoted, original), true,
				"quoted lines changed: %q", tt.quoted)
			requireSuggestionRoundTrip(t, tt.quoted)
		})
	}
}

func requireSuggestionRoundTrip(t *testing.T, quoted []string) {
	t.Helper()
	anchor := comments.Anchor{QuotedLines: quoted}
	for _, body := range []string{"", "feedback"} {
		draft := commentDraft(body, anchor)
		if got := stripUnchangedSuggestion(draft, quoted); got != body {
			t.Fatalf("round trip for %q = %q", body, got)
		}
	}
}

func TestStripSuggestionPreservesFeedback(t *testing.T) {
	quoted := []string{"+new"}
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			"following feedback",
			"before\n\n```suggestion\nnew\n```\n\nafter\n", "before\nafter\n",
		},
		{
			"edited suggestion",
			"```suggestion\nedited\n```\n", "```suggestion\nedited\n```\n",
		},
		{"removed suggestion", "feedback\n", "feedback\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripUnchangedSuggestion(tt.body, quoted)
			if got != tt.want {
				t.Fatalf("stripped body = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDraftWithoutQuotedLines(t *testing.T) {
	const body = "feedback\n\n```suggestion\nuser content\n```\n"
	if got := commentDraft(body, comments.Anchor{}); got != body {
		t.Fatalf("commentDraft() = %q, want %q", got, body)
	}
	if got := stripUnchangedSuggestion(body, nil); got != body {
		t.Fatalf("stripUnchangedSuggestion() = %q, want %q", got, body)
	}
}
