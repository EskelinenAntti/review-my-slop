package comments

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestStoreQueuesByRepositoryAndUsesFreshDatabaseName(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	path, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "inbox-v2.db" {
		t.Fatalf("database name = %q", filepath.Base(path))
	}
	store := NewStore(path)
	first := testComment("/repo/a", "first")
	second := testComment("/repo/b", "other")
	third := testComment("/repo/a", "third")
	for _, comment := range []Comment{first, second, third} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.List("/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Body != "first" || items[1].Body != "third" {
		t.Fatalf("items = %#v", items)
	}
	if err := store.Delete("/repo/a", items[0].ID); err != nil {
		t.Fatal(err)
	}
	remaining, err := store.List("/repo/a")
	if err != nil || len(remaining) != 1 || remaining[0].Body != "third" {
		t.Fatalf("remaining = %#v, err=%v", remaining, err)
	}
	directoryInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %o", directoryInfo.Mode().Perm())
	}
	databaseInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if databaseInfo.Mode().Perm() != 0o600 {
		t.Fatalf("database mode = %o", databaseInfo.Mode().Perm())
	}
}

func TestSnapshotAcknowledgesOnlyUnchangedRecords(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inbox-v2.db"))
	original, err := store.Add(testComment("/repo", "original"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot("/repo")
	if err != nil {
		t.Fatal(err)
	}
	changed := original
	changed.Body = "edited while the prompt was being written"
	if err := store.Update(changed); err != nil {
		t.Fatal(err)
	}
	if err := store.Acknowledge(snapshot); err != nil {
		t.Fatal(err)
	}
	items, err := store.List("/repo")
	if err != nil || len(items) != 1 || items[0].Body != changed.Body {
		t.Fatalf("concurrent edit was lost: %#v, err=%v", items, err)
	}

	if _, err := store.Add(testComment("/repo", "delivered")); err != nil {
		t.Fatal(err)
	}
	secondSnapshot, err := store.Snapshot("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Acknowledge(secondSnapshot); err != nil {
		t.Fatal(err)
	}
	items, err = store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("acknowledged items = %#v", items)
	}
}

func TestStoreSupportsConcurrentShortOperations(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inbox-v2.db"))
	const count = 8
	var wait sync.WaitGroup
	errors := make(chan error, count)
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, err := store.Add(testComment("/repo", fmt.Sprintf("comment %d", index)))
			errors <- err
		}(index)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.List("/repo")
	if err != nil || len(items) != count {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
}

func TestStoreRejectsLegacyShapeInsteadOfDecodingIt(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inbox-v2.db"))
	if err := store.update(func(bucket *bolt.Bucket) error {
		return bucket.Put([]byte("legacy"), []byte(`{"id":"legacy","repository":"/repo","comment":{"body":"old"}}`))
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List("/repo"); err == nil {
		t.Fatal("legacy nested comment was decoded")
	}
}

func TestWritePromptKeepsMessageNumberingAndRanges(t *testing.T) {
	var output bytes.Buffer
	comment := testComment("/repo", "Handle the nil case.")
	comment.Anchor = Anchor{FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13, QuotedLines: []string{"-old()", "+new()"}}
	if err := WritePrompt(&output, []Comment{comment, testComment("/repo", "Second.")}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"New comments since last run:", "### 1.", "### 2.", "`main.go`", "old lines 10-11", "new lines 12-13", "```diff", "Handle the nil case."} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output lacks %q:\n%s", expected, output.String())
		}
	}
	if strings.Contains(output.String(), "batch") {
		t.Fatalf("output exposes internal batches:\n%s", output.String())
	}
}

func TestStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inbox-v2.db"))
	if _, err := store.Add(testComment("/repo", "")); err == nil {
		t.Fatal("empty comment was accepted")
	}
	if _, err := store.Add(testComment("/repo", strings.Repeat("x", maxCommentBytes+1))); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestInboxOwnsRepositoryScopedCommentOperations(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "inbox-v2.db"))
	inbox, err := NewInbox(store, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := inbox.Save(Comment{Anchor: Anchor{FilePath: "file.go", NewStart: 1}, Body: "first"})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Repository != "/repo" {
		t.Fatalf("repository = %q", saved.Repository)
	}
	saved.Body = "updated"
	if _, err := inbox.Save(saved); err != nil {
		t.Fatal(err)
	}
	items, err := inbox.List()
	if err != nil || len(items) != 1 || items[0].Body != "updated" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	if err := inbox.Delete(saved); err != nil {
		t.Fatal(err)
	}
	items, err = inbox.List()
	if err != nil || len(items) != 0 {
		t.Fatalf("items after delete=%#v err=%v", items, err)
	}
}

func TestNewInboxRequiresRepository(t *testing.T) {
	if _, err := NewInbox(NewStore(filepath.Join(t.TempDir(), "inbox-v2.db")), ""); err == nil {
		t.Fatal("empty repository was accepted")
	}
}

func testComment(repository, body string) Comment {
	return Comment{ID: body, Repository: repository, CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
}
