package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchBackspaceRemovesUnicodeCodePoint(t *testing.T) {
	v := New(testPatch(), Options{})
	origin := v.cursor
	v.InsertSearch("removed")
	match := v.cursor
	v.InsertSearch("界")
	testutil.RequireEqual(t, v.search.miss, true, "extended query should miss")
	testutil.RequireEqual(t, v.cursor, match, "miss should retain focus")
	v.BackspaceSearch()
	testutil.RequireEqual(t, v.search.query, "removed",
		"backspace should remove one code point",
	)
	testutil.RequireEqual(t, v.search.miss, false,
		"shortened query should match",
	)
	testutil.RequireEqual(t, v.cursor, match,
		"shortened query should restore match",
	)
	for range len("removed") {
		v.BackspaceSearch()
	}
	testutil.RequireEqual(t, v.cursor, origin,
		"empty query should restore origin",
	)
	testutil.RequireEqual(t, v.search.miss, false,
		"empty query should not miss",
	)
}

func TestUnsuccessfulSearchPreservesRememberedTerm(t *testing.T) {
	for _, query := range []string{"", "missing"} {
		t.Run(query, func(t *testing.T) {
			v := New(testPatch(), Options{})
			v.InsertSearch("removed")
			v.AcceptSearch()
			v.BeginSearch()
			v.InsertSearch(query)
			v.AcceptSearch()
			testutil.RequireEqual(t, v.search.term, "removed",
				"unsuccessful search should preserve term",
			)
			testutil.RequireEqual(t, v.search.active, false,
				"accept should end search",
			)
			testutil.RequireEqual(t, v.search.query, "",
				"accept should clear query",
			)
			testutil.RequireEqual(t, v.Find(Forward), true,
				"remembered search should still find matches",
			)
		})
	}
}
