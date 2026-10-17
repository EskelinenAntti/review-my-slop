package main

import (
	bytes "bytes"
	filepath "path/filepath"
	strings "strings"
	testing "testing"
	time "time"
)

func TestCommentsWritePendingAcknowledgesOnlyExportedComments(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	for _, comment := range []commentsComment{
		func() commentsComment {
			var body string = "exported"
			return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
		}(),
		func() commentsComment {
			var body string = "other repository"
			return commentsComment{ID: body, Repository: "/other", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
		}(),
	} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	writer := &commentsAddingWriter{Buffer: &output, add: func() {
		added := func() commentsComment {
			var body string = "added during output"
			return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
		}()
		if _, err := store.Add(added); err != nil {
			t.Fatal(err)
		}
	}}
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, store.WritePending(writer, "/repo"))
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(output.String(), "exported"),
		true, "output = %q", output.String(),
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		!strings.Contains(
			output.String(),
			"other repository",
		), true, "output = %q",
		output.String(),
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		!strings.Contains(output.String(), "added during output"), true,
		"output = %q", output.String(),
	)

	for repository, want := range map[string]string{
		"/repo":  "added during output",
		"/other": "other repository",
	} {
		pending, err := store.List(repository)
		(func(t *testing.T, err error) {
			t.Helper()
			if err != nil {
				t.Fatal(err)
			}
		})(t, err)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, len(pending), 1, "%s pending = %#v",
			repository, pending,
		)
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, pending[0].Body, want, "%s pending = %#v",
			repository, pending,
		)
	}
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
