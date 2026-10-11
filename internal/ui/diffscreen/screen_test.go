package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	file, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && file.NewPath == "first.go" && len(lines) == one &&
			lines[zero].Text == "before",
		true,
		"default selection=%#v %#v %v",
		file,
		lines,
		ok,
	)
	v.ToggleSelection()
	v.Move(NextLine)
	_, lines, ok = v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 2, true,
		"extended selection=%#v",
		lines,
	)
	before := v.cursor
	for range minimumWidth {
		v.Move(NextLine)
	}
	requireEqual(
		t,
		v.cursor.row >= before.row,
		true,
		fixtureV,
		"selection moved backward",
	)
	file, _, _ = v.Selected()
	requireEqual(
		t,
		file.NewPath, "first.go",
		fixtureV,
		"selection crossed file",
	)
	v.ClearSelection()
	_, lines, ok = v.Selected()
	requireEqual(
		t,
		ok && len(lines) == one, true,
		"cleared selection=%#v",
		lines,
	)
}

func TestUpdateAndConfigurePreserveState(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	v.Resize(wideTestWidth, ten)
	v.Move(NextLine)
	v.ToggleSelection()
	v.Move(NextLine)
	_, want, _ := v.Current()
	p.Files[zero].Metadata = []string{"new metadata"}
	v.Update(p)
	_, got, ok := v.Current()
	requireEqual(
		t,
		ok && got == want, true,
		"updated current=%#v, want %#v",
		got,
		want,
	)
	_, lines, ok := v.Selected()
	requireEqual(
		t,
		ok && len(lines) == 2, true,
		"updated selection=%#v",
		lines,
	)
	v.Configure(Options{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	requireEqual(
		t,
		ok && got == want, true,
		"configured current=%#v, want %#v",
		got,
		want,
	)
	requireEqual(
		t,
		strings.Contains(v.Render(nil), paneDivider),
		true,
		fixtureV,
		"split layout not rendered",
	)
	before := v.Render(nil)
	v.Update(p)
	requireEqual(
		t,
		v.Render(nil), before,
		fixtureV,
		"unchanged refresh changed presentation",
	)
}

func TestNarrowLayoutFallbackPreservesPreferenceAndPosition(t *testing.T) {
	p := longPatch()
	p.Files[zero].OldPath, p.Files[zero].NewPath = fixtureLongGo, fixtureLongGo
	v := New(p, Options{SideBySide: true, Dark: true})
	v.Resize(wideTestWidth, ten)
	for range ten {
		v.Move(NextLine)
	}
	v.Align(Center)
	offset := v.cursor.row - v.viewport.top
	_, want, _ := v.Current()
	v.Resize(defaultWidth, ten)
	requireEqual(
		t,
		!strings.Contains(v.Render(nil), paneDivider),
		true,
		fixtureV,
		"narrow layout remained split",
	)
	v.Resize(wideTestWidth, ten)
	_, got, _ := v.Current()
	requireEqual(
		t,
		got == want && v.cursor.row-v.viewport.top == offset,
		true,
		fixtureV,
		"resize lost meaningful position",
	)
	requireEqual(
		t,
		strings.Contains(v.Render(nil), paneDivider),
		true,
		fixtureV,
		"resize lost split preference",
	)
}

func TestPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	v.Resize(defaultWidth, defaultHeight)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(NextPage)
	requireEqual(
		t,
		v.cursor == beforeCursor && v.viewport == beforeViewport,
		true,
		fixtureV,
		"rejected selection partially changed state",
	)
}

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(wideTestWidth, ten)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	requireEqual(
		t,
		!strings.Contains(v.Render(nil), fixtureNoMatches),
		true,
		fixtureV,
		"deletion not found",
	)
	requireEqual(
		t,
		v.cursor.pane, left,
		fixtureV,
		"search did not activate deletion pane",
	)
	first := v.cursor
	v.AcceptSearch()
	foundForward := v.Find(Forward)
	requireEqual(
		t,
		foundForward && v.cursor != first,
		true,
		fixtureV,
		"next match not found",
	)
	foundBackward := v.Find(Backward)
	requireEqual(
		t,
		foundBackward && v.cursor == first,
		true,
		fixtureV,
		"previous match not found",
	)
	v.BeginSearch()
	v.InsertSearch("missing")
	requireEqual(
		t,
		strings.Contains(v.Render(nil), fixtureNoMatches),
		true,
		fixtureV,
		"missing query matched",
	)
	v.CancelSearch()
	requireEqual(t, v.cursor, first, fixtureV, "cancel did not restore focus")
	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	requireEqual(
		t,
		v.cursor, first,
		fixtureV,
		"empty query did not restore focus",
	)
	v.CancelSearch()
	v.Move(FirstLine)
	requireEqual(t, v.cursor, origin, fixtureV, "first line changed")
}

func TestSearchOriginSurvivesPatchAndLayoutChanges(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	v.Move(NextLine)
	_, origin, _ := v.Current()
	v.BeginSearch()
	v.InsertSearch("added one")
	p.Files[zero].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(wideTestWidth, ten)
	v.Configure(Options{SideBySide: true, Dark: true})
	v.CancelSearch()
	_, got, ok := v.Current()
	requireEqual(
		t,
		ok && got == origin, true,
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
		v := New(p, Options{})
		v.Resize(ten, one)
		for motion := PreviousLine; motion <= OtherPane; motion++ {
			v.Move(motion)
		}
		v.Align(Center)
		v.ToggleSelection()
		v.ScrollHorizontal(hundred)
		v.BeginSearch()
		v.InsertSearch("anything")
		v.CancelSearch()
		{
			_, _, ok := v.Current()
			requireEqual(
				t,
				!ok,
				true,
				fixtureV,
				"empty view returned focused code",
			)
		}
		{
			_, _, ok := v.Selected()
			requireEqual(
				t,
				!ok,
				true,
				fixtureV,
				"empty view returned selected code",
			)
		}
		requireEqual(
			t,
			!v.Find(Forward),
			true,
			fixtureV,
			"empty view found a match",
		)
		_ = v.Render(nil)
	}
}

func TestHorizontalScrollEdgesAreIdempotent(t *testing.T) {
	v := New(longPatch(), Options{})
	v.Resize(defaultHeight, ten)
	distance := int(^uint(zero) >> one)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	requireEqual(
		t,
		before != zero,
		true,
		fixtureV,
		"fixture has no horizontal overflow",
	)
	v.ScrollHorizontal(distance)
	requireEqual(
		t,
		v.viewport.LeftColumn, before,
		fixtureV,
		"scroll end overflowed",
	)
	v.ScrollHorizontal(-distance)
	requireEqual(
		t,
		v.viewport.LeftColumn, zero,
		fixtureV,
		"scroll start not clamped",
	)
}

func TestVerticalScrollKeepsFocusInViewport(t *testing.T) {
	p := longPatch()
	p.Files[zero].OldPath, p.Files[zero].NewPath = fixtureLongGo, fixtureLongGo
	for _, split := range []bool{false, true} {
		v := New(p, Options{SideBySide: split})
		v.Resize(wideTestWidth, ten)
		v.ToggleSelection()
		for range four {
			v.Move(NextLine)
		}
		cursor, selection := v.cursor, *v.selection
		before := v.Render(nil)
		v.ScrollVertical(three)
		requireEqual(
			t,
			v.viewport.top == three && v.Render(nil) != before,
			true,
			fixtureV,
			"scroll did not move content",
		)
		requireEqual(
			t,
			v.cursor == cursor && *v.selection == selection,
			true,
			fixtureV,
			"scroll moved focus while it was still visible",
		)
		v.ScrollVertical(six)
		requireEqual(
			t,
			v.cursor.row == v.viewport.top && v.selection.Last == v.cursor &&
				v.selection.First == selection.First,
			true,
			fixtureV,
			"scroll did not clamp focus and extend selection",
		)
		before = v.Render(nil)
		v.Update(p)
		requireEqual(
			t,
			v.Render(nil), before,
			fixtureV,
			"refresh reset scrolled content",
		)
		v.Resize(wideTestWidth, fixtureNumber11)
		requireEqual(
			t,
			v.viewport.top, nine,
			fixtureV,
			"resize reset scrolled content",
		)
		v.Move(NextLine)
		requireEqual(
			t,
			v.view.cursorVisible(v.viewport, v.cursor),
			true,
			fixtureV,
			"keyboard navigation did not reveal focus",
		)
		v.ScrollVertical(-8)
		requireEqual(
			t,
			v.cursor.row, v.viewport.top+v.view.contentHeight(v.viewport)-one,
			fixtureV,
			"scroll up did not clamp focus to bottom of viewport",
		)
	}
}

func TestVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(defaultWidth, six)
	v.ToggleSelection()
	cursor, viewport, selection := v.cursor, v.viewport, *v.selection
	v.ScrollVertical(hundred)
	requireEqual(
		t,
		v.cursor == cursor && v.viewport == viewport &&
			*v.selection == selection,
		true,
		fixtureV,
		"scroll partially changed selection across hunks",
	)
}

func TestVerticalScrollClampsAtContentEdges(t *testing.T) {
	for _, p := range []patch.Patch{longPatch(), testPatch(), {}} {
		for _, height := range []int{one, ten, hundred} {
			v := New(p, Options{})
			v.Resize(defaultWidth, height)
			distance := int(^uint(zero) >> one)
			v.ScrollVertical(distance)
			end := v.viewport.top
			requireEqual(
				t,
				len(v.view.rows) <= v.viewport.Height || end != zero,
				true,
				fixtureV,
				"overflowing content did not scroll",
			)
			requireEqual(
				t,
				len(v.view.rows) <= zero ||
					end+v.view.contentHeight(v.viewport) >= len(v.view.rows),
				true,
				fixtureV,
				"scroll did not reach final row",
			)
			v.ScrollVertical(distance)
			requireEqual(
				t,
				v.viewport.top, end,
				fixtureV,
				"scroll past end changed viewport",
			)
			v.ScrollVertical(-distance)
			v.ScrollVertical(-distance)
			requireEqual(
				t,
				v.viewport.top, zero,
				fixtureV,
				"scroll start not clamped",
			)
		}
	}
}

func TestSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := testPatch()
	v := New(p, Options{Dark: true})
	_, origin, _ := v.Current()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	requireEqual(
		t,
		strings.Contains(v.Render(nil), "/removed界"),
		true,
		fixtureV,
		"query chunks were not appended",
	)
	v.BackspaceSearch()
	_, found, ok := v.Current()
	requireEqual(
		t,
		ok && strings.Contains(found.Text, "removed") &&
			!strings.Contains(v.Render(nil), fixtureNoMatches),
		true,
		fixtureV,
		"Unicode backspace did not restore a matching query",
	)
	p.Files[zero].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(wideTestWidth, ten)
	v.Configure(Options{SideBySide: true, Dark: true})
	requireEqual(
		t,
		strings.Contains(v.Render(nil), "/removed"),
		true,
		fixtureV,
		"reconfiguration lost the active query",
	)
	v.CancelSearch()
	_, restored, _ := v.Current()
	requireEqual(
		t,
		restored, origin,
		fixtureV,
		"cancel did not restore source focus",
	)
	v.BeginSearch()
	v.BackspaceSearch()
	requireEqual(
		t,
		!strings.Contains(v.Render(nil), "/removed"),
		true,
		fixtureV,
		"new search retained the previous query",
	)
}
