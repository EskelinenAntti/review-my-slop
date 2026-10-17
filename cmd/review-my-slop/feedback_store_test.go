package main

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
)

func TestCommentsStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := commentsStore{Path: filepath.Join(
		t.TempDir(),
		"state",
		"comments.db",
	)}
	comments := []commentsComment{
		feedbackOutputComment("first", "/repo/a"),
		feedbackOutputComment("other", "/repo/b"),
		feedbackOutputComment("third", "/repo/a"),
	}
	for _, comment := range comments {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}

	firstPeek, err := store.List("/repo/a")
	requireNoError(t, err)
	requireEqual(t, len(firstPeek), 2, "unexpected comments: %#v", firstPeek)
	requireEqual(
		t,
		firstPeek[0].Body,
		"first",
		"unexpected comments: %#v",
		firstPeek,
	)
	requireEqual(
		t,
		firstPeek[1].Body,
		"third",
		"unexpected comments: %#v",
		firstPeek,
	)

	_, addErr := store.Add(
		feedbackOutputComment("newer", "/repo/a"),
	)
	if addErr != nil {
		t.Fatal(addErr)
	}
	requireNoError(t, store.Acknowledge(
		"/repo/a",
		[]string{firstPeek[0].ID, firstPeek[1].ID},
	))
	requireRepositoryBodies(t, store, "/repo/a", []string{"newer"})
	requireRepositoryBodies(t, store, "/repo/b", []string{"other"})
	requireStorePermissions(t, store.Path)
}

func requireRepositoryBodies(
	t *testing.T,
	store commentsStore,
	repository string,
	want []string,
) {
	t.Helper()
	comments, err := store.List(repository)
	requireNoError(t, err)
	bodies := make([]string, len(comments))
	for index, comment := range comments {
		bodies[index] = comment.Body
	}
	requireEqual(
		t, len(bodies), len(want), "comments in %s = %#v", repository, comments,
	)
	for index, body := range want {
		requireEqual(
			t,
			bodies[index],
			body,
			"comments in %s = %#v",
			repository,
			comments,
		)
	}
}

func requireStorePermissions(t *testing.T, path string) {
	t.Helper()
	dirInfo, err := os.Stat(filepath.Dir(path))
	requireNoError(t, err)
	requireEqual(
		t,
		dirInfo.Mode().Perm(),
		0o700,
		"directory mode = %o, want 700",
		dirInfo.Mode().Perm(),
	)
	dbInfo, err := os.Stat(path)
	requireNoError(t, err)
	requireEqual(
		t,
		dbInfo.Mode().Perm(),
		0o600,
		"database mode = %o, want 600",
		dbInfo.Mode().Perm(),
	)
}

func TestCommentsWritePrompt(t *testing.T) {
	var out bytes.Buffer
	comment := feedbackOutputComment("Handle the nil case.", "/repo")
	comment.Anchor = commentsAnchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	requireNoError(t, writePendingComments(&out, []commentsComment{comment}))

	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		requireEqual(t, strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s", expected, out.String(),
		)
	}
	requireEqual(t, !strings.Contains(out.String(), "batch"), true,
		"output exposes internal batches:\n%s", out.String(),
	)
}

func TestCommentsWritePromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer
	comments := []commentsComment{
		func() commentsComment {
			var body = "First."
			return commentsComment{
				ID:         body,
				Repository: "/repo",
				CreatedAt:  time.Unix(1, 0).UTC(),
				Anchor: commentsAnchor{
					FilePath: "file.go",
					NewStart: 1,
					NewEnd:   1,
				},
				Body: body,
			}
		}(),
		func() commentsComment {
			var body = "Second."
			return commentsComment{
				ID:         body,
				Repository: "/repo",
				CreatedAt:  time.Unix(1, 0).UTC(),
				Anchor: commentsAnchor{
					FilePath: "file.go",
					NewStart: 1,
					NewEnd:   1,
				},
				Body: body,
			}
		}(),
	}
	requireNoError(t, writePendingComments(&out, comments))

	for _, expected := range []string{"### 1.", "### 2."} {
		requireEqual(t, strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s", expected, out.String(),
		)
	}
}

func TestCommentsStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	empty := commentsComment{ID: "", Repository: "/repo", CreatedAt: time.Unix(
		1,
		0,
	).UTC(), Anchor: commentsAnchor{
		FilePath: "file.go",
		NewStart: 1,
		NewEnd:   1,
	}, Body: ""}
	if _, err := store.Add(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := func() commentsComment {
		var body = strings.Repeat("x", commentsMaxCommentBytes+1)
		return commentsComment{
			ID:         body,
			Repository: "/repo",
			CreatedAt:  time.Unix(1, 0).UTC(),
			Anchor: commentsAnchor{
				FilePath: "file.go", NewStart: 1, NewEnd: 1,
			},
			Body:       body,
		}
	}()
	if _, err := store.Add(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestCommentsListAndUpdateComments(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	_, addErr := store.Add(
		feedbackOutputComment("first", "/repo"),
	)
	if addErr != nil {
		t.Fatal(addErr)
	}
	second := feedbackOutputComment("second", "/repo")
	second.Anchor = commentsAnchor{FilePath: "other.go", NewStart: 2, NewEnd: 2}
	if _, err := store.Add(second); err != nil {
		t.Fatal(err)
	}

	comments, err := store.List("/repo")
	requireNoError(t, err)
	requireEqual(t, len(comments), 2, "comments = %#v", comments)
	requireEqual(t, comments[1].Body, "second", "comments = %#v", comments)

	comments[1].Body = "edited"
	requireNoError(t, store.Update(comments[1]))

	updated, err := store.List("/repo")
	requireNoError(t, err)

	if got := updated[1].Body; got != "edited" {
		t.Fatalf("body = %q, want edited", got)
	}
}

func TestCommentsDeleteCommentRemovesMessage(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	_, addErr := store.Add(
		feedbackOutputComment("first", "/repo"),
	)
	if addErr != nil {
		t.Fatal(addErr)
	}
	_, addErr = store.Add(
		feedbackOutputComment("second", "/repo"),
	)
	if addErr != nil {
		t.Fatal(addErr)
	}
	comments, err := store.List("/repo")
	requireNoError(t, err)
	requireNoError(t, store.Delete("/repo", comments[0].ID))

	remaining, err := store.List("/repo")
	requireNoError(t, err)
	requireEqual(t, len(remaining), 1, "remaining = %#v", remaining)
	requireEqual(t, remaining[0].Body, "second", "remaining = %#v", remaining)
	requireNoError(t, store.Delete("/repo", remaining[0].ID))

	remaining, err = store.List("/repo")
	requireNoError(t, err)
	requireEqual(t, len(remaining), 0, "remaining = %#v, want empty", remaining)
}

func TestCommentsLegacyMessageCanBeReadAndUpdated(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
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
	requireNoError(t, writeLegacyCommentRecord(store, legacy))

	comments, err := store.List("/repo")
	requireNoError(t, err)
	requireEqual(t, len(comments), 1, "legacy comment = %#v", comments)
	requireEqual(t, comments[0].ID, "legacy", "legacy comment = %#v", comments)
	requireEqual(t, comments[0].Body, "old", "legacy comment = %#v", comments)
	requireEqual(t, comments[0].Anchor.NewStart, 3,
		"legacy comment = %#v", comments,
	)

	comments[0].Body = "updated"
	requireNoError(t, store.Update(comments[0]))

	updated, err := store.List("/repo")
	requireNoError(t, err)
	requireEqual(t, len(updated), 1, "updated comments = %#v", updated)
	requireEqual(t, updated[0].Body, "updated",
		"updated comments = %#v", updated,
	)
}

func writeLegacyCommentRecord(store commentsStore, legacy []byte) error {
	if store.Path == "" {
		return errors.New("comments path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(store.Path), 0o700); err != nil {
		return fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(store.Path), 0o700); err != nil {
		return fmt.Errorf("secure comments directory: %w", err)
	}
	db, err := bolt.Open(store.Path, 0o600, &bolt.Options{
		Timeout: 2 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("open comments: %w", err)
	}
	defer db.Close()
	if err := os.Chmod(store.Path, 0o600); err != nil {
		return fmt.Errorf("secure comments database: %w", err)
	}
	return db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
		if err != nil {
			return err
		}
		return bucket.Put([]byte{0, 0, 0, 1}, legacy)
	})
}
