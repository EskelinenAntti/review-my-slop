package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	first := mustFirst(t, old)
	cursor, ok := old.search("added one", first, forward)
	testutil.RequireEqual(t, ok, true, "%s", "cursor not found")

	selection := beginSelection(first)
	selection, ok = old.extendSelection(selection, cursor)
	testutil.RequireEqual(t, ok, true, "%s", "selection not created")

	viewport := old.newViewport(100, 4)
	viewport = old.align(viewport, cursor, middle)
	screen := &View{
		view:      old,
		cursor:    cursor,
		selection: &selection,
		viewport:  viewport,
	}

	next := newSideBySideView(testPatch(), true)
	screen.replaceView(next)
	preserved := screen
	testutil.RequireEqual(t, next.valid(preserved.cursor), true, "%s",
		"cursor was not preserved",
	)

	line, ok := next.line(preserved.cursor)
	testutil.RequireEqual(t, ok, true, "cursor line = %#v, ok=%v", line, ok)
	testutil.RequireEqual(t, line.Text, "added one", "cursor line = %#v, ok=%v",
		line, ok,
	)
	testutil.RequireEqual(t, preserved.selection != nil, true,
		"selection = %#v", preserved.selection,
	)

	firstLine, firstOK := next.line(preserved.selection.First)
	lastLine, lastOK := next.line(preserved.selection.Last)
	testutil.RequireEqual(t, firstOK, true, "selection endpoints = %#v, %#v",
		firstLine, lastLine,
	)
	testutil.RequireEqual(t, lastOK, true, "selection endpoints = %#v, %#v",
		firstLine, lastLine,
	)
	testutil.RequireEqual(t, firstLine.Text, "before",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)
	testutil.RequireEqual(t, lastLine.Text, "added one",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)

	got := preserved.cursor.row - preserved.viewport.top
	want := cursor.row - viewport.top
	if got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	cursor := mustFirst(t, old)
	screen := &View{
		view:      old,
		cursor:    cursor,
		selection: ptr(beginSelection(cursor)),
		viewport:  old.newViewport(80, 10),
	}

	screen.replaceView(newUnifiedView(patch.Patch{}, true))
	preserved := screen
	testutil.RequireEqual(t, !preserved.view.valid(preserved.cursor), true,
		"state = %#v", preserved,
	)
	testutil.RequireEqual(t, preserved.selection, nil, "state = %#v", preserved)
}

func TestPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := newUnifiedView(p, true)
	cursor := mustFirst(t, old)
	viewport := old.scrollHorizontal(old.newViewport(40, 10), 30)
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true, "%s",
		"fixture does not scroll horizontally",
	)

	short := testPatch()
	preserved := &View{view: old, cursor: cursor, viewport: viewport}
	preserved.replaceView(newUnifiedView(short, true))
	testutil.RequireEqual(t, preserved.viewport.LeftColumn, 0,
		"horizontal offset = %d", preserved.viewport.LeftColumn,
	)
}

func TestUpdatePreservesCursorAfterLineDeletion(t *testing.T) {
	original, target, cases := cursorDeletionFixtures()
	for _, split := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(
				fmt.Sprintf("%s/split=%v", tc.name, split),
				func(t *testing.T) {
					v := New(original, Options{SideBySide: split})
					v.Resize(120, 4)
					v.BeginSearch()
					v.InsertSearch("target")
					v.AcceptSearch()
					_, line, ok := v.Current()
					testutil.RequireEqual(t, ok, true,
						"initial cursor = %#v, ok=%v", line, ok,
					)
					testutil.RequireEqual(t, line, target,
						"initial cursor = %#v, ok=%v", line, ok,
					)

					v.Update(patchWithHunks(tc.hunks))
					_, line, ok = v.Current()
					testutil.RequireEqual(t, ok, true,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					testutil.RequireEqual(t, line, tc.want,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					assertCursorVisible(t, v)
				},
			)
		}
	}
}

func ptr[T any](value T) *T { return &value }

type cursorDeletionCase struct {
	name  string
	hunks []patch.Hunk
	want  patch.Line
}

func cursorDeletionFixtures() (
	original patch.Patch,
	target patch.Line,
	cases []cursorDeletionCase,
) {
	early := patch.Hunk{Header: "@@ -1,0 +1 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "early change", NewNumber: 1},
	}}
	before := patch.Line{
		Kind:      patch.Context,
		Text:      "before",
		OldNumber: 50,
		NewNumber: 50,
	}
	added := patch.Line{
		Kind:      patch.Addition,
		Text:      "removed addition",
		NewNumber: 51,
	}
	after := patch.Line{
		Kind:      patch.Context,
		Text:      "after",
		OldNumber: 51,
		NewNumber: 52,
	}
	target = patch.Line{Kind: patch.Addition, Text: "target", NewNumber: 53}
	late := patch.Hunk{Header: "@@ -100,0 +100 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "late change", NewNumber: 100},
	}}
	original = patchWithHunks([]patch.Hunk{early, {
		Header: "@@ -50,2 +50,4 @@", Lines: []patch.Line{
			before,
			added,
			after,
			target,
		},
	}, late})
	shiftedAfter, shiftedTarget := after, target
	shiftedAfter.NewNumber--
	shiftedTarget.NewNumber--
	cases = []cursorDeletionCase{
		{"line before cursor deleted", []patch.Hunk{early, {
			Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{
				before,
				shiftedAfter,
				shiftedTarget,
			},
		}, late}, shiftedTarget},
		{"cursor line deleted", []patch.Hunk{early, {
			Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{
				before,
				added,
				after,
			},
		}, late}, after},
		{"cursor hunk removed", []patch.Hunk{early, late}, late.Lines[0]},
	}
	return original, target, cases
}

func assertCursorVisible(t *testing.T, v *View) {
	t.Helper()
	top := v.viewport.top
	bottom := top + v.view.contentHeight(v.viewport)
	testutil.RequireEqual(t, v.cursor.row >= top, true,
		"cursor row %d is above viewport %d:%d", v.cursor.row, top, bottom,
	)
	testutil.RequireEqual(t, v.cursor.row < bottom, true,
		"cursor row %d is below viewport %d:%d", v.cursor.row, top, bottom,
	)
}
