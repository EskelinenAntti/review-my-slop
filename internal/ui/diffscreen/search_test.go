package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchProjectionPreservesTraversalAndMetadataFocus(t *testing.T) {
	p := patch.Patch{Files: []patch.File{
		{
			DisplayPath: "first.go",
			Metadata:    []string{"col\x1b[31mored metadata\x1b[0m"},
			Hunks: []patch.Hunk{{Header: "@@ section @@", Lines: []patch.Line{
				{Kind: patch.Context, Text: "before"},
				{Kind: patch.Deletion, Text: "needle old"},
				{Kind: patch.Addition, Text: "needle new"},
				{Kind: patch.Context, Text: "after"},
			}}},
		},
		{DisplayPath: "empty.go", Metadata: []string{"no code here"}},
		{
			DisplayPath: "last.go",
			Hunks: []patch.Hunk{{Lines: []patch.Line{
				{Kind: patch.Context, Text: "last file"},
			}}},
		},
	}}
	for _, tc := range []struct {
		name      string
		split     bool
		pane      diffPane
		direction Direction
		query     string
		want      string
	}{
		{"unified forward", false, right, Forward, "NEEDLE", "needle old"},
		{"unified backward", false, right, Backward, "needle", "needle new"},
		{"split old pane", true, left, Forward, "needle", "needle old"},
		{"split new pane", true, right, Forward, "needle", "needle new"},
		{"split other pane", true, left, Forward, "needle new", "needle new"},
		{"file header", true, right, Forward, "first.go", "before"},
		{"hunk header", true, right, Backward, "section", "before"},
		{"colored metadata", false, right, Forward, "colored", "before"},
		{"skip metadata without code", true, right, Forward, "empty.go", ""},
		{"exclude starting row", true, right, Forward, "before", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: tc.split})
			from := mustFirst(t, v)
			from.pane = tc.pane
			cursor, ok := v.search(tc.query, from, tc.direction)
			testutil.RequireEqual(t, ok, tc.want != "", "query=%q", tc.query)
			if ok {
				line, valid := v.line(cursor)
				testutil.RequireEqual(t, valid, true,
					"search did not focus code",
				)
				testutil.RequireEqual(t, line.Text, tc.want,
					"query=%q", tc.query,
				)
			}
		})
	}
}

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
