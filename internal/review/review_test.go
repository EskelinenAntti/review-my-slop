package review

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestSaveCommentAddsRepositoryContext(t *testing.T) {
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	currentReview := New(context.Background(), "/work", patch.Loader{}, store)
	p := patch.Patch{Repository: "/repo"}

	saved, err := currentReview.SaveComment(comments.Comment{Body: "check this"}, p)
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if saved.Repository != "/repo" || len(items) != 1 {
		t.Fatalf("saved = %#v, items = %#v", saved, items)
	}
}

func TestExportDoesNotAcknowledge(t *testing.T) {
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.Add(comments.Comment{ID: "one", Repository: "/repo", Body: "check"}); err != nil {
		t.Fatal(err)
	}
	currentReview := New(context.Background(), "/work", patch.Loader{}, store)
	var output bytes.Buffer

	pending, err := currentReview.ExportComments(&output, patch.Patch{Repository: "/repo"})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || len(stored) != 1 {
		t.Fatalf("pending = %#v, stored = %#v", pending, stored)
	}
	if err := currentReview.AcknowledgeRepository("/repo", pending); err != nil {
		t.Fatal(err)
	}
	stored, err = store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 0 {
		t.Fatalf("stored after acknowledgement = %#v", stored)
	}
}
