package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommentsWritePendingAcknowledgesOnlyExportedComments(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	for _, comment := range []commentsComment{
		feedbackOutputComment("exported", "/repo"),
		feedbackOutputComment("other repository", "/other"),
	} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	writer := &commentsAddingWriter{Buffer: &output, add: func() {
		added := feedbackOutputComment("added during output", "/repo")
		if _, err := store.Add(added); err != nil {
			t.Fatal(err)
		}
	}}
	requireNoError(t, store.WritePending(writer, "/repo"))
	requireContains(t, output.String(), "exported")
	requireNotContains(t, output.String(), "other repository")
	requireNotContains(t, output.String(), "added during output")
	requirePendingBody(t, store, "/repo", "added during output")
	requirePendingBody(t, store, "/other", "other repository")
}

func feedbackOutputComment(id, repository string) commentsComment {
	return commentsComment{
		ID: id, Repository: repository,
		CreatedAt: time.Unix(1, 0).UTC(),
		Anchor:    commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1},
		Body:      id,
	}
}

func requireContains(t *testing.T, text, want string) {
	t.Helper()
	requireEqual(t, strings.Contains(text, want), true, "text = %q", text)
}

func requireNotContains(t *testing.T, text, unwanted string) {
	t.Helper()
	requireEqual(t, strings.Contains(text, unwanted), false, "text = %q", text)
}

func requirePendingBody(
	t *testing.T,
	store commentsStore,
	repository string,
	want string,
) {
	t.Helper()
	pending, err := store.List(repository)
	requireNoError(t, err)
	requireEqual(t, len(pending), 1, "%s pending = %#v", repository, pending)
	requireEqual(
		t,
		pending[0].Body,
		want,
		"%s pending = %#v",
		repository,
		pending,
	)
}

type commentsAddingWriter struct {
	*bytes.Buffer
	add func()
}

func (w *commentsAddingWriter) Write(data []byte) (int, error) {
	if w.add != nil {
		w.add()
		w.add = nil
	}
	return w.Buffer.Write(data)
}
