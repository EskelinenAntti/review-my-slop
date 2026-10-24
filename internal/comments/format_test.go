package comments

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestWritePromptMarkdown(t *testing.T) {
	for _, tc := range []struct {
		name     string
		comments []Comment
		want     string
	}{
		{
			name: "empty",
			want: "No pending review comments.\n",
		},
		{
			name: "quoted and unquoted comments",
			comments: []Comment{
				{
					Anchor: Anchor{
						FilePath: "main.go",
						OldStart: 10, OldEnd: 11,
						NewStart: 12, NewEnd: 13,
						QuotedLines: []string{"-old()", "+new()"},
					},
					Body: "  Handle the nil case.\n",
				},
				{Anchor: Anchor{FilePath: "other.go"}, Body: "Second."},
			},
			want: "New comments since last run:\n" +
				"\n### 1. `main.go` (old lines 10-11, new lines 12-13)\n\n" +
				"```diff\n-old()\n+new()\n```\nHandle the nil case.\n" +
				"\n### 2. `other.go` (diff lines)\n\nSecond.\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			testutil.NoError(t, WritePrompt(&output, tc.comments))
			testutil.RequireEqual(t, output.String(), tc.want,
				"output = %q", output.String(),
			)
		})
	}
}

func TestWritePendingPreservesCommentsAfterOutputFailure(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	for _, body := range []string{"First.", "Second."} {
		if _, err := store.Add(testComment("/repo", body)); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := store.List("/repo")
	testutil.NoError(t, err)
	var output bytes.Buffer
	testutil.NoError(t, WritePrompt(&output, pending))
	for _, limit := range []int{0, output.Len() / 2, output.Len() - 1} {
		writer := &failingPromptWriter{remaining: limit}
		err := store.WritePending(writer, "/repo")
		testutil.RequireEqual(t, errors.Is(err, io.ErrClosedPipe), true,
			"limit %d: output error = %v", limit, err,
		)
		remaining, err := store.List("/repo")
		testutil.NoError(t, err)
		testutil.RequireEqual(t, len(remaining), len(pending),
			"limit %d: pending comments = %#v", limit, remaining,
		)
		for index, comment := range pending {
			testutil.RequireEqual(t, remaining[index].ID, comment.ID,
				"limit %d: pending comment changed", limit,
			)
		}
	}
}

type failingPromptWriter struct{ remaining int }

func (w *failingPromptWriter) Write(data []byte) (int, error) {
	count := min(len(data), w.remaining)
	w.remaining -= count
	if count < len(data) {
		return count, io.ErrClosedPipe
	}
	return count, nil
}

func TestWritePendingAcknowledgesOnlyExportedComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	for _, comment := range []Comment{
		testComment("/repo", "exported"),
		testComment("/other", "other repository"),
	} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	writer := &addingWriter{Buffer: &output, add: func() {
		added := testComment("/repo", "added during output")
		if _, err := store.Add(added); err != nil {
			t.Fatal(err)
		}
	}}
	testutil.NoError(t, store.WritePending(writer, "/repo"))
	testutil.RequireEqual(t, strings.Contains(output.String(), "exported"),
		true, "output = %q", output.String(),
	)
	testutil.RequireEqual(t,
		!strings.Contains(
			output.String(),
			"other repository",
		), true, "output = %q",
		output.String(),
	)
	testutil.RequireEqual(t,
		!strings.Contains(output.String(), "added during output"), true,
		"output = %q", output.String(),
	)

	for repository, want := range map[string]string{
		"/repo":  "added during output",
		"/other": "other repository",
	} {
		pending, err := store.List(repository)
		testutil.NoError(t, err)
		testutil.RequireEqual(t, len(pending), 1, "%s pending = %#v",
			repository, pending,
		)
		testutil.RequireEqual(t, pending[0].Body, want, "%s pending = %#v",
			repository, pending,
		)
	}
}

type addingWriter struct {
	*bytes.Buffer
	add func()
}

func (w *addingWriter) Write(data []byte) (int, error) {
	if w.add != nil {
		w.add()
		w.add = nil
	}
	return w.Buffer.Write(data)
}
