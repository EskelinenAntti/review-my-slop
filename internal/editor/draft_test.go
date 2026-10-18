package editor

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSuggestionBlockUsesNewSideAndSafeFence(t *testing.T) {
	for _, tt := range []struct {
		name   string
		quoted []string
		want   string
	}{
		{"empty", nil, "```suggestion\n```"},
		{"deletions", []string{"-old", "-`````"}, "```suggestion\n```"},
		{"blank lines", []string{"", "+", " "}, "```suggestion\n\n\n```"},
		{"mixed", []string{" old", "-gone", "+new"},
			"```suggestion\nold\nnew\n```"},
		{"backticks", []string{"+````", "+new"},
			"`````suggestion\n````\nnew\n`````"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := suggestionBlock(tt.quoted)
			testutil.RequireEqual(t, got, tt.want,
				"suggestion = %q, want %q", got, tt.want,
			)
		})
	}
}
