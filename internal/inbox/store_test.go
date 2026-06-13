package inbox

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anttieskelinen/review-my-slop/internal/review"
)

func TestStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state", "inbox.db")}
	first := testBatch("/repo/a", "first")
	second := testBatch("/repo/b", "other")
	third := testBatch("/repo/a", "third")
	for _, batch := range []review.Batch{first, second, third} {
		if err := store.Put(batch); err != nil {
			t.Fatal(err)
		}
	}

	taken, err := store.Peek("/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(taken.Batches) != 2 ||
		taken.Batches[0].Comments[0].Body != "first" ||
		taken.Batches[1].Comments[0].Body != "third" {
		t.Fatalf("unexpected batches: %#v", taken.Batches)
	}

	if err := store.Put(testBatch("/repo/a", "newer")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(taken); err != nil {
		t.Fatal(err)
	}

	remainingA, err := store.Peek("/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingA.Batches) != 1 || remainingA.Batches[0].Comments[0].Body != "newer" {
		t.Fatalf("newer batch was not preserved: %#v", remainingA.Batches)
	}
	remainingB, err := store.Peek("/repo/b")
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingB.Batches) != 1 {
		t.Fatalf("other repository batches = %d, want 1", len(remainingB.Batches))
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
	batch := testBatch("/repo", "Handle the nil case.")
	batch.Comments[0].Anchor = review.Anchor{
		File: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	if err := WritePrompt(&out, []review.Batch{batch}); err != nil {
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

func TestWritePromptNumbersCommentsAcrossBatches(t *testing.T) {
	var out bytes.Buffer
	if err := WritePrompt(&out, []review.Batch{
		testBatch("/repo", "First."),
		testBatch("/repo", "Second."),
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
	empty := testBatch("/repo", "")
	if err := store.Put(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := testBatch("/repo", strings.Repeat("x", maxCommentBytes+1))
	if err := store.Put(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	batch := testBatch("/repo", "first")
	batch.Comments = append(batch.Comments, review.Comment{
		Anchor: review.Anchor{File: "other.go", NewStart: 2, NewEnd: 2},
		Body:   "second",
	})
	if err := store.Put(batch); err != nil {
		t.Fatal(err)
	}

	comments, err := store.ListComments("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 2 || comments[1].Comment.Body != "second" {
		t.Fatalf("comments = %#v", comments)
	}
	comments[1].Comment.Body = "edited"
	if err := store.UpdateComment("/repo", comments[1]); err != nil {
		t.Fatal(err)
	}

	updated, err := store.ListComments("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if got := updated[1].Comment.Body; got != "edited" {
		t.Fatalf("body = %q, want edited", got)
	}
}

func TestDeleteCommentRemovesCommentAndEmptyBatch(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	batch := testBatch("/repo", "first")
	batch.Comments = append(batch.Comments, review.Comment{Body: "second"})
	if err := store.Put(batch); err != nil {
		t.Fatal(err)
	}
	comments, err := store.ListComments("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteComment("/repo", comments[0]); err != nil {
		t.Fatal(err)
	}
	remaining, err := store.ListComments("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || remaining[0].Index != 0 || remaining[0].Comment.Body != "second" {
		t.Fatalf("remaining = %#v", remaining)
	}
	if err := store.DeleteComment("/repo", remaining[0]); err != nil {
		t.Fatal(err)
	}
	remaining, err = store.ListComments("/repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining = %#v, want empty", remaining)
	}
}

func testBatch(repository, body string) review.Batch {
	return review.Batch{
		ID:         body,
		Repository: repository,
		CreatedAt:  time.Unix(1, 0).UTC(),
		Comments: []review.Comment{{
			Anchor: review.Anchor{File: "file.go", NewStart: 1, NewEnd: 1},
			Body:   body,
		}},
	}
}
