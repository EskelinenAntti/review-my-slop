package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestHorizontalScrollEdgesAreIdempotent(t *testing.T) {
	v := New(longPatch(), Options{})
	v.Resize(30, 10)
	distance := int(^uint(0) >> 1)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	testutil.RequireEqual(t, before != 0, true,
		"fixture has no horizontal overflow",
	)

	v.ScrollHorizontal(distance)
	testutil.RequireEqual(t, v.viewport.LeftColumn, before,
		"scroll end overflowed",
	)

	v.ScrollHorizontal(-distance)
	testutil.RequireEqual(t, v.viewport.LeftColumn, 0,
		"scroll start not clamped",
	)
}

func TestVerticalScrollKeepsFocusInViewport(t *testing.T) {
	p := longPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	for _, split := range []bool{false, true} {
		v := New(p, Options{SideBySide: split})
		v.Resize(120, 10)
		v.ToggleSelection()
		for range 4 {
			v.Move(NextLine)
		}
		cursor, selection := v.cursor, *v.selection
		before := v.Render(nil)
		v.ScrollVertical(3)
		testutil.RequireEqual(t, v.viewport.top, 3,
			"scroll did not move content",
		)
		testutil.RequireEqual(t, v.Render(nil) != before, true,
			"scroll did not move content",
		)
		testutil.RequireEqual(t, v.cursor, cursor,
			"scroll moved focus while it was still visible",
		)
		testutil.RequireEqual(t, *v.selection, selection,
			"scroll moved focus while it was still visible",
		)

		v.ScrollVertical(6)
		testutil.RequireEqual(t, v.cursor.row, v.viewport.top,
			"scroll did not clamp focus and extend selection",
		)
		testutil.RequireEqual(t, v.selection.Last, v.cursor,
			"scroll did not clamp focus and extend selection",
		)
		testutil.RequireEqual(t, v.selection.First, selection.First,
			"scroll did not clamp focus and extend selection",
		)

		before = v.Render(nil)
		v.Update(p)
		testutil.RequireEqual(t, v.Render(nil), before,
			"refresh reset scrolled content",
		)

		v.Resize(120, 11)
		testutil.RequireEqual(t, v.viewport.top, 9,
			"resize reset scrolled content",
		)

		v.Move(NextLine)
		testutil.RequireEqual(t, v.view.cursorVisible(v.viewport, v.cursor),
			true, "keyboard navigation did not reveal focus",
		)

		v.ScrollVertical(-8)
		testutil.RequireEqual(t, v.cursor.row,
			v.viewport.top+v.view.contentHeight(v.viewport)-1,
			"scroll up did not clamp focus to bottom of viewport",
		)
	}
}

func TestVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
	v := New(testPatch(), Options{})
	v.Resize(80, 6)
	v.ToggleSelection()
	cursor, viewport, selection := v.cursor, v.viewport, *v.selection
	v.ScrollVertical(100)
	testutil.RequireEqual(t, v.cursor, cursor,
		"scroll partially changed selection across hunks",
	)
	testutil.RequireEqual(t, v.viewport, viewport,
		"scroll partially changed selection across hunks",
	)
	testutil.RequireEqual(t, *v.selection, selection,
		"scroll partially changed selection across hunks",
	)
}

func TestVerticalScrollClampsAtContentEdges(t *testing.T) {
	for _, p := range []patch.Patch{longPatch(), testPatch(), {}} {
		for _, height := range []int{1, 10, 100} {
			v := New(p, Options{})
			v.Resize(80, height)
			distance := int(^uint(0) >> 1)
			v.ScrollVertical(distance)
			end := v.viewport.top
			testutil.RequireEqual(t,
				len(v.view.rows) <= v.viewport.Height || end != 0, true,
				"overflowing content did not scroll",
			)
			testutil.RequireEqual(t, len(v.view.rows) <= 0 ||
				end+v.view.contentHeight(v.viewport) >= len(v.view.rows),
				true, "scroll did not reach final row",
			)

			v.ScrollVertical(distance)
			testutil.RequireEqual(t, v.viewport.top, end,
				"scroll past end changed viewport",
			)

			v.ScrollVertical(-distance)
			v.ScrollVertical(-distance)
			testutil.RequireEqual(t, v.viewport.top, 0,
				"scroll start not clamped",
			)
		}
	}
}
