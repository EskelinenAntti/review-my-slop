package editor

import (
	"errors"
	"os"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestEditCommentDraft(t *testing.T) {
	sentinel := errors.New("edit failed")
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "successful edit", want: "edited"},
		{name: "failed edit", err: sentinel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			anchor := comments.Anchor{
				QuotedLines: []string{" context", "+added"},
			}
			var draftPath string
			edit := func(path string) error {
				draftPath = path
				data, err := os.ReadFile(path)
				testutil.NoError(t, err)
				testutil.RequireEqual(t, string(data),
					commentDraft("original", anchor),
					"initial draft = %q", data)
				testutil.NoError(t, os.WriteFile(path,
					[]byte(commentDraft("edited", anchor)), 0o600))
				return tc.err
			}
			body, err := editCommentDraft("original", anchor, edit)
			testutil.RequireEqual(t, errors.Is(err, tc.err), true,
				"error = %v", err)
			testutil.RequireEqual(t, body, tc.want, "body = %q", body)
			testutil.RequireEqual(t, draftPath != "", true,
				"edit was not called")
			_, err = os.Stat(draftPath)
			testutil.RequireEqual(t, errors.Is(err, os.ErrNotExist), true,
				"draft still exists: %v", err)
		})
	}
}

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
