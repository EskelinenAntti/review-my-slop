package comments

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestListMissingStoreDoesNotCreateDatabase(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "state")
	path := filepath.Join(directory, "comments.db")
	store := Store{Path: path}

	comments, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(
		t, len(comments), 0, "unexpected comments: %#v", comments,
	)

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database stat error = %v, want not exist", err)
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("directory stat error = %v, want not exist", err)
	}
}

func TestStoreViewsCanShareDatabase(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.Add(testComment("/repo", "feedback")); err != nil {
		t.Fatal(err)
	}

	err := store.view(func(_ *bolt.Bucket) error {
		comments, err := store.List("/repo")
		if err != nil {
			return err
		}
		testutil.RequireEqual(t, len(comments), 1, "comments = %#v", comments)
		return nil
	})
	testutil.NoError(t, err)
}

func TestStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state", "comments.db")}
	first := testComment("/repo/a", "first")
	second := testComment("/repo/b", "other")
	third := testComment("/repo/a", "third")
	for _, comment := range []Comment{first, second, third} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}

	comments, err := store.List("/repo/a")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(comments), 2, "unexpected comments: %#v",
		comments,
	)
	testutil.RequireEqual(t, comments[0].Body, "first",
		"unexpected comments: %#v", comments,
	)
	testutil.RequireEqual(t, comments[1].Body, "third",
		"unexpected comments: %#v", comments,
	)

	if _, err := store.Add(testComment("/repo/a", "newer")); err != nil {
		t.Fatal(err)
	}
	testutil.NoError(t,
		store.Acknowledge("/repo/a", []string{comments[0].ID, comments[1].ID}),
	)

	remainingA, err := store.List("/repo/a")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(remainingA), 1,
		"newer comment was not preserved: %#v", remainingA,
	)
	testutil.RequireEqual(t, remainingA[0].Body, "newer",
		"newer comment was not preserved: %#v", remainingA,
	)

	remainingB, err := store.List("/repo/b")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(remainingB), 1,
		"other repository comments = %d, want 1", len(remainingB),
	)

	dirInfo, err := os.Stat(filepath.Dir(store.Path))
	testutil.NoError(t, err)
	testutil.RequireEqual(t, dirInfo.Mode().Perm(), 0o700,
		"directory mode = %o, want 700", dirInfo.Mode().Perm(),
	)

	dbInfo, err := os.Stat(store.Path)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, dbInfo.Mode().Perm(), 0o600,
		"database mode = %o, want 600", dbInfo.Mode().Perm(),
	)
}

func TestWritePrompt(t *testing.T) {
	var out bytes.Buffer
	comment := testComment("/repo", "Handle the nil case.")
	comment.Anchor = Anchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	testutil.NoError(t, WritePrompt(&out, []Comment{comment}))

	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		testutil.RequireEqual(t, strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s", expected, out.String(),
		)
	}
	testutil.RequireEqual(t, !strings.Contains(out.String(), "batch"), true,
		"output exposes internal batches:\n%s", out.String(),
	)
}

func TestWritePromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer
	comments := []Comment{
		testComment("/repo", "First."),
		testComment("/repo", "Second."),
	}
	testutil.NoError(t, WritePrompt(&out, comments))

	for _, expected := range []string{"### 1.", "### 2."} {
		testutil.RequireEqual(t, strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s", expected, out.String(),
		)
	}
}

func TestStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	empty := testComment("/repo", "")
	if _, err := store.Add(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := testComment("/repo", strings.Repeat("x", maxCommentBytes+1))
	if _, err := store.Add(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestUpdateCannotExceedPendingSizeLimit(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	comment := testComment("/repo", "small")
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}
	bulkComment := testComment("/other", "bulk")
	bulkComment.ID = "bulk"
	bulkData, err := encodeComment(bulkComment)
	testutil.NoError(t, err)
	existingData, err := encodeComment(comment)
	testutil.NoError(t, err)
	paddingSize := maxPendingBytes - len(bulkData) - len(existingData) - 1
	err = store.transact(func(bucket *bolt.Bucket) error {
		padding := bytes.Repeat([]byte(" "), paddingSize)
		paddedData := append(padding, bulkData...)
		return bucket.Put([]byte(bulkComment.ID), paddedData)
	})
	testutil.NoError(t, err)

	comment.Body = strings.Repeat("x", maxCommentBytes)
	err = store.Update(comment)
	limitError := err != nil &&
		strings.Contains(err.Error(), "pending feedback exceeds")
	if !limitError {
		t.Fatalf("Update error = %v, want pending size limit error", err)
	}

	comments, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(comments), 1, "comments = %#v", comments)
	testutil.RequireEqual(t, comments[0].Body, "small", "comments = %#v",
		comments,
	)
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.Add(testComment("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	second := testComment("/repo", "second")
	second.Anchor = Anchor{FilePath: "other.go", NewStart: 2, NewEnd: 2}
	if _, err := store.Add(second); err != nil {
		t.Fatal(err)
	}

	comments, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(comments), 2, "comments = %#v", comments)
	testutil.RequireEqual(t, comments[1].Body, "second", "comments = %#v",
		comments,
	)

	comments[1].Body = "edited"
	testutil.NoError(t, store.Update(comments[1]))

	updated, err := store.List("/repo")
	testutil.NoError(t, err)

	if got := updated[1].Body; got != "edited" {
		t.Fatalf("body = %q, want edited", got)
	}
}

func TestDeleteCommentRemovesMessage(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.Add(testComment("/repo", "first")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(testComment("/repo", "second")); err != nil {
		t.Fatal(err)
	}
	comments, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.NoError(t, store.Delete("/repo", comments[0].ID))

	remaining, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(remaining), 1, "remaining = %#v", remaining)
	testutil.RequireEqual(t, remaining[0].Body, "second", "remaining = %#v",
		remaining,
	)
	testutil.NoError(t, store.Delete("/repo", remaining[0].ID))

	remaining, err = store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(remaining), 0, "remaining = %#v, want empty",
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
	err := store.transact(func(bucket *bolt.Bucket) error {
		return bucket.Put([]byte{0, 0, 0, 1}, legacy)
	})
	testutil.NoError(t, err)

	comments, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(comments), 1, "legacy comment = %#v", comments)
	testutil.RequireEqual(t, comments[0].ID, "legacy", "legacy comment = %#v",
		comments,
	)
	testutil.RequireEqual(t, comments[0].Body, "old", "legacy comment = %#v",
		comments,
	)
	testutil.RequireEqual(t, comments[0].Anchor.NewStart, 3,
		"legacy comment = %#v", comments,
	)

	comments[0].Body = "updated"
	testutil.NoError(t, store.Update(comments[0]))

	updated, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(updated), 1, "updated comments = %#v", updated)
	testutil.RequireEqual(t, updated[0].Body, "updated",
		"updated comments = %#v", updated,
	)
}

func TestStoreOperationsStayWithinRepository(t *testing.T) {
	for _, operation := range []struct {
		name      string
		apply     func(Store, Comment) error
		wantCount int
	}{
		{"update", func(store Store, comment Comment) error {
			comment.Body = "updated"
			return store.Update(comment)
		}, 1},
		{"delete", func(store Store, comment Comment) error {
			return store.Delete(comment.Repository, comment.ID)
		}, 0},
		{"acknowledge", func(store Store, comment Comment) error {
			return store.Acknowledge(comment.Repository, []string{comment.ID})
		}, 0},
	} {
		t.Run(operation.name, func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			other := testComment("/other", "original")
			target := testComment("/repo", "original")
			seedLegacyKeys(t, store, []Comment{other, target})

			listed, err := store.List(target.Repository)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(listed), 1, "comments = %#v", listed)
			testutil.RequireEqual(t, listed[0].Repository, target.Repository,
				"comment = %#v", listed[0],
			)

			testutil.NoError(t, operation.apply(store, target))

			remaining, err := store.List(target.Repository)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(remaining), operation.wantCount,
				"remaining comments = %#v", remaining,
			)
			if operation.wantCount > 0 {
				testutil.RequireEqual(t, remaining[0].Body, "updated",
					"comment = %#v", remaining[0],
				)
			}
			unchanged, err := store.List(other.Repository)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(unchanged), 1,
				"other repository comments = %#v", unchanged,
			)
			testutil.RequireEqual(t, unchanged[0].Body, other.Body,
				"other repository comment = %#v", unchanged[0],
			)
		})
	}
}

func seedLegacyKeys(t *testing.T, store Store, comments []Comment) {
	t.Helper()
	// Legacy keys need not match IDs; repositories can share the same ID.
	records := make([][]byte, len(comments))
	for i, comment := range comments {
		data, err := encodeComment(comment)
		testutil.NoError(t, err)
		records[i] = data
	}
	err := store.transact(func(bucket *bolt.Bucket) error {
		for i, data := range records {
			if err := bucket.Put([]byte{byte(i)}, data); err != nil {
				return err
			}
		}
		return nil
	})
	testutil.NoError(t, err)
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
