package comments

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestDatabaseOpenSecuresExistingPaths(t *testing.T) {
	for _, operation := range []struct {
		name string
		mode databaseMode
	}{
		{"read", readDatabase},
		{"write", writeDatabase},
	} {
		t.Run(operation.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "state")
			store := Store{Path: filepath.Join(directory, "comments.db")}
			testutil.NoError(t, store.transact(func(_ *bolt.Bucket) error {
				return nil
			}))
			testutil.NoError(t, os.Chmod(directory, 0o755))
			testutil.NoError(t, os.Chmod(store.Path, 0o644))

			db, err := store.openDatabase(operation.mode)
			testutil.NoError(t, err)
			defer db.Close()

			for path, want := range map[string]os.FileMode{
				directory:  0o700,
				store.Path: 0o600,
			} {
				info, err := os.Stat(path)
				testutil.NoError(t, err)
				testutil.RequireEqual(
					t, info.Mode().Perm(), want, "permissions for %s", path,
				)
			}
		})
	}
}

func TestDatabaseTransactionRollsBackCallbackError(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	want := errors.New("callback failed")
	err := store.transact(func(bucket *bolt.Bucket) error {
		if err := bucket.Put([]byte("key"), []byte("value")); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("transaction error = %v, want %v", err, want)
	}

	testutil.NoError(t, store.view(func(_ *bolt.Bucket) error {
		t.Fatal("rolled-back bucket should not exist")
		return nil
	}))
}
