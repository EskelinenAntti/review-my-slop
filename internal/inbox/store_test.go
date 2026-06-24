package inbox

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state", "inbox.db")}
	first := testComment("/repo/a", "first")
	second := testComment("/repo/b", "other")
	third := testComment("/repo/a", "third")
	for _, comment := range []review.Comment{first, second, third} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}

	comments, err := store.List("/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 || comments[0].Body != "first" || comments[1].Body != "third" {
		t.Fatalf("unexpected comments: %#v", comments)
	}

	if _, err := store.Add(testComment("/repo/a", "newer")); err != nil {
		t.Fatal(err)
	}
	if err := store.Acknowledge("/repo/a", []string{comments[0].ID, comments[1].ID}); err != nil {
		t.Fatal(err)
	}

	remainingA, err := store.List("/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingA) != 1 || remainingA[0].Body != "newer" {
		t.Fatalf("newer comment was not preserved: %#v", remainingA)
	}
	remainingB, err := store.List("/repo/b")
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingB) != 1 {
		t.Fatalf("other repository comments = %d, want 1", len(remainingB))
	}

	dirInfo, err := os.Stat(filepath.Dir(store.Path))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %o, want 700", dirInfo.Mode().Perm())
	}
	dbInfo, err := os.Stat(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if dbInfo.Mode().Perm() != 0o600 {
		t.Fatalf("database mode = %o, want 600", dbInfo.Mode().Perm())
	}
}

func TestWritePrompt(t *testing.T) {
	var out bytes.Buffer
	comment := testComment("/repo", "Handle the nil case.")
	comment.Anchor = review.Anchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	if err := WritePrompt(&out, []review.Comment{comment}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("output lacks %q:\n%s", expected, out.String())
		}
	}
	if strings.Contains(out.String(), "batch") {
		t.Fatalf("output exposes internal batches:\n%s", out.String())
	}
}

func TestWritePromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer
	if err := WritePrompt(&out, []review.Comment{
		testComment("/repo", "First."),
		testComment("/repo", "Second."),
	}); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"### 1.", "### 2."} {
		if !strings.Contains(out.String(), expected) {
			t.Fatalf("output lacks %q:\n%s", expected, out.String())
		}
	}
}

func TestStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	empty := testComment("/repo", "")
	if _, err := store.Add(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := testComment("/repo", strings.Repeat("x", maxCommentBytes+1))
	if _, err := store.Add(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	if _, err := store.Add(testComment("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	second := testComment("/repo", "second")
	second.Anchor = review.Anchor{FilePath: "other.go", NewStart: 2, NewEnd: 2}
	if _, err := store.Add(second); err != nil {
		t.Fatal(err)
	}

	comments, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 || comments[1].Body != "second" {
		t.Fatalf("comments = %#v", comments)
	}
	comments[1].Body = "edited"
	if _, err := store.Save(comments[1], "/repo"); err != nil {
		t.Fatal(err)
	}

	updated, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if got := updated[1].Body; got != "edited" {
		t.Fatalf("body = %q, want edited", got)
	}
}

func TestSaveAssignsRepositoryToNewComment(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	comment, err := store.Save(review.Comment{Body: "new"}, "/repo")
	if err != nil {
		t.Fatal(err)
	}
	if comment.ID == "" || comment.Repository != "/repo" {
		t.Fatalf("saved comment = %#v", comment)
	}
}

func TestDeleteCommentRemovesMessage(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	if _, err := store.Add(testComment("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(testComment("/repo", "second")); err != nil {
		t.Fatal(err)
	}
	comments, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("/repo", comments[0].ID); err != nil {
		t.Fatal(err)
	}
	remaining, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Body != "second" {
		t.Fatalf("remaining = %#v", remaining)
	}
	if err := store.Delete("/repo", remaining[0].ID); err != nil {
		t.Fatal(err)
	}
	remaining, err = store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining = %#v, want empty", remaining)
	}
}

func TestLegacyMessageCanBeReadAndUpdated(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	legacy := []byte(`{"id":"legacy","repository":"/repo","created_at":"1970-01-01T00:00:01Z","diff_fingerprint":"unused","comment":{"anchor":{"file":"main.go","hunk":"@@","start_row":1,"end_row":2,"new_start":3,"quoted_lines":["+line"]},"body":"old"}}`)
	if err := store.update(func(bucket *bolt.Bucket) error {
		return bucket.Put([]byte{0, 0, 0, 1}, legacy)
	}); err != nil {
		t.Fatal(err)
	}

	comments, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].ID != "legacy" || comments[0].Body != "old" || comments[0].Anchor.NewStart != 3 {
		t.Fatalf("legacy comment = %#v", comments)
	}
	comments[0].Body = "updated"
	if err := store.Update(comments[0]); err != nil {
		t.Fatal(err)
	}
	updated, err := store.List("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(updated) != 1 || updated[0].Body != "updated" {
		t.Fatalf("updated comments = %#v", updated)
	}
}

func TestSideBySidePreference(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}

	enabled, err := store.SideBySide()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("side-by-side defaults to enabled")
	}

	if err := store.SetSideBySide(true); err != nil {
		t.Fatal(err)
	}
	enabled, err = store.SideBySide()
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Fatal("side-by-side preference was not enabled")
	}

	if err := store.SetSideBySide(false); err != nil {
		t.Fatal(err)
	}
	enabled, err = store.SideBySide()
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Fatal("side-by-side preference was not disabled")
	}
}

func testComment(repository, body string) review.Comment {
	return review.Comment{
		ID:         body,
		Repository: repository,
		CreatedAt:  time.Unix(1, 0).UTC(),
		Anchor:     review.Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1},
		Body:       body,
	}
}
