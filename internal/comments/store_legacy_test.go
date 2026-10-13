package comments

import (
	"fmt"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
	testutil.RequireEqual(t, len(comments), 1,
		fmt.Sprintf("legacy comment = %#v", comments),
	)
	testutil.RequireEqual(t, comments[0].ID, "legacy",
		fmt.Sprintf("legacy comment = %#v", comments),
	)
	testutil.RequireEqual(t, comments[0].Body, "old",
		fmt.Sprintf("legacy comment = %#v", comments),
	)
	testutil.RequireEqual(t, comments[0].Anchor.NewStart, 3,
		fmt.Sprintf("legacy comment = %#v", comments),
	)

	comments[0].Body = "updated"
	testutil.NoError(t, store.Update(comments[0]))

	updated, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(updated), 1,
		fmt.Sprintf("updated comments = %#v", updated),
	)
	testutil.RequireEqual(t, updated[0].Body, "updated",
		fmt.Sprintf("updated comments = %#v", updated),
	)
}
