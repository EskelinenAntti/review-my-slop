package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchKeepsRememberedTermAfterUnsuccessfulSessions(t *testing.T) {
	for _, tt := range []struct {
		name   string
		finish func(*View)
	}{
		{"accept empty", (*View).AcceptSearch},
		{"accept missing", func(v *View) {
			v.InsertSearch("missing")
			v.AcceptSearch()
		}},
		{"cancel", func(v *View) {
			v.InsertSearch("added")
			v.CancelSearch()
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := New(testPatch(), Options{})
			v.InsertSearch("removed")
			v.AcceptSearch()
			first := v.cursor

			v.BeginSearch()
			tt.finish(v)

			testutil.RequireEqual(t, v.search.active, false,
				"session still active",
			)
			testutil.RequireEqual(t, v.search.query, "",
				"query was not cleared",
			)
			testutil.RequireEqual(t, v.search.term, "removed",
				"remembered term changed",
			)
			testutil.RequireEqual(t, v.Find(Forward), true,
				"remembered match not found",
			)
			testutil.RequireEqual(t, v.cursor != first, true,
				"repeat did not advance",
			)
			_, line, ok := v.Current()
			testutil.RequireEqual(t, ok, true, "repeat did not focus code")
			testutil.RequireEqual(t, line.Text, "removed two",
				"repeat used another term",
			)
		})
	}
}

func TestInactiveSearchEditingDoesNotChangeState(t *testing.T) {
	v := New(testPatch(), Options{})
	v.InsertSearch("removed")
	v.AcceptSearch()
	cursor, viewport, search := v.cursor, v.viewport, v.search

	v.BackspaceSearch()
	v.AcceptSearch()
	v.CancelSearch()
	testutil.RequireEqual(t, v.Find(Direction(0)), false,
		"invalid direction matched",
	)
	testutil.RequireEqual(t, v.cursor, cursor, "inactive search changed focus")
	testutil.RequireEqual(t, v.viewport, viewport, "inactive search scrolled")
	testutil.RequireEqual(t, v.search, search, "inactive search changed state")
}
