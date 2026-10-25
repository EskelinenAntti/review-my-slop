package comments

import (
	"errors"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestStoreRejectsEmptyDatabasePath(t *testing.T) {
	for _, operation := range []string{"read", "write"} {
		t.Run(operation, func(t *testing.T) {
			store := Store{}
			var err error
			if operation == "read" {
				_, err = store.List("/repo")
			} else {
				_, err = store.Add(testComment("/repo", "feedback"))
			}
			if err == nil || err.Error() != "comments path is empty" {
				t.Fatalf("error = %v, want empty path error", err)
			}
		})
	}
}

func TestStoreViewDoesNotCreateMissingBucket(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	db, err := bolt.Open(store.Path, 0o600, nil)
	testutil.NoError(t, err)
	testutil.NoError(t, db.Close())

	items, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(items), 0, "comments = %#v", items)

	db, err = bolt.Open(store.Path, 0o600, nil)
	testutil.NoError(t, err)
	defer db.Close()
	err = db.View(func(tx *bolt.Tx) error {
		if tx.Bucket(commentsBucket) != nil {
			t.Error("read created the comments bucket")
		}
		return nil
	})
	testutil.NoError(t, err)
}

func TestStoreTransactionRollsBackCallbackError(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	comment := testComment("/repo", "feedback")
	data, err := encodeComment(comment)
	testutil.NoError(t, err)
	want := errors.New("abort transaction")
	err = store.transact(func(bucket commentBucket) error {
		if err := bucket.add(comment.ID, data); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want callback error", err)
	}

	items, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(items), 0, "comments = %#v", items)
	_, err = store.Add(comment)
	testutil.NoError(t, err)
}
