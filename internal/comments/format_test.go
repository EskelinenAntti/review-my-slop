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
		{
			_, err := store.Add(comment)
			testutil.Require(t, err == nil, err)
		}
	}
	var output bytes.Buffer
	writer := &addingWriter{Buffer: &output, add: func() {
		{
			_, err := store.Add(testComment("/repo", "added during output"))
			testutil.Require(t, err == nil, err)
		}
	}}

	testutil.NoError(t, store.WritePending(writer, "/repo"))

	testutil.Requiref(t, strings.Contains(output.String(), "exported") && !strings.Contains(output.String(), "other repository") && !strings.Contains(output.String(), "added during output"), "output = %q", output.String())
	for repository, want := range map[string]string{"/repo": "added during output", "/other": "other repository"} {
		pending, err := store.List(repository)
		testutil.Require(t, err == nil, err)
		testutil.Requiref(t, len(pending) == 1 && pending[0].Body == want, "%s pending = %#v", repository, pending)
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
