package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchCompletionPreservesRememberedTerm(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		cancel      bool
	}{
		{name: "accept empty query"},
		{name: "accept missing query", query: "missing"},
		{name: "cancel successful query", query: "added", cancel: true},
		{name: "cancel missing query", query: "missing", cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := New(testPatch(), Options{SideBySide: true})
			v.Resize(120, 10)
			v.InsertSearch("removed")
			v.AcceptSearch()
			origin := v.cursor

			v.BeginSearch()
			v.InsertSearch(tc.query)
			if tc.cancel {
				v.CancelSearch()
			} else {
				v.AcceptSearch()
			}

			testutil.RequireEqual(t, v.search.term, "removed",
				"remembered term changed")
			testutil.RequireEqual(t, v.search.active, false,
				"search remains active")
			testutil.RequireEqual(t, v.search.miss, false,
				"search miss remains active")
			testutil.RequireEqual(t, v.search.query, "",
				"query was not cleared")
			testutil.RequireEqual(t, v.cursor, origin, "focus was not restored")
			testutil.RequireEqual(t,
				strings.Contains(v.Render(nil), "no matches"), false,
				"stale search status")
			testutil.RequireEqual(t, v.Find(Forward), true,
				"remembered search failed")
		})
	}
}

func TestInactiveSearchEditingDoesNotChangeView(t *testing.T) {
	v := New(testPatch(), Options{})
	v.InsertSearch("removed")
	v.AcceptSearch()
	v.Move(NextLine)
	cursor, viewport, search := v.cursor, v.viewport, v.search

	v.AcceptSearch()
	v.CancelSearch()
	v.BackspaceSearch()
	testutil.RequireEqual(t, v.Find(Direction(0)), false,
		"invalid direction matched")

	testutil.RequireEqual(t, v.cursor, cursor, "inactive search moved focus")
	testutil.RequireEqual(t, v.viewport, viewport,
		"inactive search moved viewport")
	testutil.RequireEqual(t, v.search, search, "inactive search changed state")
}
