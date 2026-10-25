package editor

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestSuggestionBlock(t *testing.T) {
	tests := []struct {
		name   string
		quoted []string
		want   string
	}{
		{name: "no context"},
		{
			name:   "new version only",
			quoted: []string{" context", "-old", "+new", ""},
			want:   "```suggestion\ncontext\nnew\n```",
		},
		{
			name:   "deletion only",
			quoted: []string{"-old"},
			want:   "```suggestion\n```",
		},
		{
			name:   "blank added line",
			quoted: []string{"+"},
			want:   "```suggestion\n\n```",
		},
		{
			name:   "fence follows included lines",
			quoted: []string{"-``````", "+````"},
			want:   "`````suggestion\n````\n`````",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := suggestionBlock(tt.quoted); got != tt.want {
				t.Fatalf("block = %q, want %q", got, tt.want)
			}
			anchor := comments.Anchor{QuotedLines: tt.quoted}
			draft := commentDraft("body", anchor)
			got := stripUnchangedSuggestion(draft, tt.quoted)
			if got != "body" {
				t.Fatalf("round trip = %q, want body", got)
			}
		})
	}
}
