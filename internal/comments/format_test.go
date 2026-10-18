package comments

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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

func TestWritePrompt(t *testing.T) {
	var out bytes.Buffer
	comment := testComment("/repo", "Handle the nil case.")
	comment.Anchor = Anchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	testutil.NoError(t, WritePrompt(&out, []Comment{comment}))

	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		testutil.RequireEqual(t, strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s", expected, out.String(),
		)
	}
	testutil.RequireEqual(t, !strings.Contains(out.String(), "batch"), true,
		"output exposes internal batches:\n%s", out.String(),
	)
}

func TestWritePromptNamesPatchWhenRangeIsUnavailable(t *testing.T) {
	var output bytes.Buffer
	err := WritePrompt(&output, []Comment{{
		Anchor: Anchor{FilePath: "main.go"},
		Body:   "check this",
	}})
	testutil.NoError(t, err)
	testutil.RequireEqual(t,
		strings.Contains(output.String(), "`main.go` (Patch lines)"), true,
		"output = %q", output.String(),
	)
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
