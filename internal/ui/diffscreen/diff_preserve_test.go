package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	first := mustFirst(t, old)
	cursor, ok := old.search("added one", first, forward)
	requireEqual(t, ok, true, fixtureV, "cursor not found")
	selection := old.beginSelection(first)
	selection, ok = old.extendSelection(selection, cursor)
	requireEqual(t, ok, true, fixtureV, "selection not created")
	viewport := old.newViewport(hundred, four)
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

	requireEqual(
		t,
		next.valid(preserved.cursor),
		true,
		fixtureV,
		"cursor was not preserved",
	)
	line, ok := next.line(preserved.cursor)
	requireEqual(
		t,
		ok && line.Text == "added one", true,
		"cursor line = %#v, ok=%v",
		line,
		ok,
	)
	requireEqual(
		t,
		preserved.selection != nil, true,
		"selection = %#v",
		preserved.selection,
	)
	firstLine, firstOK := next.line(preserved.selection.First)
	lastLine, lastOK := next.line(preserved.selection.Last)
	requireEqual(
		t,
		firstOK && lastOK && firstLine.Text == "before" &&
			lastLine.Text == "added one",
		true,
		"selection endpoints = %#v, %#v",
		firstLine,
		lastLine,
	)
	{
		got := preserved.cursor.row - preserved.viewport.top
		want := cursor.row - viewport.top
		requireEqual(t, got, want, "screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := newUnifiedView(testPatch(), true)
	cursor := mustFirst(t, old)
	screen := &View{
		view:      old,
		cursor:    cursor,
		selection: ptr(old.beginSelection(cursor)),
		viewport:  old.newViewport(defaultWidth, ten),
	}

	screen.replaceView(newUnifiedView(patch.Patch{}, true))
	preserved := screen

	requireEqual(
		t,
		preserved.selection == nil && !preserved.view.valid(preserved.cursor),
		true,
		"state = %#v",
		preserved,
	)
}

func TestPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := longPatch()
	p.Files[zero].OldPath, p.Files[zero].NewPath = "long.go", "long.go"
	old := newUnifiedView(p, true)
	cursor := mustFirst(t, old)
	viewport := old.scrollHorizontal(
		old.newViewport(forty, ten),
		defaultHeight,
	)
	requireEqual(
		t,
		viewport.LeftColumn != zero,
		true,
		fixtureV,
		"fixture does not scroll horizontally",
	)
	short := testPatch()
	preserved := &View{view: old, cursor: cursor, viewport: viewport}
	preserved.replaceView(newUnifiedView(short, true))
	requireEqual(
		t,
		preserved.viewport.LeftColumn, zero,
		"horizontal offset = %d",
		preserved.viewport.LeftColumn,
	)
}

func TestUpdatePreservesCursorAfterLineDeletion(t *testing.T) {
	original, target, cases := cursorDeletionCases()
	for _, split := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(

				fmt.Sprintf("%s/split=%v", tc.name, split),

				func(
					t *testing.T,
				) {
					assertDeletedCursor(
						t,
						original,
						target,
						tc,
						Options{
							SideBySide: split,
						},
					)
				},
			)
		}
	}
}

func ptr[T any](value T) *T { return &value }

type deletionCase struct {
	name  string
	hunks []patch.Hunk
	want  patch.Line
}

func cursorDeletionCases() (patch.Patch, patch.Line, []deletionCase) {
	source := deletionSource()
	original, target := source.original, source.target
	before, added, after := source.before, source.added, source.after
	early, late := source.early, source.late
	shiftedAfter, shiftedTarget := after, target
	shiftedAfter.NewNumber--
	shiftedTarget.NewNumber--

	return original, target, []deletionCase{
		{
			"line before cursor deleted",
			[]patch.Hunk{
				early,
				{
					Header: "@@ -50,2 +50,3 @@",
					Lines: []patch.Line{
						before,
						shiftedAfter,
						shiftedTarget,
					},
				},
				late,
			},
			shiftedTarget,
		},

		{
			"cursor line deleted",
			[]patch.Hunk{
				early,
				{
					Header: "@@ -50,2 +50,3 @@",
					Lines:  []patch.Line{before, added, after},
				},
				late,
			},
			after,
		},

		{
			"cursor hunk removed",
			[]patch.Hunk{early, late},
			late.Lines[zero],
		},
	}
}

func hunkPatch(hunks []patch.Hunk) patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{OldPath: "file.go", NewPath: "file.go", Hunks: hunks},
		},
	}
}

func assertDeletedCursor(
	t *testing.T,
	original patch.Patch,
	target patch.Line,
	tc deletionCase,
	options Options,
) {
	v := New(original, options)
	v.Resize(wideTestWidth, four)
	v.BeginSearch()
	v.InsertSearch("target")
	v.AcceptSearch()
	_, line, ok := v.Current()
	requireEqual(
		t,
		ok && line == target, true,
		"initial cursor = %#v, ok=%v",
		line,
		ok,
	)
	v.Update(hunkPatch(tc.hunks))
	_, line, ok = v.Current()
	requireEqual(
		t,
		ok && line == tc.want, true,
		"restored cursor = %#v, ok=%v, want %#v",
		line,
		ok,
		tc.want,
	)
	requireEqual(
		t,
		v.cursor.row >= v.viewport.top &&
			v.cursor.row < v.viewport.top+
				v.view.contentHeight(v.viewport),
		true,
		fixtureV,
		"restored cursor is outside the viewport",
	)
}

type deletionFixture struct {
	original                     patch.Patch
	before, added, after, target patch.Line
	early, late                  patch.Hunk
}

func deletionSource() deletionFixture {
	const (
		beforeSourceLine   = 50
		addedSourceLine    = beforeSourceLine + 1
		afterOldSourceLine = beforeSourceLine + 1
		afterNewSourceLine = addedSourceLine + 1
		targetSourceLine   = afterNewSourceLine + 1
	)
	early := patch.Hunk{Header: "@@ -1,0 +1 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "early change", NewNumber: one},
	}}
	before := patch.Line{
		Kind:      patch.Context,
		Text:      "before",
		OldNumber: beforeSourceLine,
		NewNumber: beforeSourceLine,
	}
	added := patch.Line{
		Kind:      patch.Addition,
		Text:      "removed addition",
		NewNumber: addedSourceLine,
	}
	after := patch.Line{
		Kind:      patch.Context,
		Text:      "after",
		OldNumber: afterOldSourceLine,
		NewNumber: afterNewSourceLine,
	}
	target := patch.Line{
		Kind:      patch.Addition,
		Text:      "target",
		NewNumber: targetSourceLine,
	}
	late := patch.Hunk{Header: "@@ -100,0 +100 @@", Lines: []patch.Line{
		{Kind: patch.Addition, Text: "late change", NewNumber: hundred},
	}}

	original := hunkPatch(
		[]patch.Hunk{
			early,
			{
				Header: "@@ -50,2 +50,4 @@",
				Lines:  []patch.Line{before, added, after, target},
			},
			late,
		},
	)
	return deletionFixture{
		original: original, before: before, added: added, after: after,
		target: target, early: early, late: late,
	}
}
