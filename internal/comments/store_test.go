package comments

import (
	"bytes"
	"errors"
	"fmt"
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
	first := func() Comment {
		var body string = "first"
		return Comment{ID: body, Repository: "/repo/a", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	second := func() Comment {
		var body string = "other"
		return Comment{ID: body, Repository: "/repo/b", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	third := func() Comment {
		var body string = "third"
		return Comment{ID: body, Repository: "/repo/a", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
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

	if _, err := store.Add(func() Comment {
		var body string = "newer"
		return Comment{ID: body, Repository: "/repo/a", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
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
	comment := func() Comment {
		var body string = "Handle the nil case."
		return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
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
		func() Comment {
			var body string = "First."
			return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
		}(),
		func() Comment {
			var body string = "Second."
			return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
		}(),
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
	empty := Comment{ID: "", Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: ""}
	if _, err := store.Add(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := func() Comment {
		var body string = strings.Repeat("x", maxCommentBytes+1)
		return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	if _, err := store.Add(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.Add(func() Comment {
		var body string = "first"
		return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
		t.Fatal(err)
	}
	second := func() Comment {
		var body string = "second"
		return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
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
	if _, err := store.Add(func() Comment {
		var body string = "first"
		return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(func() Comment {
		var body string = "second"
		return Comment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: Anchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
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
	err := func() error {
		db, err := func() (*bolt.DB, error) {
			if store.Path == "" {
				return nil, errors.New("comments path is empty")
			}
			if err := os.MkdirAll(filepath.Dir(store.Path), 0o700); err != nil {
				return nil, fmt.Errorf("create comments directory: %w", err)
			}
			if err := os.Chmod(filepath.Dir(store.Path), 0o700); err != nil {
				return nil, fmt.Errorf("secure comments directory: %w", err)
			}
			db, err := bolt.Open(store.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
			if err != nil {
				return nil, fmt.Errorf("open comments: %w", err)
			}
			if err := os.Chmod(store.Path, 0o600); err != nil {
				db.Close()
				return nil, fmt.Errorf("secure comments database: %w", err)
			}
			return db, nil
		}()
		if err != nil {
			return err
		}
		defer db.Close()
		return db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists(messagesBucket)
			if err != nil {
				return err
			}
			return func(bucket *bolt.Bucket) error { return bucket.Put([]byte{0, 0, 0, 1}, legacy) }(bucket)
		})
	}()
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
