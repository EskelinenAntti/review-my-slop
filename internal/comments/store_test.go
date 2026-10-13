package comments

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
	testutil.RequireEqual(t, len(comments), 2,
		fmt.Sprintf("unexpected comments: %#v", comments),
	)
	testutil.RequireEqual(t, comments[0].Body, "first",
		fmt.Sprintf("unexpected comments: %#v", comments),
	)
	testutil.RequireEqual(t, comments[1].Body, "third",
		fmt.Sprintf("unexpected comments: %#v", comments),
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
		fmt.Sprintf("newer comment was not preserved: %#v", remainingA),
	)
	testutil.RequireEqual(t, remainingA[0].Body, "newer",
		fmt.Sprintf("newer comment was not preserved: %#v", remainingA),
	)

	remainingB, err := store.List("/repo/b")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(remainingB), 1,
		fmt.Sprintf("other repository comments = %d, want 1", len(remainingB)),
	)

	dirInfo, err := os.Stat(filepath.Dir(store.Path))
	testutil.NoError(t, err)
	testutil.RequireEqual(t, dirInfo.Mode().Perm(), 0o700,
		fmt.Sprintf("directory mode = %o, want 700", dirInfo.Mode().Perm()),
	)

	dbInfo, err := os.Stat(store.Path)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, dbInfo.Mode().Perm(), 0o600,
		fmt.Sprintf("database mode = %o, want 600", dbInfo.Mode().Perm()),
	)
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
	testutil.RequireEqual(t, len(comments), 2,
		fmt.Sprintf("comments = %#v", comments),
	)
	testutil.RequireEqual(t, comments[1].Body, "second",
		fmt.Sprintf("comments = %#v", comments),
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
	testutil.RequireEqual(t, len(remaining), 1,
		fmt.Sprintf("remaining = %#v", remaining),
	)
	testutil.RequireEqual(t, remaining[0].Body, "second",
		fmt.Sprintf("remaining = %#v", remaining),
	)
	testutil.NoError(t, store.Delete("/repo", remaining[0].ID))

	remaining, err = store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(remaining), 0,
		fmt.Sprintf("remaining = %#v, want empty", remaining),
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
