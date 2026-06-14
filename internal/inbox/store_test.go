package inbox

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state", "inbox.db")}
	first := testMessage("/repo/a", "first")
	second := testMessage("/repo/b", "other")
	third := testMessage("/repo/a", "third")
	for _, message := range []Message{first, second, third} {
		if err := store.Put(message); err != nil {
			t.Fatal(err)
		}
	}

	taken, err := store.Peek("/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(taken.Messages) != 2 ||
		taken.Messages[0].Comment.Body != "first" ||
		taken.Messages[1].Comment.Body != "third" {
		t.Fatalf("unexpected messages: %#v", taken.Messages)
	}

	if err := store.Put(testMessage("/repo/a", "newer")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(taken); err != nil {
		t.Fatal(err)
	}

	remainingA, err := store.Peek("/repo/a")
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingA.Messages) != 1 || remainingA.Messages[0].Comment.Body != "newer" {
		t.Fatalf("newer message was not preserved: %#v", remainingA.Messages)
	}
	remainingB, err := store.Peek("/repo/b")
	if err != nil {
		t.Fatal(err)
	}
	if len(remainingB.Messages) != 1 {
		t.Fatalf("other repository messages = %d, want 1", len(remainingB.Messages))
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
	message := testMessage("/repo", "Handle the nil case.")
	message.Comment.Anchor = review.Anchor{
		File: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	if err := WritePrompt(&out, []Message{message}); err != nil {
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
	if err := WritePrompt(&out, []Message{
		testMessage("/repo", "First."),
		testMessage("/repo", "Second."),
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
	empty := testMessage("/repo", "")
	if err := store.Put(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := testMessage("/repo", strings.Repeat("x", maxCommentBytes+1))
	if err := store.Put(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	if err := store.Put(testMessage("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	second := testMessage("/repo", "second")
	second.Comment.Anchor = review.Anchor{File: "other.go", NewStart: 2, NewEnd: 2}
	if err := store.Put(second); err != nil {
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

func TestDeleteCommentRemovesMessage(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "inbox.db")}
	if err := store.Put(testMessage("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	if err := store.Put(testMessage("/repo", "second")); err != nil {
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
	if len(remaining) != 1 || remaining[0].Comment.Body != "second" {
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

func testMessage(repository, body string) Message {
	return Message{
		ID:         body,
		Repository: repository,
		CreatedAt:  time.Unix(1, 0).UTC(),
		Comment: review.Comment{
			Anchor: review.Anchor{File: "file.go", NewStart: 1, NewEnd: 1},
			Body:   body,
		},
	}
}
