package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("added one", mustFirst(t, oldView), forward)
	line, _ := oldView.line(cursor)
	changed := testPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := newUnifiedView(changed, true)
	translated, ok := newView.findCursor(identify(oldView, cursor))
	testutil.RequireEqual(t, ok, true, "semantic cursor was not found")
	testutil.RequireEqual(t, translated.row != cursor.row, true,
		"cursor coordinate was reused after rows shifted",
	)

	translatedLine, _ := newView.line(translated)
	testutil.RequireEqual(t, translatedLine, line,
		fmt.Sprintf("translated line = %#v, want %#v", translatedLine, line),
	)
}

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "deleted.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Deletion, Text: "gone", OldNumber: 1},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor, ok := v.first()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("first cursor = %#v, %v", cursor, ok),
	)
	testutil.RequireEqual(t, cursor.pane, left,
		fmt.Sprintf("first cursor = %#v, %v", cursor, ok),
	)

	line, _ := v.line(cursor)
	testutil.RequireEqual(t, line.Text, "gone",
		fmt.Sprintf("first line = %q", line.Text),
	)
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("removed two", mustFirst(t, oldView), forward)
	changed := testPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := newUnifiedView(changed, true)
	fallback, ok := newView.findCursor(identify(oldView, cursor))
	testutil.RequireEqual(t, ok, true, "nearby cursor was not found")

	fallbackLine, _ := newView.line(fallback)
	testutil.RequireEqual(t, fallbackLine.Kind, patch.Deletion,
		fmt.Sprintf("fallback kind = %v", fallbackLine.Kind),
	)
}
