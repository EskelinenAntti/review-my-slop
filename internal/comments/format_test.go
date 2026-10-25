package comments

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestWritePromptFormatting(t *testing.T) {
	for _, test := range []struct {
		name     string
		comments []Comment
		want     string
	}{
		{name: "empty", want: "No pending review comments.\n"},
		{
			name: "quoted and unquoted comments",
			comments: []Comment{
				{
					Anchor: Anchor{
						FilePath: "main.go", OldStart: 2, OldEnd: 3,
						NewStart: 4, NewEnd: 4,
						QuotedLines: []string{"-old", "+new"},
					},
					Body: "  first comment\n ",
				},
				{Anchor: Anchor{FilePath: "other.go"}, Body: "second comment"},
			},
			want: "New comments since last run:\n" +
				"\n### 1. `main.go` (old lines 2-3, new line 4)\n\n" +
				"```diff\n-old\n+new\n```\nfirst comment\n" +
				"\n### 2. `other.go` (diff lines)\n\nsecond comment\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			testutil.NoError(t, WritePrompt(&output, test.comments))
			testutil.RequireEqual(t, output.String(), test.want,
				"output = %q", output.String())
		})
	}
}

func TestWritePendingRetainsCommentsOnPartialOutput(t *testing.T) {
	for _, limit := range []int{0, len("New comments since last run:\n"), 50} {
		store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
		_, err := store.Add(testComment("/repo", "pending comment"))
		testutil.NoError(t, err)
		failure := errors.New("output failed")
		writer := &failingPromptWriter{remaining: limit, err: failure}
		err = store.WritePending(writer, "/repo")
		if !errors.Is(err, failure) {
			t.Fatalf("limit %d: error = %v, want %v", limit, err, failure)
		}
		pending, err := store.List("/repo")
		testutil.NoError(t, err)
		testutil.RequireEqual(t, len(pending), 1,
			"limit %d: pending = %#v", limit, pending)
	}
}

type failingPromptWriter struct {
	remaining int
	err       error
}

func (w *failingPromptWriter) Write(data []byte) (int, error) {
	count := min(len(data), w.remaining)
	w.remaining -= count
	if count < len(data) {
		return count, w.err
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
