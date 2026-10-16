package comments

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state", "comments.db")}
	first := testComment("/repo/a", "first")
	second := testComment("/repo/b", "other")
	third := testComment("/repo/a", "third")
	for _, comment := range []Comment{first, second, third} {
		if _, err := store.AddComment(comment); err != nil {
			t.Fatal(err)
		}
	}

	comments, err := store.ListComments("/repo/a")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(comments), 2, "unexpected comments: %#v",
		comments,
	)
	testutil.RequireValuesEqual(t, comments[0].Body, "first",
		"unexpected comments: %#v", comments,
	)
	testutil.RequireValuesEqual(t, comments[1].Body, "third",
		"unexpected comments: %#v", comments,
	)

	if _, err := store.AddComment(testComment("/repo/a", "newer")); err != nil {
		t.Fatal(err)
	}
	testutil.RequireNoError(
		t,
		store.DeleteDeliveredComments(
			"/repo/a",
			[]string{comments[0].ID, comments[1].ID},
		),
	)

	remainingA, err := store.ListComments("/repo/a")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(remainingA), 1,
		"newer comment was not preserved: %#v", remainingA,
	)
	testutil.RequireValuesEqual(t, remainingA[0].Body, "newer",
		"newer comment was not preserved: %#v", remainingA,
	)

	remainingB, err := store.ListComments("/repo/b")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(remainingB), 1,
		"other repository comments = %d, want 1", len(remainingB),
	)

	dirInfo, err := os.Stat(filepath.Dir(store.Path))
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, dirInfo.Mode().Perm(), 0o700,
		"directory mode = %o, want 700", dirInfo.Mode().Perm(),
	)

	dbInfo, err := os.Stat(store.Path)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, dbInfo.Mode().Perm(), 0o600,
		"database mode = %o, want 600", dbInfo.Mode().Perm(),
	)
}

func TestWriteCommentPrompt(t *testing.T) {
	var out bytes.Buffer
	comment := testComment("/repo", "Handle the nil case.")
	comment.Anchor = Anchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	testutil.RequireNoError(t, WriteCommentPrompt(&out, []Comment{comment}))

	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		testutil.RequireValuesEqual(
			t,
			strings.Contains(out.String(), expected),
			true,
			"output lacks %q:\n%s",
			expected,
			out.String(),
		)
	}
	testutil.RequireValuesEqual(
		t,
		!strings.Contains(out.String(), "batch"),
		true,
		"output exposes internal batches:\n%s",
		out.String(),
	)
}

func TestWriteCommentPromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer
	comments := []Comment{
		testComment("/repo", "First."),
		testComment("/repo", "Second."),
	}
	testutil.RequireNoError(t, WriteCommentPrompt(&out, comments))

	for _, expected := range []string{"### 1.", "### 2."} {
		testutil.RequireValuesEqual(
			t,
			strings.Contains(out.String(), expected),
			true,
			"output lacks %q:\n%s",
			expected,
			out.String(),
		)
	}
}

func TestStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	empty := testComment("/repo", "")
	if _, err := store.AddComment(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := testComment("/repo", strings.Repeat("x", maxCommentBytes+1))
	if _, err := store.AddComment(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.AddComment(testComment("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	second := testComment("/repo", "second")
	second.Anchor = Anchor{FilePath: "other.go", NewStart: 2, NewEnd: 2}
	if _, err := store.AddComment(second); err != nil {
		t.Fatal(err)
	}

	comments, err := store.ListComments("/repo")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(comments), 2, "comments = %#v", comments)
	testutil.RequireValuesEqual(t, comments[1].Body, "second", "comments = %#v",
		comments,
	)

	comments[1].Body = "edited"
	testutil.RequireNoError(t, store.UpdateComment(comments[1]))

	updated, err := store.ListComments("/repo")
	testutil.RequireNoError(t, err)

	if got := updated[1].Body; got != "edited" {
		t.Fatalf("body = %q, want edited", got)
	}
}

func TestUpdateCannotExceedPendingCommentLimit(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	target, err := store.AddComment(testComment("/repo", "short"))
	testutil.RequireNoError(t, err)

	filler, err := json.Marshal(
		testComment("/other", strings.Repeat("x", maxPendingBytes-2048)),
	)
	testutil.RequireNoError(t, err)
	database, err := store.openCommentsFile()
	testutil.RequireNoError(t, err)
	err = database.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
		if err != nil {
			return err
		}
		return bucket.Put([]byte("filler"), filler)
	})
	testutil.RequireNoError(t, err)
	testutil.RequireNoError(t, database.Close())

	target.Body = strings.Repeat("x", maxCommentBytes)
	if err := store.UpdateComment(target); err == nil {
		t.Fatal("update exceeded the pending comment limit")
	}

	comments, err := store.ListComments("/repo")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(comments), 1, "comments = %#v", comments)
	testutil.RequireValuesEqual(
		t,
		comments[0].Body,
		"short",
		"comments = %#v",
		comments,
	)
}

func TestDeleteCommentRemovesMessage(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.AddComment(testComment("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddComment(testComment("/repo", "second")); err != nil {
		t.Fatal(err)
	}
	comments, err := store.ListComments("/repo")
	testutil.RequireNoError(t, err)
	testutil.RequireNoError(t, store.DeleteComment("/repo", comments[0].ID))

	remaining, err := store.ListComments("/repo")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(
		t,
		len(remaining),
		1,
		"remaining = %#v",
		remaining,
	)
	testutil.RequireValuesEqual(
		t,
		remaining[0].Body,
		"second",
		"remaining = %#v",
		remaining,
	)
	testutil.RequireNoError(t, store.DeleteComment("/repo", remaining[0].ID))

	remaining, err = store.ListComments("/repo")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(
		t,
		len(remaining),
		0,
		"remaining = %#v, want empty",
		remaining,
	)
}

func TestLegacyMessageCanBeReadAndUpdated(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	legacy := []byte(
		`{
  "id": "legacy",
  "repository": "/repo",
  "created_at": "1970-01-01T00:00:01Z",
  "diff_fingerprint": "unused",
  "comment": {
    "anchor": {
      "file": "main.go",
      "hunk": "@@",
      "start_row": 1,
      "end_row": 2,
      "new_start": 3,
      "quoted_lines": [
        "+line"
      ]
    },
    "body": "old"
  }
}`,
	)
	err := store.saveComments(func(bucket *bolt.Bucket) error {
		return bucket.Put([]byte{0, 0, 0, 1}, legacy)
	})
	testutil.RequireNoError(t, err)

	comments, err := store.ListComments("/repo")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(
		t,
		len(comments),
		1,
		"legacy comment = %#v",
		comments,
	)
	testutil.RequireValuesEqual(
		t,
		comments[0].ID,
		"legacy",
		"legacy comment = %#v",
		comments,
	)
	testutil.RequireValuesEqual(
		t,
		comments[0].Body,
		"old",
		"legacy comment = %#v",
		comments,
	)
	testutil.RequireValuesEqual(t, comments[0].Anchor.NewStart, 3,
		"legacy comment = %#v", comments,
	)

	comments[0].Body = "updated"
	testutil.RequireNoError(t, store.UpdateComment(comments[0]))

	updated, err := store.ListComments("/repo")
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(
		t,
		len(updated),
		1,
		"updated comments = %#v",
		updated,
	)
	testutil.RequireValuesEqual(t, updated[0].Body, "updated",
		"updated comments = %#v", updated,
	)
}

func testComment(repository, body string) Comment {
	return Comment{
		ID:         body,
		Repository: repository,
		CreatedAt:  time.Unix(1, 0).UTC(),
		Anchor:     Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1},
		Body:       body,
	}
}
