package comments

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
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
	if err := store.WritePending(writer, "/repo"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "exported") || strings.Contains(output.String(), "other repository") || strings.Contains(output.String(), "added during output") {
		t.Fatalf("output = %q", output.String())
	}
	for repository, want := range map[string]string{"/repo": "added during output", "/other": "other repository"} {
		pending, err := store.List(repository)
		if err != nil {
			t.Fatal(err)
		}
		if len(pending) != 1 || pending[0].Body != want {
			t.Fatalf("%s pending = %#v", repository, pending)
		}
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
