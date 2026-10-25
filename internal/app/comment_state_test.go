package app

import (
	"errors"
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentMutationsInvalidateLoads(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*commentState)
		want   []comments.Comment
	}{
		{
			name: "add",
			mutate: func(s *commentState) {
				s.upsert(comments.Comment{Body: "added"}, -1)
			},
			want: []comments.Comment{{Body: "original"}, {Body: "added"}},
		},
		{
			name: "edit",
			mutate: func(s *commentState) {
				s.upsert(comments.Comment{Body: "edited"}, 0)
			},
			want: []comments.Comment{{Body: "edited"}},
		},
		{
			name:   "delete",
			mutate: func(s *commentState) { s.remove(0) },
			want:   []comments.Comment{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := commentState{
				items:    []comments.Comment{{Body: "original"}},
				revision: 4,
			}
			load := commentsLoadedMsg{
				comments: []comments.Comment{{Body: "stale"}},
				revision: state.revision,
			}
			tt.mutate(&state)
			testutil.RequireEqual(t, state.revision, uint64(5),
				"revision = %d", state.revision)
			loadErrors := []error{nil, errors.New("stale load failed")}
			for _, loadErr := range loadErrors {
				load.err = loadErr
				updated, err := state.applyLoad(load)
				testutil.NoError(t, err)
				testutil.CheckEqual(t, updated, false,
					"stale load updated comments")
				if !reflect.DeepEqual(state.items, tt.want) {
					t.Fatalf("comments = %#v, want %#v", state.items, tt.want)
				}
			}
		})
	}
}

func TestCommentLoadPreservesRevision(t *testing.T) {
	state := commentState{revision: 4}
	for _, body := range []string{"first load", "second load"} {
		items := []comments.Comment{{Body: body}}
		updated, err := state.applyLoad(commentsLoadedMsg{
			comments: items, revision: 4,
		})
		testutil.NoError(t, err)
		testutil.RequireEqual(t, updated, true,
			"current load did not update comments")
		testutil.CheckEqual(t, state.revision, uint64(4),
			"revision = %d", state.revision)
		if !reflect.DeepEqual(state.items, items) {
			t.Fatalf("comments = %#v, want %#v", state.items, items)
		}
	}
}

func TestFailedCommentLoadPreservesComments(t *testing.T) {
	items := []comments.Comment{{Body: "original"}}
	state := commentState{items: items, revision: 4}
	loadErr := errors.New("load failed")
	updated, err := state.applyLoad(commentsLoadedMsg{
		revision: 4, err: loadErr,
	})
	testutil.CheckEqual(t, err, loadErr, "error = %v, want %v", err, loadErr)
	testutil.CheckEqual(t, updated, false, "failed load updated comments")
	testutil.CheckEqual(t, state.revision, uint64(4),
		"revision = %d", state.revision)
	if !reflect.DeepEqual(state.items, items) {
		t.Fatalf("comments = %#v, want %#v", state.items, items)
	}
}
