package comments

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestWritePromptExactOutput(t *testing.T) {
	tests := []struct {
		name     string
		comments []Comment
		want     string
	}{
		{name: "empty", want: "No pending review comments.\n"},
		{
			name: "numbered comments with and without context",
			comments: []Comment{
				{
					Anchor: Anchor{
						FilePath:    "main.go",
						OldStart:    2,
						OldEnd:      3,
						NewStart:    4,
						QuotedLines: []string{"-old", "+new"},
					},
					Body: "  first comment\n\n",
				},
				{Anchor: Anchor{FilePath: "other.go"}, Body: "second comment"},
			},
			want: "New comments since last run:\n" +
				"\n### 1. `main.go` (old lines 2-3, new line 4)\n\n" +
				"```diff\n-old\n+new\n```\nfirst comment\n" +
				"\n### 2. `other.go` (diff lines)\n\nsecond comment\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			testutil.NoError(t, WritePrompt(&output, tt.comments))
			testutil.CheckEqual(t, output.String(), tt.want,
				"output = %q, want %q", output.String(), tt.want)
		})
	}
}

func TestWritePendingPreservesCommentsOnOutputFailure(t *testing.T) {
	for _, limit := range []int{0, 40} {
		t.Run(fmt.Sprintf("after %d bytes", limit), func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			saved, err := store.Add(testComment("/repo", "pending feedback"))
			testutil.NoError(t, err)
			writeErr := errors.New("output failed")
			writer := &limitedPromptWriter{remaining: limit, err: writeErr}
			err = store.WritePending(writer, "/repo")
			if !errors.Is(err, writeErr) {
				t.Fatalf("WritePending error = %v, want %v", err, writeErr)
			}
			pending, err := store.List("/repo")
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(pending), 1, "pending = %#v", pending)
			testutil.CheckEqual(t, pending[0].ID, saved.ID,
				"pending ID = %q, want %q", pending[0].ID, saved.ID)
		})
	}
}

type limitedPromptWriter struct {
	remaining int
	err       error
}

func (w *limitedPromptWriter) Write(data []byte) (int, error) {
	count := min(w.remaining, len(data))
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
