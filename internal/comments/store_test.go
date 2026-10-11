package comments

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestStoreQueuesByRepositoryAndDeletesExactPeek(t *testing.T) {
	store := Store{
		Path: filepath.Join(t.TempDir(), "state", fixtureCommentsDb),
	}
	first := testComment(fixtureRepoA, fixtureFirst)
	second := testComment("/repo/b", "other")
	third := testComment(fixtureRepoA, "third")
	for _, comment := range []Comment{first, second, third} {
		{
			_, err := store.Add(comment)
			requireEqual(t, err == nil, true, fixtureV, err)
		}
	}

	comments, err := store.List(fixtureRepoA)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(comments) == two && comments[zero].Body == fixtureFirst &&
			comments[one].Body == "third",
		true,
		"unexpected comments: %#v",
		comments,
	)

	{
		_, err := store.Add(testComment(fixtureRepoA, "newer"))
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		err := store.Acknowledge(
			fixtureRepoA,
			[]string{comments[zero].ID, comments[one].ID},
		)
		requireEqual(t, err == nil, true, fixtureV, err)
	}

	remainingA, err := store.List(fixtureRepoA)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(remainingA) == one && remainingA[zero].Body == "newer", true,
		"newer comment was not preserved: %#v",
		remainingA,
	)
	remainingB, err := store.List("/repo/b")
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(remainingB), one,
		"other repository comments = %d, want 1",
		len(remainingB),
	)

	dirInfo, err := os.Stat(filepath.Dir(store.Path))
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		dirInfo.Mode().Perm(), privateDirMode,
		"directory mode = %o, want 700",
		dirInfo.Mode().Perm(),
	)
	dbInfo, err := os.Stat(store.Path)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		dbInfo.Mode().Perm(), privateFileMode,
		"database mode = %o, want 600",
		dbInfo.Mode().Perm(),
	)
}

func TestWritePrompt(t *testing.T) {
	var out bytes.Buffer
	comment := testComment(fixtureRepository, "Handle the nil case.")
	comment.Anchor = Anchor{
		FilePath: "main.go",
		OldStart: ten,
		OldEnd:   fixtureNumber11,
		NewStart: fixtureNumber12,
		NewEnd:   fixtureNumber13,

		QuotedLines: []string{"-old()", "+new()"},
	}
	{
		err := WritePrompt(&out, []Comment{comment})
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	for _, expected := range []string{
		"New comments since last run:",
		"`main.go`",
		"old lines 10-11",
		"new lines 12-13",
		"```diff",
		"Handle the nil case.",
	} {
		requireEqual(
			t,
			strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s",
			expected,
			out.String(),
		)
	}
	requireEqual(
		t,
		!strings.Contains(out.String(), "batch"), true,
		"output exposes internal batches:\n%s",
		out.String(),
	)
}

func TestWritePromptNumbersMessages(t *testing.T) {
	var out bytes.Buffer
	{
		err := WritePrompt(&out, []Comment{
			testComment(fixtureRepository, "First."),
			testComment(fixtureRepository, "Second."),
		})
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	for _, expected := range []string{"### 1.", "### 2."} {
		requireEqual(
			t,
			strings.Contains(out.String(), expected), true,
			"output lacks %q:\n%s",
			expected,
			out.String(),
		)
	}
}

func TestStoreRejectsEmptyAndOversizedComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), fixtureCommentsDb)}
	empty := testComment(fixtureRepository, "")
	{
		_, err := store.Add(empty)
		requireEqual(
			t,
			err != nil,
			true,
			fixtureV,
			"empty comment was accepted",
		)
	}
	large := testComment(
		fixtureRepository,
		strings.Repeat("x", maxCommentBytes+one),
	)
	{
		_, err := store.Add(large)
		requireEqual(
			t,
			err != nil,
			true,
			fixtureV,
			"oversized comment was accepted",
		)
	}
}

func TestListAndUpdateComments(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), fixtureCommentsDb)}
	{
		_, err := store.Add(testComment(fixtureRepository, fixtureFirst))
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	second := testComment(fixtureRepository, "second")
	second.Anchor = Anchor{FilePath: "other.go", NewStart: two, NewEnd: two}
	{
		_, err := store.Add(second)
		requireEqual(t, err == nil, true, fixtureV, err)
	}

	comments, err := store.List(fixtureRepository)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(comments) == two && comments[one].Body == "second", true,
		"comments = %#v",
		comments,
	)
	comments[one].Body = "edited"
	{
		err := store.Update(comments[one])
		requireEqual(t, err == nil, true, fixtureV, err)
	}

	updated, err := store.List(fixtureRepository)
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		got := updated[one].Body
		requireEqual(t, got, "edited", "body = %q, want edited", got)
	}
}

func TestDeleteCommentRemovesMessage(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), fixtureCommentsDb)}
	{
		_, err := store.Add(testComment(fixtureRepository, fixtureFirst))
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		_, err := store.Add(testComment(fixtureRepository, "second"))
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	comments, err := store.List(fixtureRepository)
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		err := store.Delete(fixtureRepository, comments[zero].ID)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	remaining, err := store.List(fixtureRepository)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(remaining) == one && remaining[zero].Body == "second", true,
		"remaining = %#v",
		remaining,
	)
	{
		err := store.Delete(fixtureRepository, remaining[zero].ID)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	remaining, err = store.List(fixtureRepository)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(remaining), zero,
		"remaining = %#v, want empty",
		remaining,
	)
}

func TestLegacyMessageCanBeReadAndUpdated(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), fixtureCommentsDb)}
	legacy := []byte(
		"{\"id\":\"legacy\",\"repository\":\"/repo\",\"cre" +
			"ated_at\":\"1970-01-01T00:00:01Z\",\"diff_fi" +
			"ngerprint\":\"unused\",\"comment\":{\"anchor\":" +
			"{\"file\":\"main.go\",\"hunk\":\"@@\",\"start_row" +
			"\":1,\"end_row\":2,\"new_start\":3,\"quoted_li" +
			"nes\":[\"+line\"]},\"body\":\"old\"}}",
	)
	{
		err := store.writeComments(func(bucket *bolt.Bucket) error {
			return bucket.Put([]byte{zero, zero, zero, one}, legacy)
		})
		requireEqual(t, err == nil, true, fixtureV, err)
	}

	comments, err := store.List(fixtureRepository)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(comments) == one && comments[zero].ID == "legacy" &&
			comments[zero].Body == "old" &&
			comments[zero].Anchor.NewStart == 3,
		true,
		"legacy comment = %#v",
		comments,
	)
	comments[zero].Body = "updated"
	{
		err := store.Update(comments[zero])
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	updated, err := store.List(fixtureRepository)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(updated) == one && updated[zero].Body == "updated", true,
		"updated comments = %#v",
		updated,
	)
}

func testComment(repository, body string) Comment {
	return Comment{
		ID:         body,
		Repository: repository,
		CreatedAt:  time.Unix(one, zero).UTC(),
		Anchor:     Anchor{FilePath: "file.go", NewStart: one, NewEnd: one},
		Body:       body,
	}
}
