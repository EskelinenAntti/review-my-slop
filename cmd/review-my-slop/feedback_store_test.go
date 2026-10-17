package main

import (
	bytes "bytes"
	errors "errors"
	fmt "fmt"
	io "io"
	"io/fs"
	os "os"
	filepath "path/filepath"
	strings "strings"
	testing "testing"
	time "time"

	bolt "go.etcd.io/bbolt"
)

func TestCommentsStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "state", "comments.db")}
	first := func() commentsComment {
		var body string = "first"
		return commentsComment{ID: body, Repository: "/repo/a", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	second := func() commentsComment {
		var body string = "other"
		return commentsComment{ID: body, Repository: "/repo/b", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	third := func() commentsComment {
		var body string = "third"
		return commentsComment{ID: body, Repository: "/repo/a", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	for _, comment := range []commentsComment{first, second, third} {
		if _, err := store.Add(comment); err != nil {
			t.Fatal(err)
		}
	}

	comments, err := store.List("/repo/a")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(comments), 2, "unexpected comments: %#v",
		comments,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, comments[0].Body, "first",
		"unexpected comments: %#v", comments,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, comments[1].Body, "third",
		"unexpected comments: %#v", comments,
	)

	if _, err := store.Add(func() commentsComment {
		var body string = "newer"
		return commentsComment{ID: body, Repository: "/repo/a", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
		t.Fatal(err)
	}
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t,
		store.Acknowledge("/repo/a", []string{comments[0].ID, comments[1].ID}),
	)

	remainingA, err := store.List("/repo/a")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(remainingA), 1,
		"newer comment was not preserved: %#v", remainingA,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, remainingA[0].Body, "newer",
		"newer comment was not preserved: %#v", remainingA,
	)

	remainingB, err := store.List("/repo/b")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(remainingB), 1,
		"other repository comments = %d, want 1", len(remainingB),
	)

	dirInfo, err := os.Stat(filepath.Dir(store.Path))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want fs.FileMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, dirInfo.Mode().Perm(), 0o700,
		"directory mode = %o, want 700", dirInfo.Mode().Perm(),
	)

	dbInfo, err := os.Stat(store.Path)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want fs.FileMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, dbInfo.Mode().Perm(), 0o600,
		"database mode = %o, want 600", dbInfo.Mode().Perm(),
	)
}

func TestCommentsWritePrompt(t *testing.T) {
	var out bytes.Buffer
	comment := func() commentsComment {
		var body string = "Handle the nil case."
		return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	comment.Anchor = commentsAnchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, (func(w io.Writer, comments []commentsComment) error {
		if len(comments) == 0 {
			_, err := fmt.Fprintln(w, "No pending review comments.")
			return err
		}
		if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
			return err
		}
		for index, comment := range comments {
			if err := func() error {
				anchor := comment.Anchor
				_, headingErr := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, anchor.FilePath, func() string {
					var sides []string
					if anchor.OldStart > 0 {
						sides = append(sides, func() string {
							var side string = "old"
							if anchor.OldEnd == 0 || anchor.OldEnd == anchor.OldStart {
								return fmt.Sprintf("%s line %d", side, anchor.OldStart)
							}
							return fmt.Sprintf("%s lines %d-%d", side, anchor.OldStart, anchor.OldEnd)
						}())
					}
					if anchor.NewStart > 0 {
						sides = append(sides, func() string {
							var side string = "new"
							if anchor.NewEnd == 0 || anchor.NewEnd == anchor.NewStart {
								return fmt.Sprintf("%s line %d", side, anchor.NewStart)
							}
							return fmt.Sprintf("%s lines %d-%d", side, anchor.NewStart, anchor.NewEnd)
						}())
					}
					if len(sides) == 0 {
						return "diff lines"
					}
					return strings.Join(sides, ", ")
				}())
				if headingErr != nil {
					return headingErr
				}
				if err := func() error {
					if len(anchor.QuotedLines) == 0 {
						return nil
					}
					if _, err := fmt.Fprintln(w, "```diff"); err != nil {
						return err
					}
					for _, line := range anchor.QuotedLines {
						if _, err := fmt.Fprintln(w, line); err != nil {
							return err
						}
					}
					_, err := fmt.Fprintln(w, "```")
					return err
				}(); err != nil {
					return err
				}
				_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
				return err
			}(); err != nil {
				return err
			}
		}
		return nil
	})(&out, []commentsComment{comment}))

	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s", expected, out.String(),
		)
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(out.String(), "batch"), true,
		"output exposes internal batches:\n%s", out.String(),
	)
}

func TestCommentsWritePromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer
	comments := []commentsComment{
		func() commentsComment {
			var body string = "First."
			return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
		}(),
		func() commentsComment {
			var body string = "Second."
			return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
		}(),
	}
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, (func(w io.Writer, comments []commentsComment) error {
		if len(comments) == 0 {
			_, err := fmt.Fprintln(w, "No pending review comments.")
			return err
		}
		if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
			return err
		}
		for index, comment := range comments {
			if err := func() error {
				anchor := comment.Anchor
				_, headingErr := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, anchor.FilePath, func() string {
					var sides []string
					if anchor.OldStart > 0 {
						sides = append(sides, func() string {
							var side string = "old"
							if anchor.OldEnd == 0 || anchor.OldEnd == anchor.OldStart {
								return fmt.Sprintf("%s line %d", side, anchor.OldStart)
							}
							return fmt.Sprintf("%s lines %d-%d", side, anchor.OldStart, anchor.OldEnd)
						}())
					}
					if anchor.NewStart > 0 {
						sides = append(sides, func() string {
							var side string = "new"
							if anchor.NewEnd == 0 || anchor.NewEnd == anchor.NewStart {
								return fmt.Sprintf("%s line %d", side, anchor.NewStart)
							}
							return fmt.Sprintf("%s lines %d-%d", side, anchor.NewStart, anchor.NewEnd)
						}())
					}
					if len(sides) == 0 {
						return "diff lines"
					}
					return strings.Join(sides, ", ")
				}())
				if headingErr != nil {
					return headingErr
				}
				if err := func() error {
					if len(anchor.QuotedLines) == 0 {
						return nil
					}
					if _, err := fmt.Fprintln(w, "```diff"); err != nil {
						return err
					}
					for _, line := range anchor.QuotedLines {
						if _, err := fmt.Fprintln(w, line); err != nil {
							return err
						}
					}
					_, err := fmt.Fprintln(w, "```")
					return err
				}(); err != nil {
					return err
				}
				_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
				return err
			}(); err != nil {
				return err
			}
		}
		return nil
	})(&out, comments))

	for _, expected := range []string{"### 1.", "### 2."} {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s", expected, out.String(),
		)
	}
}

func TestCommentsStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	empty := commentsComment{ID: "", Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: ""}
	if _, err := store.Add(empty); err == nil {
		t.Fatal("empty comment was accepted")
	}
	large := func() commentsComment {
		var body string = strings.Repeat("x", commentsMaxCommentBytes+1)
		return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	if _, err := store.Add(large); err == nil {
		t.Fatal("oversized comment was accepted")
	}
}

func TestCommentsListAndUpdateComments(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.Add(func() commentsComment {
		var body string = "first"
		return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
		t.Fatal(err)
	}
	second := func() commentsComment {
		var body string = "second"
		return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()
	second.Anchor = commentsAnchor{FilePath: "other.go", NewStart: 2, NewEnd: 2}
	if _, err := store.Add(second); err != nil {
		t.Fatal(err)
	}

	comments, err := store.List("/repo")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(comments), 2, "comments = %#v", comments)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, comments[1].Body, "second", "comments = %#v",
		comments,
	)

	comments[1].Body = "edited"
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, store.Update(comments[1]))

	updated, err := store.List("/repo")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	if got := updated[1].Body; got != "edited" {
		t.Fatalf("body = %q, want edited", got)
	}
}

func TestCommentsDeleteCommentRemovesMessage(t *testing.T) {
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	if _, err := store.Add(func() commentsComment {
		var body string = "first"
		return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add(func() commentsComment {
		var body string = "second"
		return commentsComment{ID: body, Repository: "/repo", CreatedAt: time.Unix(1, 0).UTC(), Anchor: commentsAnchor{FilePath: "file.go", NewStart: 1, NewEnd: 1}, Body: body}
	}()); err != nil {
		t.Fatal(err)
	}
	comments, err := store.List("/repo")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, store.Delete("/repo", comments[0].ID))

	remaining, err := store.List("/repo")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(remaining), 1, "remaining = %#v", remaining)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, remaining[0].Body, "second", "remaining = %#v",
		remaining,
	)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, store.Delete("/repo", remaining[0].ID))

	remaining, err = store.List("/repo")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(remaining), 0, "remaining = %#v, want empty",
		remaining,
	)
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
			bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
			if err != nil {
				return err
			}
			return func(bucket *bolt.Bucket) error { return bucket.Put([]byte{0, 0, 0, 1}, legacy) }(bucket)
		})
	}()
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	comments, err := store.List("/repo")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(comments), 1, "legacy comment = %#v", comments)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, comments[0].ID, "legacy", "legacy comment = %#v",
		comments,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, comments[0].Body, "old", "legacy comment = %#v",
		comments,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, comments[0].Anchor.NewStart, 3,
		"legacy comment = %#v", comments,
	)

	comments[0].Body = "updated"
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, store.Update(comments[0]))

	updated, err := store.List("/repo")
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(updated), 1, "updated comments = %#v", updated)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, updated[0].Body, "updated",
		"updated comments = %#v", updated,
	)
}
