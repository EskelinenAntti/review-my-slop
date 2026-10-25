package diffscreen

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
	v := New(testPatch(), Options{Dark: true})
	file, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true, "default selection=%#v %#v %v", file,
		lines, ok,
	)
	testutil.RequireEqual(t, file.NewPath, "first.go",
		"default selection=%#v %#v %v", file, lines, ok,
	)
	testutil.RequireEqual(t, len(lines), 1, "default selection=%#v %#v %v",
		file, lines, ok,
	)
	testutil.RequireEqual(t, lines[0].Text, "before",
		"default selection=%#v %#v %v", file, lines, ok,
	)

	v.ToggleSelection()
	v.Move(NextLine)
	_, lines, ok = v.Selected()
	testutil.RequireEqual(t, ok, true, "extended selection=%#v", lines)
	testutil.RequireEqual(t, len(lines), 2, "extended selection=%#v", lines)

	before := v.cursor
	for range 20 {
		v.Move(NextLine)
	}
	testutil.RequireEqual(t, v.cursor.row >= before.row, true, "%s",
		"selection moved backward",
	)

	file, _, _ = v.Selected()
	testutil.RequireEqual(t, file.NewPath, "first.go", "%s",
		"selection crossed file",
	)

	v.ClearSelection()
	_, lines, ok = v.Selected()
	testutil.RequireEqual(t, ok, true, "cleared selection=%#v", lines)
	testutil.RequireEqual(t, len(lines), 1, "cleared selection=%#v", lines)
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
	testutil.RequireEqual(t, ok, true, "updated current=%#v, want %#v", got,
		want,
	)
	testutil.RequireEqual(t, got, want, "updated current=%#v, want %#v", got,
		want,
	)

	_, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true, "updated selection=%#v", lines)
	testutil.RequireEqual(t, len(lines), 2, "updated selection=%#v", lines)

	v.Configure(Options{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	testutil.RequireEqual(t, ok, true, "configured current=%#v, want %#v", got,
		want,
	)
	testutil.RequireEqual(t, got, want, "configured current=%#v, want %#v", got,
		want,
	)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), " │ "), true,
		"%s", "split layout not rendered",
	)

	before := v.Render(nil)
	v.Update(p)
	testutil.RequireEqual(t, v.Render(nil), before, "%s",
		"unchanged refresh changed presentation",
	)
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
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), " │ "), true,
		"%s", "narrow layout remained split",
	)

	v.Resize(120, 10)
	_, got, _ := v.Current()
	testutil.RequireEqual(t, got, want, "%s", "resize lost meaningful position")
	testutil.RequireEqual(t, v.cursor.row-v.viewport.top, offset, "%s",
		"resize lost meaningful position",
	)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), " │ "), true,
		"%s", "resize lost split preference",
	)
}

func TestMovementCommitsFocusViewportAndSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		move func(*View)
	}{
		{"next line", func(v *View) { v.Move(NextLine) }},
		{"next page", func(v *View) { v.Move(NextPage) }},
		{"scroll down", func(v *View) { v.ScrollVertical(8) }},
	} {
		for _, selected := range []bool{false, true} {
			name := fmt.Sprintf("%s/selected=%t", tc.name, selected)
			t.Run(name, func(t *testing.T) {
				v := New(longPatch(), Options{})
				v.Resize(80, 8)
				for range 4 {
					v.Move(NextLine)
				}
				if selected {
					v.ToggleSelection()
				}
				beforeCursor, beforeViewport := v.cursor, v.viewport

				tc.move(v)

				testutil.RequireEqual(t, v.cursor != beforeCursor, true,
					"accepted movement did not move focus")
				testutil.RequireEqual(t, v.viewport != beforeViewport, true,
					"accepted movement did not scroll")
				visible := v.view.cursorVisible(v.viewport, v.cursor)
				testutil.RequireEqual(t, visible, true,
					"accepted movement left focus outside viewport")
				if selected {
					testutil.RequireEqual(t, v.selection.First, beforeCursor,
						"accepted movement changed selection start")
					testutil.RequireEqual(t, v.selection.Last, v.cursor,
						"accepted movement did not extend selection to focus")
				} else if v.selection != nil {
					t.Fatal("movement started a selection")
				}
			})
		}
	}
}

func TestMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		backward bool
		move     func(*View)
	}{
		{"next line", false, func(v *View) { v.Move(NextLine) }},
		{"next page", false, func(v *View) { v.Move(NextPage) }},
		{"scroll down", false, func(v *View) { v.ScrollVertical(math.MaxInt) }},
		{"previous line", true, func(v *View) { v.Move(PreviousLine) }},
		{"previous page", true, func(v *View) { v.Move(PreviousPage) }},
		{"scroll up", true, func(v *View) { v.ScrollVertical(-math.MaxInt) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := longPatch()
			p.Files[0].Hunks = append(p.Files[0].Hunks, p.Files[0].Hunks[0])
			v := New(p, Options{Dark: true})
			v.Resize(80, 8)
			steps := len(p.Files[0].Hunks[0].Lines) - 1
			if tc.backward {
				steps++
			}
			for range steps {
				v.Move(NextLine)
			}
			v.ToggleSelection()
			beforeCursor, beforeViewport := v.cursor, v.viewport
			beforeSelection := *v.selection
			tc.move(v)
			testutil.RequireEqual(t, v.cursor, beforeCursor, "%s",
				"rejected selection changed cursor",
			)
			testutil.RequireEqual(t, v.viewport, beforeViewport, "%s",
				"rejected selection changed viewport",
			)
			testutil.RequireEqual(t, *v.selection, beforeSelection, "%s",
				"rejected selection changed endpoints",
			)
		})
	}
}

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := New(testPatch(), Options{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "deletion not found",
	)
	testutil.RequireEqual(t, v.cursor.pane, left, "%s",
		"search did not activate deletion pane",
	)

	first := v.cursor
	v.AcceptSearch()
	testutil.RequireEqual(t, v.Find(Forward), true, "%s",
		"next match not found",
	)
	testutil.RequireEqual(t, v.cursor != first, true, "%s",
		"next match not found",
	)
	testutil.RequireEqual(t, v.Find(Backward), true, "%s",
		"previous match not found",
	)
	testutil.RequireEqual(t, v.cursor, first, "%s", "previous match not found")

	v.BeginSearch()
	v.InsertSearch("missing")
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "missing query matched",
	)

	v.CancelSearch()
	testutil.RequireEqual(t, v.cursor, first, "%s",
		"cancel did not restore focus",
	)

	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	testutil.RequireEqual(t, v.cursor, first, "%s",
		"empty query did not restore focus",
	)

	v.CancelSearch()
	v.Move(FirstLine)
	testutil.RequireEqual(t, v.cursor, origin, "%s", "first line changed")
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
	testutil.RequireEqual(t, ok, true, "restored source=%#v, want %#v", got,
		origin,
	)
	testutil.RequireEqual(t, got, origin, "restored source=%#v, want %#v", got,
		origin,
	)
}

func TestSectionMotions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		start  Motion
		motion Motion
		file   string
		text   string
		pane   diffPane
	}{
		{"next file", FirstLine, NextFile, "second.go", "other", right},
		{"previous file", LastLine, PreviousFile, "first.go", "after", right},
		{"old pane", FirstLine, OldPane, "first.go", "before", left},
		{"new pane", FirstLine, NewPane, "first.go", "before", right},
		{"other pane", FirstLine, OtherPane, "first.go", "before", left},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := New(testPatch(), Options{SideBySide: true})
			v.Resize(120, 10)
			v.Move(tc.start)
			if tc.motion == NewPane {
				v.Move(OldPane)
			}
			cursor := v.cursor
			v.drag = &cursor
			v.search.repeatMiss = true

			v.Move(tc.motion)

			file, line, ok := v.Current()
			testutil.RequireEqual(t, ok, true, "motion lost focus")
			testutil.RequireEqual(t, file.DisplayPath, tc.file, "wrong file")
			testutil.RequireEqual(t, line.Text, tc.text, "wrong source line")
			testutil.RequireEqual(t, v.cursor.pane, tc.pane, "wrong pane")
			if v.drag != nil || v.search.repeatMiss {
				t.Fatal("motion did not clear drag and repeated-search miss")
			}
		})
	}
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
		if _, _, ok := v.Current(); ok {
			t.Fatal("empty view returned focused code")
		}
		if _, _, ok := v.Selected(); ok {
			t.Fatal("empty view returned selected code")
		}
		testutil.RequireEqual(t, !v.Find(Forward), true, "%s",
			"empty view found a match",
		)

		_ = v.Render(nil)
	}
}

func TestHorizontalScrollEdgesAreIdempotent(t *testing.T) {
	v := New(longPatch(), Options{})
	v.Resize(30, 10)
	distance := math.MaxInt
	v.ScrollHorizontal(distance)
	before := v.viewport.leftColumn
	testutil.RequireEqual(t, before != 0, true, "%s",
		"fixture has no horizontal overflow",
	)

	v.ScrollHorizontal(distance)
	testutil.RequireEqual(t, v.viewport.leftColumn, before, "%s",
		"scroll end overflowed",
	)

	v.ScrollHorizontal(-distance)
	testutil.RequireEqual(t, v.viewport.leftColumn, 0, "%s",
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
		testutil.RequireEqual(t, v.viewport.top, 3, "%s",
			"scroll did not move content",
		)
		testutil.RequireEqual(t, v.Render(nil) != before, true, "%s",
			"scroll did not move content",
		)
		testutil.RequireEqual(t, v.cursor, cursor, "%s",
			"scroll moved focus while it was still visible",
		)
		testutil.RequireEqual(t, *v.selection, selection, "%s",
			"scroll moved focus while it was still visible",
		)

		v.ScrollVertical(6)
		testutil.RequireEqual(t, v.cursor.row, v.viewport.top, "%s",
			"scroll did not clamp focus and extend selection",
		)
		testutil.RequireEqual(t, v.selection.Last, v.cursor, "%s",
			"scroll did not clamp focus and extend selection",
		)
		testutil.RequireEqual(t, v.selection.First, selection.First, "%s",
			"scroll did not clamp focus and extend selection",
		)

		before = v.Render(nil)
		v.Update(p)
		testutil.RequireEqual(t, v.Render(nil), before, "%s",
			"refresh reset scrolled content",
		)

		v.Resize(120, 11)
		testutil.RequireEqual(t, v.viewport.top, 9, "%s",
			"resize reset scrolled content",
		)

		v.Move(NextLine)
		testutil.RequireEqual(t, v.view.cursorVisible(v.viewport, v.cursor),
			true, "%s", "keyboard navigation did not reveal focus",
		)

		v.ScrollVertical(-8)
		testutil.RequireEqual(t, v.cursor.row,
			v.viewport.top+v.view.contentHeight(v.viewport)-1, "%s",
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
	testutil.RequireEqual(t, v.cursor, cursor, "%s",
		"scroll partially changed selection across hunks",
	)
	testutil.RequireEqual(t, v.viewport, viewport, "%s",
		"scroll partially changed selection across hunks",
	)
	testutil.RequireEqual(t, *v.selection, selection, "%s",
		"scroll partially changed selection across hunks",
	)
}

func TestVerticalScrollClampsAtContentEdges(t *testing.T) {
	for _, p := range []patch.Patch{longPatch(), testPatch(), {}} {
		for _, height := range []int{1, 10, 100} {
			v := New(p, Options{})
			v.Resize(80, height)
			distance := math.MaxInt
			v.ScrollVertical(distance)
			end := v.viewport.top
			testutil.RequireEqual(t,
				len(v.view.rows) <= v.viewport.height || end != 0, true, "%s",
				"overflowing content did not scroll",
			)
			testutil.RequireEqual(
				t,
				len(v.view.rows) <= 0 ||
					end+v.view.contentHeight(v.viewport) >= len(v.view.rows),
				true,
				"%s",
				"scroll did not reach final row",
			)

			v.ScrollVertical(distance)
			testutil.RequireEqual(t, v.viewport.top, end, "%s",
				"scroll past end changed viewport",
			)

			v.ScrollVertical(-distance)
			v.ScrollVertical(-distance)
			testutil.RequireEqual(t, v.viewport.top, 0, "%s",
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
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "/removed界"),
		true, "%s", "query chunks were not appended",
	)

	v.BackspaceSearch()
	_, found, ok := v.Current()
	testutil.RequireEqual(t, ok, true, "%s",
		"Unicode backspace did not restore a matching query",
	)
	testutil.RequireEqual(t, strings.Contains(found.Text, "removed"), true,
		"%s", "Unicode backspace did not restore a matching query",
	)
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "Unicode backspace did not restore a matching query",
	)

	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(Options{SideBySide: true, Dark: true})
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "/removed"), true,
		"%s", "reconfiguration lost the active query",
	)

	v.CancelSearch()
	_, restored, _ := v.Current()
	testutil.RequireEqual(t, restored, origin, "%s",
		"cancel did not restore source focus",
	)

	v.BeginSearch()
	v.BackspaceSearch()
	testutil.RequireEqual(t, !strings.Contains(v.Render(nil), "/removed"), true,
		"%s", "new search retained the previous query",
	)
}
