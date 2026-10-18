package comments

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestWritePromptCommentFormatting(t *testing.T) {
	for _, tt := range []struct {
		name   string
		quoted []string
		want   string
	}{
		{"without quote", nil, "feedback\n"},
		{"with quote", []string{"-old", "", "+new"},
			"```diff\n-old\n\n+new\n```\nfeedback\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			err := writePromptComment(&output, 2, Comment{
				Anchor: Anchor{FilePath: "file.go", QuotedLines: tt.quoted},
				Body:   " \nfeedback\n ",
			})
			testutil.NoError(t, err)
			want := "\n### 2. `file.go` (diff lines)\n\n" + tt.want
			testutil.RequireEqual(t, output.String(), want,
				"output = %q, want %q", output.String(), want,
			)
		})
	}
}

func TestWritePendingPreservesCommentsWhenCommentOutputFails(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	_, err := store.Add(testComment("/repo", "keep feedback"))
	testutil.NoError(t, err)
	wantErr := errors.New("output failed")
	writer := &failAfterHeadingWriter{err: wantErr}
	err = store.WritePending(writer, "/repo")
	testutil.RequireEqual(t, errors.Is(err, wantErr), true,
		"error = %v, want %v", err, wantErr,
	)
	pending, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(pending), 1,
		"pending comments = %#v", pending,
	)
	testutil.RequireEqual(t, pending[0].Body, "keep feedback",
		"pending comment = %#v", pending[0],
	)
}

type failAfterHeadingWriter struct {
	writes int
	err    error
}

func (w *failAfterHeadingWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(data), nil
	}
	return 0, w.err
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
