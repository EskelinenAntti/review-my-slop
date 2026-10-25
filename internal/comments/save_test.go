package comments

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSaveAddsCommentWithoutID(t *testing.T) {
	for _, createdAt := range []time.Time{{}, time.Unix(1, 0).UTC()} {
		t.Run(createdAt.String(), func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			comment := testComment("/repo", "feedback")
			comment.ID, comment.CreatedAt = "", createdAt

			saved, err := store.Save(comment)
			testutil.NoError(t, err)
			testutil.RequireEqual(t, saved.ID != "", true, "missing ID")
			testutil.RequireEqual(t, saved.CreatedAt.IsZero(), false,
				"missing creation time")
			if !createdAt.IsZero() {
				testutil.CheckEqual(t, saved.CreatedAt, createdAt,
					"creation time")
			}
			comment.ID, comment.CreatedAt = saved.ID, saved.CreatedAt
			testutil.CheckEqual(t, reflect.DeepEqual(saved, comment), true,
				"saved = %#v, want %#v", saved, comment)
			stored, err := store.List(comment.Repository)
			testutil.NoError(t, err)
			testutil.CheckEqual(t,
				reflect.DeepEqual(stored, []Comment{saved}), true,
				"stored = %#v, want %#v", stored, saved,
			)
		})
	}
}

func TestSaveUpdatesCommentWithID(t *testing.T) {
	store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	comment, err := store.Add(testComment("/repo", "feedback"))
	testutil.NoError(t, err)
	comment.Body = "edited"
	comment.Anchor = Anchor{FilePath: "other.go", NewStart: 2, NewEnd: 3}

	saved, err := store.Save(comment)
	testutil.NoError(t, err)
	testutil.CheckEqual(t, reflect.DeepEqual(saved, comment), true,
		"saved = %#v, want %#v", saved, comment)
	stored, err := store.List(comment.Repository)
	testutil.NoError(t, err)
	testutil.CheckEqual(t, reflect.DeepEqual(stored, []Comment{comment}), true,
		"stored = %#v, want %#v", stored, comment)
}

func TestSaveRejectsInvalidChanges(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Comment)
	}{
		{"unknown ID", func(c *Comment) { c.ID = "missing" }},
		{"wrong repository", func(c *Comment) { c.Repository = "/other" }},
		{"missing repository", func(c *Comment) { c.Repository = "" }},
		{"empty update", func(c *Comment) { c.Body = "" }},
		{"oversized update", func(c *Comment) {
			c.Body = strings.Repeat("x", maxCommentBytes+1)
		}},
		{"empty addition", func(c *Comment) { c.ID, c.Body = "", "" }},
		{"oversized addition", func(c *Comment) {
			c.ID, c.Body = "", strings.Repeat("x", maxCommentBytes+1)
		}},
		{"addition without repository", func(c *Comment) {
			c.ID, c.Repository = "", ""
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := Store{Path: filepath.Join(t.TempDir(), "comments.db")}
			original, err := store.Add(testComment("/repo", "feedback"))
			testutil.NoError(t, err)
			changed := original
			test.change(&changed)

			saved, err := store.Save(changed)
			testutil.RequireEqual(t, err != nil, true, "invalid save succeeded")
			testutil.CheckEqual(t, reflect.DeepEqual(saved, Comment{}), true,
				"failed save returned %#v", saved)
			stored, err := store.List(original.Repository)
			testutil.NoError(t, err)
			testutil.CheckEqual(t,
				reflect.DeepEqual(stored, []Comment{original}), true,
				"stored = %#v, want unchanged %#v", stored, original,
			)
			other, err := store.List("/other")
			testutil.NoError(t, err)
			testutil.CheckEqual(t, len(other), 0,
				"unexpected comments: %#v", other)
		})
	}
}
