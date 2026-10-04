package comments

import (
	"path/filepath"
	"testing"
	"time"
)

func TestQueueSaveEditsWithoutLosingCreationMetadata(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	queue := Bind(store, "/repo")
	created, err := queue.Save(Draft{Anchor: Anchor{FilePath: "old.go"}, Body: "first"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := queue.Save(Draft{ID: created.ID, Anchor: Anchor{FilePath: "new.go"}, Body: "edited"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) || updated.Repository != "/repo" || updated.Anchor.FilePath != "new.go" || updated.Body != "edited" {
		t.Fatalf("updated comment = %#v; created was %#v", updated, created)
	}
	if _, err := queue.Save(Draft{ID: "missing", Body: "edited"}); err == nil {
		t.Fatal("editing a missing ID succeeded")
	}
	if _, err := queue.Save(Draft{ID: created.ID, Body: ""}); err == nil {
		t.Fatal("empty body edit succeeded")
	}
}

func TestQueueAcknowledgeRemovesOnlyExportedSnapshot(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	queue := Bind(store, "/repo")
	first, err := queue.Save(Draft{Body: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := queue.Save(Draft{Body: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Acknowledge([]Comment{first}); err != nil {
		t.Fatal(err)
	}
	pending, err := queue.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].ID != second.ID {
		t.Fatalf("pending after acknowledging snapshot = %#v", pending)
	}
}

func TestQueueSaveUsesBoundRepository(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	createdAt := time.Now().UTC()
	created, err := store.Add(Comment{Repository: "/elsewhere", CreatedAt: createdAt, Body: "other"})
	if err != nil {
		t.Fatal(err)
	}
	queue := Bind(store, "/repo")
	if _, err := queue.Save(Draft{ID: created.ID, Body: "changed"}); err == nil {
		t.Fatal("queue edited a comment from a different repository")
	}
}
