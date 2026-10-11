package comments

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestWritePendingAcknowledgesOnlyExportedComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	for _, comment := range []Comment{
		testComment(fixtureRepository, "exported"),
		testComment("/other", "other repository"),
	} {
		{
			_, err := store.Add(comment)
			requireEqual(t, err == nil, true, fixtureV, err)
		}
	}
	var output bytes.Buffer
	writer := &addingWriter{
		Buffer: &output,
		add: func() {
			{
				_, err := store.Add(
					testComment(fixtureRepository, "added during output"),
				)
				requireEqual(t, err == nil, true, fixtureV, err)
			}
		},
	}
	{
		err := store.WritePending(writer, fixtureRepository)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	requireEqual(
		t,
		strings.Contains(output.String(), "exported") &&
			!strings.Contains(output.String(), "other repository") &&
			!strings.Contains(output.String(), "added during output"),
		true,
		"output = %q",
		output.String(),
	)
	for repository, want := range map[string]string{
		fixtureRepository: "added during output",
		"/other":          "other repository",
	} {
		pending, err := store.List(repository)
		requireEqual(t, err == nil, true, fixtureV, err)
		requireEqual(
			t,
			len(pending) == 1 && pending[0].Body == want, true,
			"%s pending = %#v",
			repository,
			pending,
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
