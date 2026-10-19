package comments

import (
	"errors"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestDatabaseRejectsEmptyPath(t *testing.T) {
	for _, mode := range []databaseMode{readDatabase, writeDatabase} {
		_, err := (Store{}).openDatabase(mode)
		if err == nil || err.Error() != "comments path is empty" {
			t.Fatalf("mode %d: error = %v, want empty path error", mode, err)
		}
	}
}

func TestViewWithoutCommentsBucket(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	db, err := store.openDatabase(writeDatabase)
	testutil.NoError(t, err)
	testutil.NoError(t, db.Close())

	err = store.view(func(_ *bolt.Bucket) error {
		t.Fatal("view called for missing comments bucket")
		return nil
	})
	testutil.NoError(t, err)

	// A read must not initialize the bucket in an existing database.
	db, err = store.openDatabase(readDatabase)
	testutil.NoError(t, err)
	defer db.Close()
	testutil.NoError(t, db.View(func(tx *bolt.Tx) error {
		if tx.Bucket(commentsBucket) != nil {
			t.Fatal("view created the comments bucket")
		}
		return nil
	}))
}

func TestTransactionRollsBackCallbackError(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	comment, err := store.Add(testComment("/repo", "original"))
	testutil.NoError(t, err)
	wantErr := errors.New("callback failed")

	err = store.transact(func(bucket *bolt.Bucket) error {
		if err := bucket.Delete([]byte(comment.ID)); err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("transact error = %v, want %v", err, wantErr)
	}

	remaining, err := store.List("/repo")
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(remaining), 1, "comments = %#v", remaining)
	testutil.RequireEqual(t, remaining[0].ID, comment.ID,
		"comment was not preserved: %#v", remaining,
	)
}
