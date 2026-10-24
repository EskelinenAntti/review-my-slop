package app

import (
	"errors"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentReloadIgnoresResultsQueuedBeforeLocalChanges(t *testing.T) {
	for _, mutation := range []string{"save", "delete"} {
		for _, result := range []string{"success", "failure"} {
			t.Run(mutation+"/"+result, func(t *testing.T) {
				original := comments.Comment{ID: "one", Body: "original"}
				m := testModel(modelPatch(), []comments.Comment{original}, nil)
				m.load = func() ([]comments.Comment, error) {
					if result == "failure" {
						return nil, errors.New("storage unavailable")
					}
					return []comments.Comment{original}, nil
				}
				cmd := m.loadComments()
				testutil.RequireEqual(t, cmd != nil, true,
					"missing load command")

				if mutation == "save" {
					edited := original
					edited.Body = "edited"
					m.updateSavedComment(edited, 0)
				} else {
					m.delete = func(comments.Comment) error { return nil }
					m.deleteComment(0)
				}
				err := errors.New("current error")
				m.err = err
				before := m.commentView.Render(nil)
				m = updateModel(t, m, cmd())

				testutil.RequireEqual(t, m.err, err, "stale load changed error")
				testutil.RequireEqual(t, m.commentView.Render(nil), before,
					"stale load changed comment presentation")
				if mutation == "save" {
					testutil.RequireEqual(t, len(m.comments.items), 1,
						"stale load removed edited comment")
					testutil.RequireEqual(t, m.comments.items[0].Body, "edited",
						"stale load replaced edited comment")
				} else {
					testutil.RequireEqual(t, len(m.comments.items), 0,
						"stale load restored deleted comment")
				}
			})
		}
	}
}
