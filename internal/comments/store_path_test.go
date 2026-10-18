package comments

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestStoreRejectsEmptyDatabasePath(t *testing.T) {
	store := Store{}
	comment := Comment{ID: "id", Repository: "/repo", Body: "feedback"}
	operations := map[string]func() error{
		"add": func() error {
			_, err := store.Add(comment)
			return err
		},
		"list": func() error {
			_, err := store.List(comment.Repository)
			return err
		},
		"update": func() error { return store.Update(comment) },
		"delete": func() error {
			return store.Delete(comment.Repository, comment.ID)
		},
		"acknowledge": func() error {
			return store.Acknowledge(comment.Repository, []string{comment.ID})
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			err := operation()
			if err == nil {
				t.Fatal("expected an empty database path error")
			}
			testutil.RequireEqual(t, err.Error(), "comments path is empty",
				"error = %v", err,
			)
		})
	}
}

func TestAcknowledgeNothingDoesNotRequireDatabasePath(t *testing.T) {
	testutil.NoError(t, (Store{}).Acknowledge("/repo", nil))
}
