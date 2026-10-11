package diffscreen

import (
	"math"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

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
	v.Resize(defaultHeight, 10)
	distance := math.MaxInt
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	requireEqual(
		t,
		before != 0,
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
		v.viewport.LeftColumn, 0,
		fixtureV,
		"scroll start not clamped",
	)
}

func TestVerticalScrollKeepsFocusInViewport(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = fixtureLongGo, fixtureLongGo
	for _, split := range []bool{false, true} {
		v := New(p, Options{SideBySide: split})
		v.Resize(wideTestWidth, 10)
		v.ToggleSelection()
		for range 4 {
			v.Move(NextLine)
		}
		cursor, selection := v.cursor, *v.selection
		before := v.Render(nil)
		v.ScrollVertical(3)
		requireEqual(
			t,
			v.viewport.top == 3 && v.Render(nil) != before,
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
		v.ScrollVertical(6)
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
		v.Resize(wideTestWidth, stickyHeaderTestHeight)
		requireEqual(
			t,
			v.viewport.top, 9,
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
			v.cursor.row, v.viewport.top+v.view.contentHeight(v.viewport)-1,
			fixtureV,
			"scroll up did not clamp focus to bottom of viewport",
		)
	}
}

func TestVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(defaultWidth, 6)
	v.ToggleSelection()
	cursor, viewport, selection := v.cursor, v.viewport, *v.selection
	v.ScrollVertical(100)
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
		for _, height := range []int{1, 10, 100} {
			v := New(p, Options{})
			v.Resize(defaultWidth, height)
			distance := math.MaxInt
			v.ScrollVertical(distance)
			end := v.viewport.top
			requireEqual(
				t,
				len(v.view.rows) <= v.viewport.Height || end != 0,
				true,
				fixtureV,
				"overflowing content did not scroll",
			)
			requireEqual(
				t,
				len(v.view.rows) <= 0 ||
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
				v.viewport.top, 0,
				fixtureV,
				"scroll start not clamped",
			)
		}
	}
}
