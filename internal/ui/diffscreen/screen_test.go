package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	file, lines, ok := v.Selected()
	testutil.Requiref(t, ok && file.NewPath == "first.go" && len(lines) == 1 && lines[0].Text == "before", "default selection=%#v %#v %v", file, lines, ok)
	v.ToggleSelection()
	v.Move(NextLine)
	_, lines, ok = v.Selected()
	testutil.Requiref(t, ok && len(lines) == 2, "extended selection=%#v", lines)
	before := v.cursor
	for range 20 {
		v.Move(NextLine)
	}
	testutil.Require(t, v.cursor.row >= before.row, "selection moved backward")
	file, _, _ = v.Selected()
	testutil.Require(t, file.NewPath == "first.go", "selection crossed file")
	v.ClearSelection()
	_, lines, ok = v.Selected()
	testutil.Requiref(t, ok && len(lines) == 1, "cleared selection=%#v", lines)
}

func TestUpdateAndConfigurePreserveState(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	v.Resize(120, 10)
	v.Move(NextLine)
	v.ToggleSelection()
	v.Move(NextLine)
	_, want, _ := v.Current()
	p.Files[0].Metadata = []string{"new metadata"}
	v.Update(p)
	_, got, ok := v.Current()
	testutil.Requiref(t, ok && got == want, "updated current=%#v, want %#v", got, want)
	_, lines, ok := v.Selected()
	testutil.Requiref(t, ok && len(lines) == 2, "updated selection=%#v", lines)
	v.Configure(Options{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	testutil.Requiref(t, ok && got == want, "configured current=%#v, want %#v", got, want)
	testutil.Require(t, strings.Contains(v.Render(nil), " │ "), "split layout not rendered")
	before := v.Render(nil)
	v.Update(p)
	testutil.Require(t, v.Render(nil) == before, "unchanged refresh changed presentation")
}

func TestNarrowLayoutFallbackPreservesPreferenceAndPosition(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	v := New(p, Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	for range 10 {
		v.Move(NextLine)
	}
	v.Align(Center)
	offset := v.cursor.row - v.viewport.top
	_, want, _ := v.Current()
	v.Resize(80, 10)
	testutil.Require(t, !strings.Contains(v.Render(nil), " │ "), "narrow layout remained split")
	v.Resize(120, 10)
	_, got, _ := v.Current()
	testutil.Require(t, got == want && v.cursor.row-v.viewport.top == offset, "resize lost meaningful position")
	testutil.Require(t, strings.Contains(v.Render(nil), " │ "), "resize lost split preference")
}

func TestPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	v.Resize(80, 30)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(NextPage)
	testutil.Require(t, v.cursor == beforeCursor && v.viewport == beforeViewport, "rejected selection partially changed state")
}

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	testutil.Require(t, !strings.Contains(v.Render(nil), "no matches"), "deletion not found")
	testutil.Require(t, v.cursor.pane == left, "search did not activate deletion pane")
	first := v.cursor
	v.AcceptSearch()
	testutil.Require(t, v.Find(Forward) && v.cursor != first, "next match not found")
	testutil.Require(t, v.Find(Backward) && v.cursor == first, "previous match not found")
	v.BeginSearch()
	v.InsertSearch("missing")
	testutil.Require(t, strings.Contains(v.Render(nil), "no matches"), "missing query matched")
	v.CancelSearch()
	testutil.Require(t, v.cursor == first, "cancel did not restore focus")
	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	testutil.Require(t, v.cursor == first, "empty query did not restore focus")
	v.CancelSearch()
	v.Move(FirstLine)
	testutil.Require(t, v.cursor == origin, "first line changed")
}

func TestSearchOriginSurvivesPatchAndLayoutChanges(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	v.Move(NextLine)
	_, origin, _ := v.Current()
	v.BeginSearch()
	v.InsertSearch("added one")
	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	v.CancelSearch()
	_, got, ok := v.Current()
	testutil.Requiref(t, ok && got == origin, "restored source=%#v, want %#v", got, origin)
}

func TestEmptyAndMetadataOnlyPatches(t *testing.T) {
	for _, p := range []patch.Patch{
		{},
		{Files: []patch.File{{DisplayPath: "binary", Metadata: []string{"Binary files differ"}}}},
	} {
		v := New(p, Options{})
		v.Resize(10, 1)
		for motion := PreviousLine; motion <= OtherPane; motion++ {
			v.Move(motion)
		}
		v.Align(Center)
		v.ToggleSelection()
		v.ScrollHorizontal(100)
		v.BeginSearch()
		v.InsertSearch("anything")
		v.CancelSearch()
		{
			_, _, ok := v.Current()
			testutil.Require(t, !ok, "empty view returned focused code")
		}
		{
			_, _, ok := v.Selected()
			testutil.Require(t, !ok, "empty view returned selected code")
		}
		testutil.Require(t, !v.Find(Forward), "empty view found a match")
		_ = v.Render(nil)
	}
}

func TestHorizontalScrollEdgesAreIdempotent(t *testing.T) {
	v := New(longPatch(), Options{})
	v.Resize(30, 10)
	distance := int(^uint(0) >> 1)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	testutil.Require(t, before != 0, "fixture has no horizontal overflow")
	v.ScrollHorizontal(distance)
	testutil.Require(t, v.viewport.LeftColumn == before, "scroll end overflowed")
	v.ScrollHorizontal(-distance)
	testutil.Require(t, v.viewport.LeftColumn == 0, "scroll start not clamped")
}

func TestVerticalScrollKeepsFocusInViewport(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	for _, split := range []bool{false, true} {
		assertScrollFocus(t, p, split)
	}
}

func TestVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(80, 6)
	v.ToggleSelection()
	cursor, viewport, selection := v.cursor, v.viewport, *v.selection
	v.ScrollVertical(100)
	testutil.Require(t, v.cursor == cursor && v.viewport == viewport && *v.selection == selection, "scroll partially changed selection across hunks")
}

func TestVerticalScrollClampsAtContentEdges(t *testing.T) {
	for _, p := range []patch.Patch{longPatch(), testPatch(), {}} {
		for _, height := range []int{1, 10, 100} {
			assertScrollEdges(t, p, height)
		}
	}
}

func TestSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	_, origin, _ := v.Current()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	testutil.Require(t, strings.Contains(v.Render(nil), "/removed界"), "query chunks were not appended")
	v.BackspaceSearch()
	_, found, ok := v.Current()
	testutil.Require(t, ok && strings.Contains(found.Text, "removed") && !strings.Contains(v.Render(nil), "no matches"), "Unicode backspace did not restore a matching query")
	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	testutil.Require(t, strings.Contains(v.Render(nil), "/removed"), "reconfiguration lost the active query")
	v.CancelSearch()
	_, restored, _ := v.Current()
	testutil.Require(t, restored == origin, "cancel did not restore source focus")
	v.BeginSearch()
	v.BackspaceSearch()
	testutil.Require(t, !strings.Contains(v.Render(nil), "/removed"), "new search retained the previous query")
}

func assertScrollFocus(t *testing.T, p patch.Patch, split bool) {
	t.Helper()
	v := New(p, Options{SideBySide: split})
	v.Resize(120, 10)
	v.ToggleSelection()
	for range 4 {
		v.Move(NextLine)
	}
	cursor, selection := v.cursor, *v.selection
	before := v.Render(nil)
	v.ScrollVertical(3)
	testutil.Require(t, v.viewport.top == 3 && v.Render(nil) != before, "scroll did not move content")
	testutil.Require(t, v.cursor == cursor && *v.selection == selection, "scroll moved focus while it was still visible")
	v.ScrollVertical(6)
	testutil.Require(t, v.cursor.row == v.viewport.top && v.selection.Last == v.cursor && v.selection.First == selection.First, "scroll did not clamp focus and extend selection")
	before = v.Render(nil)
	v.Update(p)
	testutil.Require(t, v.Render(nil) == before, "refresh reset scrolled content")
	v.Resize(120, 11)
	testutil.Require(t, v.viewport.top == 9, "resize reset scrolled content")
	v.Move(NextLine)
	testutil.Require(t, v.view.cursorVisible(v.viewport, v.cursor), "keyboard navigation did not reveal focus")
	v.ScrollVertical(-8)
	testutil.Require(t, v.cursor.row == v.viewport.top+v.view.contentHeight(v.viewport)-1, "scroll up did not clamp focus to bottom of viewport")
}

func assertScrollEdges(t *testing.T, p patch.Patch, height int) {
	t.Helper()
	v := New(p, Options{})
	v.Resize(80, height)
	distance := int(^uint(0) >> 1)
	v.ScrollVertical(distance)
	end := v.viewport.top
	testutil.Require(t, len(v.view.rows) <= v.viewport.Height || end != 0, "overflowing content did not scroll")
	testutil.Require(t, len(v.view.rows) <= 0 || end+v.view.contentHeight(v.viewport) >= len(v.view.rows), "scroll did not reach final row")
	v.ScrollVertical(distance)
	testutil.Require(t, v.viewport.top == end, "scroll past end changed viewport")
	v.ScrollVertical(-distance)
	v.ScrollVertical(-distance)
	testutil.Require(t, v.viewport.top == 0, "scroll start not clamped")
}
