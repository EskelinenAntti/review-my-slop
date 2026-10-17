package main

import (
	"fmt"
	"testing"
)

func TestDiffscreenPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	patch := diffscreenTwoFileSelectionPatch()
	old := diffscreenTestViewWithTheme(patch, true)
	first, ok := diffscreenFirstCursor(old)
	requireEqual(t, ok, true, "%s", "no first cursor")
	cursor, foundCursor := diffscreenFindSearchCursor(old, "added one", first)
	requireEqual(t, foundCursor, true, "%s", "cursor not found")

	selection, ok := diffscreenSelectionForDrag(old, first, cursor)
	requireEqual(t, ok, true, "%s", "selection not created")
	screen := &diffscreenView{
		view: old, cursor: cursor, selection: &selection,
		viewport: diffscreenInitialViewport(old, 100, 4),
		options:  diffscreenOptions{SideBySide: true}, width: 100,
	}
	screen.Align(diffscreenCenter)
	viewport := screen.viewport

	next := diffscreenTestSplitViewWithTheme(
		diffscreenTwoFileSelectionPatch(), true,
	)
	screen.Update(next.patch)
	preserved := screen
	requireEqual(t, diffscreenCursorLine(next, preserved.cursor) >= 0, true,
		"%s", "cursor was not preserved")

	_, line, ok := diffscreenPatchLineAtCursor(next, preserved.cursor)
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
	cursor, ok := diffscreenFirstCursor(old)
	requireEqual(t, ok, true, "%s", "no first cursor")
	viewport := diffscreenInitialViewport(old, 80, 10)
	screen := &diffscreenView{
		view: old, cursor: cursor,
		selection: &diffscreenDiffSelection{First: cursor, Last: cursor},
		viewport:  viewport,
	}

	screen.Update(patchPatch{})
	preserved := screen
	requireEqual(
		t, diffscreenCursorLine(preserved.view, preserved.cursor) < 0, true,
		"state = %#v", preserved,
	)
	requireEqual(t, preserved.selection, nil, "state = %#v", preserved)
}

func TestDiffscreenPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := diffscreenLongContextPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := diffscreenTestViewWithTheme(p, true)
	cursor, ok := diffscreenFirstCursor(old)
	requireEqual(t, ok, true, "%s", "no first cursor")
	viewport := diffscreenInitialViewport(old, 40, 10)
	viewport.LeftColumn = 30
	viewport = diffscreenClampViewport(old, viewport)
	requireEqual(t, viewport.LeftColumn != 0, true, "%s",
		"fixture does not scroll horizontally",
	)

	short := diffscreenTwoFileSelectionPatch()
	preserved := &diffscreenView{
		view: old, cursor: cursor, viewport: viewport,
		options: diffscreenOptions{}, width: 40,
	}
	preserved.Update(short)
	requireEqual(t, preserved.viewport.LeftColumn, 0,
		"horizontal offset = %d", preserved.viewport.LeftColumn,
	)
}

func diffscreenCursorDeletionFixtures() (
	original patchPatch, target patchLine, cases []diffscreenCursorDeletionCase,
) {
	early := patchHunk{Header: "@@ -1,0 +1 @@", Lines: []patchLine{{
		Kind:      patchAddition,
		Text:      "early change",
		NewNumber: 1,
	}}}
	before := patchLine{
		Kind:      patchContext,
		Text:      "before",
		OldNumber: 50,
		NewNumber: 50,
	}
	added := patchLine{
		Kind:      patchAddition,
		Text:      "removed addition",
		NewNumber: 51,
	}
	after := patchLine{
		Kind:      patchContext,
		Text:      "after",
		OldNumber: 51,
		NewNumber: 52,
	}
	target = patchLine{Kind: patchAddition, Text: "target", NewNumber: 53}
	late := patchHunk{Header: "@@ -100,0 +100 @@", Lines: []patchLine{{
		Kind:      patchAddition,
		Text:      "late change",
		NewNumber: 100,
	}}}
	original = patchPatch{Files: []patchFile{{
		OldPath: "file.go",
		NewPath: "file.go",
		Hunks: []patchHunk{
			early,
			{
				Header: "@@ -50,2 +50,4 @@",
				Lines:  []patchLine{before, added, after, target},
			},
			late,
		},
	}}}
	shiftedAfter, shiftedTarget := after, target
	shiftedAfter.NewNumber--
	shiftedTarget.NewNumber--
	cases = []diffscreenCursorDeletionCase{{
		"line before cursor deleted",
		[]patchHunk{
			early,
			{
				Header: "@@ -50,2 +50,3 @@",
				Lines:  []patchLine{before, shiftedAfter, shiftedTarget},
			},
			late,
		},
		shiftedTarget,
	}, {"cursor line deleted", []patchHunk{early, {
		Header: "@@ -50,2 +50,3 @@",
		Lines:  []patchLine{before, added, after},
	}, late}, after}, {"cursor hunk removed", []patchHunk{
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
					v := newDiffscreenTestView(original, diffscreenOptions{
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

					v.Update(patchPatch{Files: []patchFile{{
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

func assertDiffscreenRestoredCursorVisible(t *testing.T, view *diffscreenView) {
	t.Helper()
	top := view.viewport.top
	bottom := top + diffscreenViewportHeight(view.view, view.viewport)
	requireEqual(t, view.cursor.row >= top, true,
		"cursor row %d is above viewport %d:%d", view.cursor.row, top, bottom,
	)
	requireEqual(t, view.cursor.row < bottom, true,
		"cursor row %d is below viewport %d:%d", view.cursor.row, top, bottom,
	)
}

type diffscreenCursorDeletionCase struct {
	name  string
	hunks []patchHunk
	want  patchLine
}
