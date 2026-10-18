package comments

import (
	"path/filepath"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestStoreMutationsDoNotCrossRepositoryBoundary(t *testing.T) {
	for _, operation := range []string{"update", "delete", "acknowledge"} {
		t.Run(operation, func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			original, err := store.Add(Comment{
				ID: "id", Repository: "/other", Body: "keep this feedback",
			})
			testutil.NoError(t, err)
			var mutationErr error
			switch operation {
			case "update":
				mutationErr = store.Update(Comment{
					ID: original.ID, Repository: "/repo", Body: "replacement",
				})
			case "delete":
				mutationErr = store.Delete("/repo", original.ID)
			case "acknowledge":
				mutationErr = store.Acknowledge("/repo", []string{original.ID})
			default:
				t.Fatalf("unknown mutation %q", operation)
			}
			testutil.RequireEqual(t, mutationErr != nil,
				operation != "acknowledge", "mutation error = %v", mutationErr,
			)
			remaining, err := store.List("/other")
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(remaining), 1,
				"other repository comments = %#v", remaining,
			)
			testutil.RequireEqual(t, remaining[0].Body, original.Body,
				"other repository body = %q", remaining[0].Body,
			)
			local, err := store.List("/repo")
			testutil.NoError(t, err)
			testutil.RequireEqual(t, len(local), 0,
				"unexpected local comments = %#v", local,
			)
		})
	}
}
