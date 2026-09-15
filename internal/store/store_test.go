package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	bbolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
)

func TestStoreKeepsRepositoriesAndSupportsCommentLifecycle(t *testing.T) {
	storage := Store{Path: filepath.Join(t.TempDir(), "review-my-slop", "inbox.db")}
	first, err := storage.Add(comment.Comment{Repository: "/repo/one", Anchor: comment.Anchor{FilePath: "one.go"}, Body: "first"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := storage.Add(comment.Comment{Repository: "/repo/one", Anchor: comment.Anchor{FilePath: "two.go"}, Body: "second"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Add(comment.Comment{Repository: "/repo/two", Body: "other"}); err != nil {
		t.Fatal(err)
	}

	items, err := storage.List("/repo/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	first.Body = "edited"
	if err := storage.Update(first); err != nil {
		t.Fatal(err)
	}
	if err := storage.Delete("/repo/one", second.ID); err != nil {
		t.Fatal(err)
	}
	items, err = storage.List("/repo/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Body != "edited" {
		t.Fatalf("after lifecycle = %#v", items)
	}
	if err := storage.Acknowledge("/repo/one", []string{first.ID}); err != nil {
		t.Fatal(err)
	}
	items, err = storage.List("/repo/one")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("acknowledged items = %#v", items)
	}
	other, err := storage.List("/repo/two")
	if err != nil || len(other) != 1 {
		t.Fatalf("other repository = %#v, err=%v", other, err)
	}
}

func TestStoreSetsSecurePermissionsAndSideBySidePreference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "inbox.db")
	storage := Store{Path: path}
	enabled, err := storage.SideBySide()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("side-by-side preference unexpectedly enabled")
	}
	if err := storage.SetSideBySide(true); err != nil {
		t.Fatal(err)
	}
	enabled, err = storage.SideBySide()
	if err != nil || !enabled {
		t.Fatalf("enabled=%v err=%v", enabled, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("database mode = %o", info.Mode().Perm())
	}
	directory, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if directory.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %o", directory.Mode().Perm())
	}
}

func TestStoreReadsLegacyNestedCommentRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inbox.db")
	database, err := bbolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacy := struct {
		ID         string    `json:"id"`
		Repository string    `json:"repository"`
		CreatedAt  time.Time `json:"created_at"`
		Comment    struct {
			Anchor comment.Anchor `json:"anchor"`
			Body   string         `json:"body"`
		} `json:"comment"`
	}{
		ID:         "legacy",
		Repository: "/repo",
		CreatedAt:  time.Unix(10, 0).UTC(),
		Comment: struct {
			Anchor comment.Anchor `json:"anchor"`
			Body   string         `json:"body"`
		}{Anchor: comment.Anchor{FilePath: "main.go", NewStart: 4}, Body: "legacy body"},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	err = database.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucket(messagesBucket)
		if err != nil {
			return err
		}
		return bucket.Put([]byte("legacy"), data)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	items, err := (Store{Path: path}).List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "legacy" || items[0].Body != "legacy body" || items[0].Anchor.NewStart != 4 {
		t.Fatalf("legacy item = %#v", items)
	}
}

func TestStoreRejectsInvalidCommentsAndDuplicateIDs(t *testing.T) {
	storage := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	for name, item := range map[string]comment.Comment{
		"missing repository": {Body: "body"},
		"missing body":       {Repository: "/repo"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := storage.Add(item); err == nil {
				t.Fatal("invalid comment was accepted")
			}
		})
	}
	item, err := storage.Add(comment.Comment{ID: "same", Repository: "/repo", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.Add(item); err == nil {
		t.Fatal("duplicate comment ID was accepted")
	}
	tooLarge := comment.Comment{Repository: "/repo", Body: string(make([]byte, maxCommentBytes+1))}
	if _, err := storage.Add(tooLarge); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestDefaultPathUsesApplicationDataDirectory(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(data, "review-my-slop", "inbox.db")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}
