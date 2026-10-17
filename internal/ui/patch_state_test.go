package ui

import (
	"fmt"
	"testing"

	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestDiffscreenPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	patch := diffscreenTwoFileSelectionPatch()
	old := diffscreenTestViewWithTheme(patch, true)
	first, ok := firstCursor(old)
	requireEqual(t, ok, true, "%s", "no first cursor")
	cursor, foundCursor := findSearchCursor(old, "added one", first)
	requireEqual(t, foundCursor, true, "%s", "cursor not found")

	selection, ok := selectionForDrag(old, first, cursor)
	requireEqual(t, ok, true, "%s", "selection not created")
	screen := &Diff{
		view: old, cursor: cursor, selection: &selection,
		viewport: initialViewport(old, 100, 4),
		options:  Options{SideBySide: true}, width: 100,
	}
	screen.Align(Center)
	viewport := screen.viewport

	next := diffscreenTestSplitViewWithTheme(
		diffscreenTwoFileSelectionPatch(), true,
	)
	screen.Update(next.patch)
	preserved := screen
	requireEqual(t, cursorLine(next, preserved.cursor) >= 0, true,
		"%s", "cursor was not preserved")

	_, line, ok := patchLineAtCursor(next, preserved.cursor)
	requireEqual(t, ok, true, "cursor line = %#v, ok=%v", line, ok)
	requireEqual(t, line.Text, "added one", "cursor line = %#v, ok=%v",
		line, ok,
	)
	requireEqual(t, preserved.selection != nil, true,
		"selection = %#v", preserved.selection,
	)
	firstLine, firstOK := diffscreenSelectionFirstLine(
		*preserved.selection, next,
	)
	lastLine, lastOK := diffscreenSelectionLastLine(
		*preserved.selection, next,
	)
	requireEqual(t, firstOK, true,
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)
	requireEqual(t, lastOK, true,
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)
	requireEqual(t, firstLine.Text, "before",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)
	requireEqual(t, lastLine.Text, "added one",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)

	got := preserved.cursor.row - preserved.viewport.top
	want := cursor.row - viewport.top
	if got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestDiffscreenPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := diffscreenTestViewWithTheme(diffscreenTwoFileSelectionPatch(), true)
	cursor, ok := firstCursor(old)
	requireEqual(t, ok, true, "%s", "no first cursor")
	viewport := initialViewport(old, 80, 10)
	screen := &Diff{
		view: old, cursor: cursor,
		selection: &diffSelection{First: cursor, Last: cursor},
		viewport:  viewport,
	}

	screen.Update(patches.Patch{})
	preserved := screen
	requireEqual(
		t, cursorLine(preserved.view, preserved.cursor) < 0, true,
		"state = %#v", preserved,
	)
	requireEqual(t, preserved.selection, nil, "state = %#v", preserved)
}

func TestDiffscreenPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := diffscreenLongContextPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := diffscreenTestViewWithTheme(p, true)
	cursor, ok := firstCursor(old)
	requireEqual(t, ok, true, "%s", "no first cursor")
	viewport := initialViewport(old, 40, 10)
	viewport.LeftColumn = 30
	viewport = clampViewport(old, viewport)
	requireEqual(t, viewport.LeftColumn != 0, true, "%s",
		"fixture does not scroll horizontally",
	)

	short := diffscreenTwoFileSelectionPatch()
	preserved := &Diff{
		view: old, cursor: cursor, viewport: viewport,
		options: Options{}, width: 40,
	}
	preserved.Update(short)
	requireEqual(t, preserved.viewport.LeftColumn, 0,
		"horizontal offset = %d", preserved.viewport.LeftColumn,
	)
}

func diffscreenCursorDeletionFixtures() (
	original patches.Patch, target patches.Line,
	cases []diffscreenCursorDeletionCase,
) {
	early := patches.Hunk{Header: "@@ -1,0 +1 @@", Lines: []patches.Line{{
		Kind:      patches.Addition,
		Text:      "early change",
		NewNumber: 1,
	}}}
	before := patches.Line{
		Kind:      patches.Context,
		Text:      "before",
		OldNumber: 50,
		NewNumber: 50,
	}
	added := patches.Line{
		Kind:      patches.Addition,
		Text:      "removed addition",
		NewNumber: 51,
	}
	after := patches.Line{
		Kind:      patches.Context,
		Text:      "after",
		OldNumber: 51,
		NewNumber: 52,
	}
	target = patches.Line{Kind: patches.Addition, Text: "target", NewNumber: 53}
	late := patches.Hunk{Header: "@@ -100,0 +100 @@", Lines: []patches.Line{{
		Kind:      patches.Addition,
		Text:      "late change",
		NewNumber: 100,
	}}}
	original = patches.Patch{Files: []patches.File{{
		OldPath: "file.go",
		NewPath: "file.go",
		Hunks: []patches.Hunk{
			early,
			{
				Header: "@@ -50,2 +50,4 @@",
				Lines:  []patches.Line{before, added, after, target},
			},
			late,
		},
	}}}
	shiftedAfter, shiftedTarget := after, target
	shiftedAfter.NewNumber--
	shiftedTarget.NewNumber--
	cases = []diffscreenCursorDeletionCase{{
		"line before cursor deleted",
		[]patches.Hunk{
			early,
			{
				Header: "@@ -50,2 +50,3 @@",
				Lines:  []patches.Line{before, shiftedAfter, shiftedTarget},
			},
			late,
		},
		shiftedTarget,
	}, {"cursor line deleted", []patches.Hunk{early, {
		Header: "@@ -50,2 +50,3 @@",
		Lines:  []patches.Line{before, added, after},
	}, late}, after}, {"cursor hunk removed", []patches.Hunk{
		early,
		late,
	}, late.Lines[0]}}
	return original, target, cases
}

func TestDiffscreenUpdatePreservesCursorAfterLineDeletion(t *testing.T) {
	original, target, cases := diffscreenCursorDeletionFixtures()
	for _, split := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(
				fmt.Sprintf("%s/split=%v", tc.name, split),
				func(t *testing.T) {
					v := newDiffscreenTestView(original, Options{
						SideBySide: split,
					})
					v.Resize(120, 4)
					v.BeginSearch()
					v.InsertSearch("target")
					v.AcceptSearch()
					_, line, ok := v.Current()
					requireEqual(t, ok, true,
						"initial cursor = %#v, ok=%v", line, ok,
					)
					requireEqual(t, line, target,
						"initial cursor = %#v, ok=%v", line, ok,
					)

					v.Update(patches.Patch{Files: []patches.File{{
						OldPath: "file.go",
						NewPath: "file.go",
						Hunks:   tc.hunks,
					}}})
					_, line, ok = v.Current()
					requireEqual(t, ok, true,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					requireEqual(t, line, tc.want,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					assertDiffscreenRestoredCursorVisible(t, v)
				},
			)
		}
	}
}

func assertDiffscreenRestoredCursorVisible(t *testing.T, view *Diff) {
	t.Helper()
	top := view.viewport.top
	bottom := top + viewportHeight(view.view, view.viewport)
	requireEqual(t, view.cursor.row >= top, true,
		"cursor row %d is above viewport %d:%d", view.cursor.row, top, bottom,
	)
	requireEqual(t, view.cursor.row < bottom, true,
		"cursor row %d is below viewport %d:%d", view.cursor.row, top, bottom,
	)
}

type diffscreenCursorDeletionCase struct {
	name  string
	hunks []patches.Hunk
	want  patches.Line
}
