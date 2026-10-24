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
