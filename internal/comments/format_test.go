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
	for _, comment := range []Comment{testComment("/repo", "exported"), testComment("/other", "other repository")} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	writer := &addingWriter{Buffer: &output, add: func() {
		if _, err := store.Add(testComment("/repo", "added during output")); err != nil {
			t.Fatal(err)
		}
	}}
	testutil.NoError(t, store.WritePending(writer, "/repo"))
	testutil.RequireEqual(t, strings.Contains(output.String(), "exported"), true, "output = %q", output.String())
	testutil.RequireEqual(t, !strings.Contains(output.String(), "other repository"), true, "output = %q", output.String())
	testutil.RequireEqual(t, !strings.Contains(output.String(), "added during output"), true, "output = %q", output.String())

	for repository, want := range map[string]string{"/repo": "added during output", "/other": "other repository"} {
		pending, err := store.List(repository)
		testutil.NoError(t, err)
		testutil.RequireEqual(t, len(pending), 1, "%s pending = %#v", repository, pending)
		testutil.RequireEqual(t, pending[0].Body, want, "%s pending = %#v", repository, pending)
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
