package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestFocusedSelectionAndMovementExtendRange(t *testing.T) {
	v := NewChangesView(testPatch(), Options{Dark: true})
	file, lines, ok := v.SelectedFileAndLines()
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"default selection=%#v %#v %v",
		file,
		lines,
		ok,
	)
	testutil.RequireValuesEqual(t, file.NewPath, "first.go",
		"default selection=%#v %#v %v", file, lines, ok,
	)
	testutil.RequireValuesEqual(
		t,
		len(lines),
		1,
		"default selection=%#v %#v %v",
		file,
		lines,
		ok,
	)
	testutil.RequireValuesEqual(t, lines[0].Text, "before",
		"default selection=%#v %#v %v", file, lines, ok,
	)

	v.ToggleSelection()
	v.MoveFocusWithinChanges(NextLine)
	_, lines, ok = v.SelectedFileAndLines()
	testutil.RequireValuesEqual(t, ok, true, "extended selection=%#v", lines)
	testutil.RequireValuesEqual(
		t,
		len(lines),
		2,
		"extended selection=%#v",
		lines,
	)

	before := v.cursor
	for range 20 {
		v.MoveFocusWithinChanges(NextLine)
	}
	testutil.RequireValuesEqual(t, v.cursor.row >= before.row, true, "%s",
		"selection moved backward",
	)

	file, _, _ = v.SelectedFileAndLines()
	testutil.RequireValuesEqual(t, file.NewPath, "first.go", "%s",
		"selection crossed file",
	)

	v.ClearSelection()
	_, lines, ok = v.SelectedFileAndLines()
	testutil.RequireValuesEqual(t, ok, true, "cleared selection=%#v", lines)
	testutil.RequireValuesEqual(
		t,
		len(lines),
		1,
		"cleared selection=%#v",
		lines,
	)
}

func TestReplaceChangesAndSetOptionsPreserveState(t *testing.T) {
	p := testPatch()
	v := NewChangesView(p, Options{Dark: true})
	v.Resize(120, 10)
	v.MoveFocusWithinChanges(NextLine)
	v.ToggleSelection()
	v.MoveFocusWithinChanges(NextLine)
	_, want, _ := v.FocusedFileAndLine()
	p.Files[0].Metadata = []string{"new metadata"}
	v.ReplaceChanges(p)
	_, got, ok := v.FocusedFileAndLine()
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"updated current=%#v, want %#v",
		got,
		want,
	)
	testutil.RequireValuesEqual(
		t,
		got,
		want,
		"updated current=%#v, want %#v",
		got,
		want,
	)

	_, lines, ok := v.SelectedFileAndLines()
	testutil.RequireValuesEqual(t, ok, true, "updated selection=%#v", lines)
	testutil.RequireValuesEqual(
		t,
		len(lines),
		2,
		"updated selection=%#v",
		lines,
	)

	v.SetOptions(Options{SideBySide: true, Dark: false})
	_, got, ok = v.FocusedFileAndLine()
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"configured current=%#v, want %#v",
		got,
		want,
	)
	testutil.RequireValuesEqual(
		t,
		got,
		want,
		"configured current=%#v, want %#v",
		got,
		want,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(v.FormatReviewScreen(nil), " │ "),
		true,
		"%s",
		"split layout not rendered",
	)

	before := v.FormatReviewScreen(nil)
	v.ReplaceChanges(p)
	testutil.RequireValuesEqual(t, v.FormatReviewScreen(nil), before, "%s",
		"unchanged refresh changed presentation",
	)
}

func TestNarrowLayoutFallbackPreservesPreferenceAndPosition(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	v := NewChangesView(p, Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	for range 10 {
		v.MoveFocusWithinChanges(NextLine)
	}
	v.PositionFocusedLine(Center)
	offset := v.cursor.row - v.viewport.top
	_, want, _ := v.FocusedFileAndLine()
	v.Resize(80, 10)
	testutil.RequireValuesEqual(
		t,
		!strings.Contains(v.FormatReviewScreen(nil), " │ "),
		true,
		"%s",
		"narrow layout remained split",
	)

	v.Resize(120, 10)
	_, got, _ := v.FocusedFileAndLine()
	testutil.RequireValuesEqual(
		t,
		got,
		want,
		"%s",
		"resize lost meaningful position",
	)
	testutil.RequireValuesEqual(t, v.cursor.row-v.viewport.top, offset, "%s",
		"resize lost meaningful position",
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(v.FormatReviewScreen(nil), " │ "),
		true,
		"%s",
		"resize lost split preference",
	)
}

func TestPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	v := NewChangesView(testPatch(), Options{Dark: true})
	v.Resize(80, 30)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.MoveFocusWithinChanges(NextPage)
	testutil.RequireValuesEqual(t, v.cursor, beforeCursor, "%s",
		"rejected selection partially changed state",
	)
	testutil.RequireValuesEqual(t, v.viewport, beforeViewport, "%s",
		"rejected selection partially changed state",
	)
}

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := NewChangesView(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	testutil.RequireValuesEqual(
		t,
		!strings.Contains(v.FormatReviewScreen(nil), "no matches"),
		true,
		"%s",
		"deletion not found",
	)
	testutil.RequireValuesEqual(t, v.cursor.pane, left, "%s",
		"findTextInChanges did not activate deletion pane",
	)

	first := v.cursor
	v.AcceptSearch()
	testutil.RequireValuesEqual(t, v.FindNextMatch(Forward), true, "%s",
		"next match not found",
	)
	testutil.RequireValuesEqual(t, v.cursor != first, true, "%s",
		"next match not found",
	)
	testutil.RequireValuesEqual(t, v.FindNextMatch(Backward), true, "%s",
		"previous match not found",
	)
	testutil.RequireValuesEqual(
		t,
		v.cursor,
		first,
		"%s",
		"previous match not found",
	)

	v.BeginSearch()
	v.InsertSearch("missing")
	testutil.RequireValuesEqual(
		t,
		strings.Contains(v.FormatReviewScreen(nil), "no matches"),
		true,
		"%s",
		"missing query matched",
	)

	v.CancelSearch()
	testutil.RequireValuesEqual(t, v.cursor, first, "%s",
		"cancel did not restore focus",
	)

	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	testutil.RequireValuesEqual(t, v.cursor, first, "%s",
		"empty query did not restore focus",
	)

	v.CancelSearch()
	v.MoveFocusWithinChanges(FirstLine)
	testutil.RequireValuesEqual(t, v.cursor, origin, "%s", "first line changed")
}

func TestSearchOriginSurvivesPatchAndLayoutChanges(t *testing.T) {
	p := testPatch()
	v := NewChangesView(p, Options{Dark: true})
	v.MoveFocusWithinChanges(NextLine)
	_, origin, _ := v.FocusedFileAndLine()
	v.BeginSearch()
	v.InsertSearch("added one")
	p.Files[0].Metadata = []string{"inserted"}
	v.ReplaceChanges(p)
	v.Resize(120, 10)
	v.SetOptions(Options{SideBySide: true, Dark: true})
	v.CancelSearch()
	_, got, ok := v.FocusedFileAndLine()
	testutil.RequireValuesEqual(
		t,
		ok,
		true,
		"restored source=%#v, want %#v",
		got,
		origin,
	)
	testutil.RequireValuesEqual(
		t,
		got,
		origin,
		"restored source=%#v, want %#v",
		got,
		origin,
	)
}

func TestEmptyAndMetadataOnlyPatches(t *testing.T) {
	for _, p := range []patch.Patch{
		{},
		{
			Files: []patch.File{
				{
					DisplayPath: "binary",
					Metadata:    []string{"Binary files differ"},
				},
			},
		},
	} {
		v := NewChangesView(p, Options{})
		v.Resize(10, 1)
		for motion := PreviousLine; motion <= OtherPane; motion++ {
			v.MoveFocusWithinChanges(motion)
		}
		v.PositionFocusedLine(Center)
		v.ToggleSelection()
		v.ScrollHorizontal(100)
		v.BeginSearch()
		v.InsertSearch("anything")
		v.CancelSearch()
		if _, _, ok := v.FocusedFileAndLine(); ok {
			t.Fatal("empty view returned focused code")
		}
		if _, _, ok := v.SelectedFileAndLines(); ok {
			t.Fatal("empty view returned selected code")
		}
		testutil.RequireValuesEqual(t, !v.FindNextMatch(Forward), true, "%s",
			"empty view found a match",
		)

		_ = v.FormatReviewScreen(nil)
	}
}

func TestHorizontalScrollEdgesAreIdempotent(t *testing.T) {
	v := NewChangesView(longPatch(), Options{})
	v.Resize(30, 10)
	distance := int(^uint(0) >> 1)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	testutil.RequireValuesEqual(t, before != 0, true, "%s",
		"fixture has no horizontal overflow",
	)

	v.ScrollHorizontal(distance)
	testutil.RequireValuesEqual(t, v.viewport.LeftColumn, before, "%s",
		"scroll end overflowed",
	)

	v.ScrollHorizontal(-distance)
	testutil.RequireValuesEqual(t, v.viewport.LeftColumn, 0, "%s",
		"scroll start not clamped",
	)
}

func TestVerticalScrollKeepsFocusInViewport(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	for _, split := range []bool{false, true} {
		v := NewChangesView(p, Options{SideBySide: split})
		v.Resize(120, 10)
		v.ToggleSelection()
		for range 4 {
			v.MoveFocusWithinChanges(NextLine)
		}
		cursor, selection := v.cursor, *v.selection
		before := v.FormatReviewScreen(nil)
		v.ScrollVertical(3)
		testutil.RequireValuesEqual(t, v.viewport.top, 3, "%s",
			"scroll did not move content",
		)
		testutil.RequireValuesEqual(
			t,
			v.FormatReviewScreen(nil) != before,
			true,
			"%s",
			"scroll did not move content",
		)
		testutil.RequireValuesEqual(t, v.cursor, cursor, "%s",
			"scroll moved focus while it was still visible",
		)
		testutil.RequireValuesEqual(t, *v.selection, selection, "%s",
			"scroll moved focus while it was still visible",
		)

		v.ScrollVertical(6)
		testutil.RequireValuesEqual(t, v.cursor.row, v.viewport.top, "%s",
			"scroll did not clamp focus and extend selection",
		)
		testutil.RequireValuesEqual(t, v.selection.Last, v.cursor, "%s",
			"scroll did not clamp focus and extend selection",
		)
		testutil.RequireValuesEqual(t, v.selection.First, selection.First, "%s",
			"scroll did not clamp focus and extend selection",
		)

		before = v.FormatReviewScreen(nil)
		v.ReplaceChanges(p)
		testutil.RequireValuesEqual(t, v.FormatReviewScreen(nil), before, "%s",
			"refresh reset scrolled content",
		)

		v.Resize(120, 11)
		testutil.RequireValuesEqual(t, v.viewport.top, 9, "%s",
			"resize reset scrolled content",
		)

		v.MoveFocusWithinChanges(NextLine)
		testutil.RequireValuesEqual(
			t,
			v.view.isFocusedLineVisible(v.viewport, v.cursor),
			true,
			"%s",
			"keyboard navigation did not reveal focus",
		)

		v.ScrollVertical(-8)
		testutil.RequireValuesEqual(t, v.cursor.row,
			v.viewport.top+v.view.visibleRowsBelowHeader(v.viewport)-1, "%s",
			"scroll up did not clamp focus to bottom of viewport",
		)
	}
}

func TestVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
	v := NewChangesView(testPatch(), Options{})
	v.Resize(80, 6)
	v.ToggleSelection()
	cursor, viewport, selection := v.cursor, v.viewport, *v.selection
	v.ScrollVertical(100)
	testutil.RequireValuesEqual(t, v.cursor, cursor, "%s",
		"scroll partially changed selection across hunks",
	)
	testutil.RequireValuesEqual(t, v.viewport, viewport, "%s",
		"scroll partially changed selection across hunks",
	)
	testutil.RequireValuesEqual(t, *v.selection, selection, "%s",
		"scroll partially changed selection across hunks",
	)
}

func TestVerticalScrollClampsAtContentEdges(t *testing.T) {
	for _, p := range []patch.Patch{longPatch(), testPatch(), {}} {
		for _, height := range []int{1, 10, 100} {
			v := NewChangesView(p, Options{})
			v.Resize(80, height)
			distance := int(^uint(0) >> 1)
			v.ScrollVertical(distance)
			end := v.viewport.top
			testutil.RequireValuesEqual(t,
				len(v.view.rows) <= v.viewport.Height || end != 0, true, "%s",
				"overflowing content did not scroll",
			)
			testutil.RequireValuesEqual(
				t,
				len(v.view.rows) <= 0 ||
					end+v.view.visibleRowsBelowHeader(
						v.viewport,
					) >= len(
						v.view.rows,
					),
				true,
				"%s",
				"scroll did not reach final row",
			)

			v.ScrollVertical(distance)
			testutil.RequireValuesEqual(t, v.viewport.top, end, "%s",
				"scroll past end changed viewport",
			)

			v.ScrollVertical(-distance)
			v.ScrollVertical(-distance)
			testutil.RequireValuesEqual(t, v.viewport.top, 0, "%s",
				"scroll start not clamped",
			)
		}
	}
}

func TestSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := testPatch()
	v := NewChangesView(p, Options{Dark: true})
	_, origin, _ := v.FocusedFileAndLine()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	testutil.RequireValuesEqual(
		t,
		strings.Contains(v.FormatReviewScreen(nil), "/removed界"),
		true,
		"%s",
		"query chunks were not appended",
	)

	v.BackspaceSearch()
	_, found, ok := v.FocusedFileAndLine()
	testutil.RequireValuesEqual(t, ok, true, "%s",
		"Unicode backspace did not restore a matching query",
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(found.Text, "removed"),
		true,
		"%s",
		"Unicode backspace did not restore a matching query",
	)
	testutil.RequireValuesEqual(
		t,
		!strings.Contains(v.FormatReviewScreen(nil), "no matches"),
		true,
		"%s",
		"Unicode backspace did not restore a matching query",
	)

	p.Files[0].Metadata = []string{"inserted"}
	v.ReplaceChanges(p)
	v.Resize(120, 10)
	v.SetOptions(Options{SideBySide: true, Dark: true})
	testutil.RequireValuesEqual(
		t,
		strings.Contains(v.FormatReviewScreen(nil), "/removed"),
		true,
		"%s",
		"reconfiguration lost the active query",
	)

	v.CancelSearch()
	_, restored, _ := v.FocusedFileAndLine()
	testutil.RequireValuesEqual(t, restored, origin, "%s",
		"cancel did not restore source focus",
	)

	v.BeginSearch()
	v.BackspaceSearch()
	testutil.RequireValuesEqual(
		t,
		!strings.Contains(v.FormatReviewScreen(nil), "/removed"),
		true,
		"%s",
		"new search retained the previous query",
	)
}
