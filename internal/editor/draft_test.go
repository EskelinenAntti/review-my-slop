package editor

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestCommentDraftSuggestionRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
		block string
	}{
		{
			name:  "context and additions",
			lines: []string{" context", "-removed", "+added"},
			block: "```suggestion\ncontext\nadded\n```",
		},
		{
			name:  "deletions only",
			lines: []string{"-removed", "-``````"},
			block: "```suggestion\n```",
		},
		{
			name:  "empty quoted entries and source lines",
			lines: []string{"", " ", "+", ""},
			block: "```suggestion\n\n\n```",
		},
		{
			name:  "source diff markers are preserved",
			lines: []string{"++increment", "+-negative", " --context"},
			block: "```suggestion\n+increment\n-negative\n--context\n```",
		},
		{
			name:  "longest retained backtick run sets fence",
			lines: []string{"-````````", "+a````b", " ```"},
			block: "`````suggestion\na````b\n```\n`````",
		},
		{
			name:  "backtick runs do not span lines",
			lines: []string{"+``", "+``"},
			block: "```suggestion\n``\n``\n```",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			anchor := comments.Anchor{QuotedLines: tc.lines}
			want := "body\n\n" + tc.block + "\n"
			draft := commentDraft("body", anchor)
			if draft != want {
				t.Fatalf("draft = %q, want %q", draft, want)
			}
			if got := stripUnchangedSuggestion(draft, tc.lines); got != "body" {
				t.Fatalf("unchanged draft body = %q, want body", got)
			}
		})
	}
}

func TestCommentDraftSpacing(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		quoted     []string
		want       string
	}{
		{
			name: "no quoted lines",
			body: "body\n\n",
			want: "body\n\n",
		},
		{
			name:   "empty body",
			quoted: []string{"+new"},
			want:   "\n```suggestion\nnew\n```\n",
		},
		{
			name:   "body without newline",
			body:   "body",
			quoted: []string{"+new"},
			want:   "body\n\n```suggestion\nnew\n```\n",
		},
		{
			name:   "body with newline",
			body:   "body\n",
			quoted: []string{"+new"},
			want:   "body\n\n```suggestion\nnew\n```\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := commentDraft(
				tc.body, comments.Anchor{QuotedLines: tc.quoted},
			)
			if got != tc.want {
				t.Fatalf("draft = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStripUnchangedSuggestionPreservesFeedback(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		quoted     []string
		want       string
	}{
		{
			name: "no quoted lines",
			body: "body\n\n```suggestion\nnew\n```\n",
			want: "body\n\n```suggestion\nnew\n```\n",
		},
		{
			name:   "suggestion removed by editor",
			body:   "body\n\n",
			quoted: []string{"+new"},
			want:   "body\n\n",
		},
		{
			name:   "edited suggestion",
			body:   "body\n\n```suggestion\nbetter\n```\n",
			quoted: []string{"+new"},
			want:   "body\n\n```suggestion\nbetter\n```\n",
		},
		{
			name:   "trailing whitespace only",
			body:   "body\n\n```suggestion\nnew\n```\n \t\n",
			quoted: []string{"+new"},
			want:   "body",
		},
		{
			name:   "feedback after suggestion",
			body:   "before\n\n```suggestion\nnew\n```\n\nafter\n",
			quoted: []string{"+new"},
			want:   "before\nafter\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := stripUnchangedSuggestion(tc.body, tc.quoted)
			if got != tc.want {
				t.Fatalf("body = %q, want %q", got, tc.want)
			}
		})
	}
}
