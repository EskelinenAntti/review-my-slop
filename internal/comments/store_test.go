package comments

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	bolt "go.etcd.io/bbolt"
)

func TestStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "state", "comments.db")}
	first := testComment("/repo/a", "first")
	second := testComment("/repo/b", "other")
	third := testComment("/repo/a", "third")
	for _, comment := range []Comment{first, second, third} {
		{
			_, err := store.Add(comment)
			testutil.Require(t, err == nil, err)
		}
	}

	comments, err := store.List("/repo/a")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(comments) == 2 && comments[0].Body == "first" && comments[1].Body == "third", "unexpected comments: %#v", comments)

	{
		_, err := store.Add(testComment("/repo/a", "newer"))
		testutil.Require(t, err == nil, err)
	}

	testutil.NoError(t, store.Acknowledge("/repo/a", []string{comments[0].ID, comments[1].ID}))

	remainingA, err := store.List("/repo/a")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(remainingA) == 1 && remainingA[0].Body == "newer", "newer comment was not preserved: %#v", remainingA)
	remainingB, err := store.List("/repo/b")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(remainingB) == 1, "other repository comments = %d, want 1", len(remainingB))

	dirInfo, err := os.Stat(filepath.Dir(store.Path))
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, dirInfo.Mode().Perm() == 0o700, "directory mode = %o, want 700", dirInfo.Mode().Perm())
	dbInfo, err := os.Stat(store.Path)
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, dbInfo.Mode().Perm() == 0o600, "database mode = %o, want 600", dbInfo.Mode().Perm())
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
		testutil.Requiref(t, strings.Contains(out.String(), expected), "output lacks %q:\n%s", expected, out.String())
	}
	testutil.Requiref(t, !strings.Contains(out.String(), "batch"), "output exposes internal batches:\n%s", out.String())
}

func TestWritePromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer

	testutil.NoError(t, WritePrompt(&out, []Comment{
		testComment("/repo", "First."),
		testComment("/repo", "Second."),
	}),
	)

	for _, expected := range []string{"### 1.", "### 2."} {
		testutil.Requiref(t, strings.Contains(out.String(), expected), "output lacks %q:\n%s", expected, out.String())
	}
}

func TestStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	empty := testComment("/repo", "")
	{
		_, err := store.Add(empty)
		testutil.Require(t, err != nil, "empty comment was accepted")
	}
	large := testComment("/repo", strings.Repeat("x", maxCommentBytes+1))
	{
		_, err := store.Add(large)
		testutil.Require(t, err != nil, "oversized comment was accepted")
	}
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	{
		_, err := store.Add(testComment("/repo", "first"))
		testutil.Require(t, err == nil, err)
	}
	second := testComment("/repo", "second")
	second.Anchor = Anchor{FilePath: "other.go", NewStart: 2, NewEnd: 2}
	{
		_, err := store.Add(second)
		testutil.Require(t, err == nil, err)
	}

	comments, err := store.List("/repo")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(comments) == 2 && comments[1].Body == "second", "comments = %#v", comments)
	comments[1].Body = "edited"

	testutil.NoError(t, store.Update(comments[1]))

	updated, err := store.List("/repo")
	testutil.Require(t, err == nil, err)
	{
		got := updated[1].Body
		testutil.Requiref(t, got == "edited", "body = %q, want edited", got)
	}
}

func TestDeleteCommentRemovesMessage(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	{
		_, err := store.Add(testComment("/repo", "first"))
		testutil.Require(t, err == nil, err)
	}
	{
		_, err := store.Add(testComment("/repo", "second"))
		testutil.Require(t, err == nil, err)
	}
	comments, err := store.List("/repo")
	testutil.Require(t, err == nil, err)

	testutil.NoError(t, store.Delete("/repo", comments[0].ID))

	remaining, err := store.List("/repo")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(remaining) == 1 && remaining[0].Body == "second", "remaining = %#v", remaining)

	testutil.NoError(t, store.Delete("/repo", remaining[0].ID))

	remaining, err = store.List("/repo")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(remaining) == 0, "remaining = %#v, want empty", remaining)
}

func TestLegacyMessageCanBeReadAndUpdated(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	legacy := []byte(`{"id":"legacy","repository":"/repo","created_at":"1970-01-01T00:00:01Z","diff_fingerprint":"unused","comment":{"anchor":{"file":"main.go","hunk":"@@","start_row":1,"end_row":2,"new_start":3,"quoted_lines":["+line"]},"body":"old"}}`)

	testutil.NoError(t, store.update(func(bucket *bolt.Bucket) error {
		return bucket.Put([]byte{0, 0, 0, 1}, legacy)
	}),
	)

	comments, err := store.List("/repo")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(comments) == 1 && comments[0].ID == "legacy" && comments[0].Body == "old" && comments[0].Anchor.NewStart == 3, "legacy comment = %#v", comments)
	comments[0].Body = "updated"

	testutil.NoError(t, store.Update(comments[0]))

	updated, err := store.List("/repo")
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(updated) == 1 && updated[0].Body == "updated", "updated comments = %#v", updated)
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
