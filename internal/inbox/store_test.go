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
		"Apply the following review feedback",
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
