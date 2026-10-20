package editor

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestSuggestionDraftRoundTrip(t *testing.T) {
	for _, tt := range []struct {
		name   string
		quoted []string
		want   string
	}{
		{
			name: "no quoted lines",
			want: "body",
		},
		{
			name:   "only deletions",
			quoted: []string{"-old()"},
			want:   "body\n\n```suggestion\n```\n",
		},
		{
			name:   "context and additions",
			quoted: []string{" unchanged()", "-old()", "+new()"},
			want:   "body\n\n```suggestion\nunchanged()\nnew()\n```\n",
		},
		{
			name:   "empty entries and blank lines",
			quoted: []string{"", " ", "+", "-"},
			want:   "body\n\n```suggestion\n\n\n```\n",
		},
		{
			name:   "deleted fences do not affect delimiter",
			quoted: []string{"-``````", "+new()"},
			want:   "body\n\n```suggestion\nnew()\n```\n",
		},
		{
			name:   "retained fences lengthen delimiter",
			quoted: []string{" ````go", "+```", " ````"},
			want: "body\n\n`````suggestion\n" +
				"````go\n```\n````\n`````\n",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			anchor := comments.Anchor{QuotedLines: tt.quoted}
			draft := commentDraft("body", anchor)
			if draft != tt.want {
				t.Fatalf("draft = %q, want %q", draft, tt.want)
			}
			got := stripUnchangedSuggestion(draft, tt.quoted)
			if got != "body" {
				t.Fatalf("body = %q, want %q", got, "body")
			}
		})
	}
}
