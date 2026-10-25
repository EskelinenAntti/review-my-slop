package comments

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestStoreMutationsResolveLegacyKeys(t *testing.T) {
	for _, operation := range []string{"update", "delete", "acknowledge"} {
		t.Run(operation, func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			target := testComment("/repo", "target")
			other := testComment("/other", "other")
			legacyKey := []byte{0, 0, 0, 1}
			err := store.transact(func(bucket commentBucket) error {
				for index, comment := range []Comment{target, other} {
					data, err := encodeComment(comment)
					if err != nil {
						return err
					}
					key := []byte{0, 0, 0, byte(index + 1)}
					if err := bucket.bucket.Put(key, data); err != nil {
						return err
					}
				}
				return nil
			})
			testutil.NoError(t, err)

			switch operation {
			case "update":
				target.Body = "updated"
				err = store.Update(target)
			case "delete":
				err = store.Delete(target.Repository, target.ID)
			case "acknowledge":
				err = store.Acknowledge(target.Repository, []string{
					target.ID, target.ID, other.ID, "missing",
				})
			default:
				t.Fatalf("unknown operation %q", operation)
			}
			testutil.NoError(t, err)

			remaining, err := store.List(target.Repository)
			testutil.NoError(t, err)
			if operation == "update" {
				testutil.RequireEqual(t, len(remaining), 1,
					"updated comments = %#v", remaining)
				testutil.CheckEqual(t, remaining[0].Body, target.Body,
					"updated body")
			} else {
				testutil.CheckEqual(t, len(remaining), 0,
					"remaining comments = %#v", remaining)
			}
			remaining, err = store.List(other.Repository)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(remaining), 1,
				"other repository comments = %#v", remaining)
			testutil.CheckEqual(t, remaining[0].Body, other.Body,
				"other repository body")

			err = store.view(func(bucket commentBucket) error {
				testutil.CheckEqual(t, bucket.bucket.Get(legacyKey) == nil,
					true, "legacy key was removed")
				testutil.CheckEqual(t,
					bucket.bucket.Get([]byte(target.ID)) != nil,
					operation == "update", "updated record uses comment ID")
				return nil
			})
			testutil.NoError(t, err)
		})
	}
}

func TestAcknowledgePreservesRecordsOnDecodeError(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	comment := testComment("/repo", "feedback")
	data, err := encodeComment(comment)
	testutil.NoError(t, err)
	err = store.transact(func(bucket commentBucket) error {
		if err := bucket.add(comment.ID, data); err != nil {
			return err
		}
		return bucket.bucket.Put([]byte("zzz-corrupt"), []byte("not JSON"))
	})
	testutil.NoError(t, err)

	err = store.Acknowledge(comment.Repository, []string{comment.ID})
	testutil.RequireEqual(t, err != nil, true, "corrupt record was accepted")
	err = store.view(func(bucket commentBucket) error {
		testutil.CheckEqual(t,
			bytes.Equal(bucket.bucket.Get([]byte(comment.ID)), data),
			true, "failed acknowledgment preserved the original record")
		return nil
	})
	testutil.NoError(t, err)
}
