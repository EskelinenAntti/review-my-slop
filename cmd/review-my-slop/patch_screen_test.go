package main

import (
	"strings"
	"testing"
)

func TestDiffscreenSelectedDefaultsToCurrentAndMovementExtendsRange(
	t *testing.T,
) {
	v := newDiffscreenTestView(
		diffscreenTwoFileSelectionPatch(), diffscreenOptions{Dark: true},
	)
	file, lines, ok := v.Selected()
	requireEqual(t, ok, true, "default selection=%#v %#v %v", file, lines, ok)
	requireEqual(t, file.NewPath, "first.go",
		"default selection=%#v %#v %v", file, lines, ok,
	)
	requireEqual(t, len(lines), 1, "default selection=%#v %#v %v",
		file, lines, ok,
	)
	requireEqual(t, lines[0].Text, "before",
		"default selection=%#v %#v %v", file, lines, ok,
	)

	v.ToggleSelection()
	v.Move(diffscreenNextLine)
	_, lines, ok = v.Selected()
	requireEqual(t, ok, true, "extended selection=%#v", lines)
	requireEqual(t, len(lines), 2, "extended selection=%#v", lines)

	before := v.cursor
	for range 20 {
		v.Move(diffscreenNextLine)
	}
	requireEqual(t, v.cursor.row >= before.row, true, "%s",
		"selection moved backward",
	)

	file, _, _ = v.Selected()
	requireEqual(t, file.NewPath, "first.go", "%s", "selection crossed file")

	v.ClearSelection()
	_, lines, ok = v.Selected()
	requireEqual(t, ok, true, "cleared selection=%#v", lines)
	requireEqual(t, len(lines), 1, "cleared selection=%#v", lines)
}

func TestDiffscreenUpdateAndConfigurePreserveState(t *testing.T) {
	p := diffscreenTwoFileSelectionPatch()
	v := newDiffscreenTestView(p, diffscreenOptions{Dark: true})
	v.Resize(120, 10)
	v.Move(diffscreenNextLine)
	v.ToggleSelection()
	v.Move(diffscreenNextLine)
	_, want, _ := v.Current()
	p.Files[0].Metadata = []string{"new metadata"}
	v.Update(p)
	_, got, ok := v.Current()
	requireEqual(t, ok, true, "updated current=%#v, want %#v", got, want)
	requireEqual(t, got, want, "updated current=%#v, want %#v", got, want)

	_, lines, ok := v.Selected()
	requireEqual(t, ok, true, "updated selection=%#v", lines)
	requireEqual(t, len(lines), 2, "updated selection=%#v", lines)

	v.Configure(diffscreenOptions{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	requireEqual(t, ok, true, "configured current=%#v, want %#v", got, want)
	requireEqual(t, got, want, "configured current=%#v, want %#v", got, want)
	requireEqual(t, strings.Contains(v.Render(nil), " │ "), true,
		"%s", "split layout not rendered",
	)

	before := v.Render(nil)
	v.Update(p)
	requireEqual(t, v.Render(nil), before, "%s",
		"unchanged refresh changed presentation",
	)
}

func TestDiffscreenNarrowLayoutFallbackPreservesPreferenceAndPosition(
	t *testing.T,
) {
	p := diffscreenLongContextPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	v := newDiffscreenTestView(p, diffscreenOptions{
		SideBySide: true,
		Dark:       true,
	})
	v.Resize(120, 10)
	for range 10 {
		v.Move(diffscreenNextLine)
	}
	v.Align(diffscreenCenter)
	offset := v.cursor.row - v.viewport.top
	_, want, _ := v.Current()
	v.Resize(80, 10)
	requireEqual(t, !strings.Contains(v.Render(nil), " │ "), true,
		"%s", "narrow layout remained split",
	)

	v.Resize(120, 10)
	_, got, _ := v.Current()
	requireEqual(t, got, want, "%s", "resize lost meaningful position")
	requireEqual(t, v.cursor.row-v.viewport.top, offset, "%s",
		"resize lost meaningful position",
	)
	requireEqual(t, strings.Contains(v.Render(nil), " │ "), true,
		"%s", "resize lost split preference",
	)
}

func TestDiffscreenPageMovementDoesNotPartiallyChangeRejectedSelection(
	t *testing.T,
) {
	v := newDiffscreenTestView(patchPatch{Root: "/repo", Files: []patchFile{{
		DisplayPath: "first.go",
		OldPath:     "first.go",
		NewPath:     "first.go",
		Hunks: []patchHunk{
			{
				Header: "@@ -1,3 +1,3 @@",
				Lines: []patchLine{
					{
						Kind:      patchContext,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{
						Kind:      patchDeletion,
						Text:      "removed one",
						OldNumber: 2,
					},
					{
						Kind:      patchDeletion,
						Text:      "removed two",
						OldNumber: 3,
					},
					{
						Kind:      patchAddition,
						Text:      "added one",
						NewNumber: 2,
					},
					{
						Kind:      patchContext,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				},
			},
		},
	}, {
		DisplayPath: "second.go",
		OldPath:     "second.go",
		NewPath:     "second.go",
		Hunks: []patchHunk{
			{
				Header: "@@ -1 +1 @@",
				Lines: []patchLine{
					{
						Kind:      patchAddition,
						Text:      "other",
						NewNumber: 1,
					},
				},
			},
		},
	}}}, diffscreenOptions{Dark: true})
	v.Resize(80, 30)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(diffscreenNextPage)
	requireEqual(t, v.cursor, beforeCursor, "%s",
		"rejected selection partially changed state",
	)
	requireEqual(t, v.viewport, beforeViewport, "%s",
		"rejected selection partially changed state",
	)
}

func TestDiffscreenSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenTwoFileSelectionPatch(),
		diffscreenOptions{SideBySide: true, Dark: true},
	)
	v.Resize(120, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	requireEqual(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "deletion not found",
	)
	requireEqual(t, v.cursor.pane, diffscreenLeft, "%s",
		"search did not activate deletion pane",
	)

	first := v.cursor
	v.AcceptSearch()
	requireEqual(t, v.Find(diffscreenForward), true, "%s",
		"next match not found",
	)
	requireEqual(t, v.cursor != first, true, "%s", "next match not found")
	requireEqual(t, v.Find(diffscreenBackward), true, "%s",
		"previous match not found",
	)
	requireEqual(t, v.cursor, first, "%s", "previous match not found")

	v.BeginSearch()
	v.InsertSearch("missing")
	requireEqual(t, strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "missing query matched",
	)

	v.CancelSearch()
	requireEqual(t, v.cursor, first, "%s", "cancel did not restore focus")

	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	requireEqual(t, v.cursor, first, "%s", "empty query did not restore focus")

	v.CancelSearch()
	v.Move(diffscreenFirstLine)
	requireEqual(t, v.cursor, origin, "%s", "first line changed")
}

func TestDiffscreenSearchOriginSurvivesPatchAndLayoutChanges(t *testing.T) {
	p := diffscreenTwoFileSelectionPatch()
	v := newDiffscreenTestView(p, diffscreenOptions{Dark: true})
	v.Move(diffscreenNextLine)
	_, origin, _ := v.Current()
	v.BeginSearch()
	v.InsertSearch("added one")
	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(diffscreenOptions{SideBySide: true, Dark: true})
	v.CancelSearch()
	_, got, ok := v.Current()
	requireEqual(t, ok, true, "restored source=%#v, want %#v", got, origin)
	requireEqual(t, got, origin, "restored source=%#v, want %#v", got, origin)
}

func TestDiffscreenEmptyAndMetadataOnlyPatches(t *testing.T) {
	for _, p := range []patchPatch{
		{},
		{
			Files: []patchFile{
				{
					DisplayPath: "binary",
					Metadata:    []string{"Binary files differ"},
				},
			},
		},
	} {
		v := newDiffscreenTestView(p, diffscreenOptions{})
		v.Resize(10, 1)
		lastMotion := diffscreenOtherPane
		for motion := diffscreenPreviousLine; motion <= lastMotion; motion++ {
			v.Move(motion)
		}
		v.Align(diffscreenCenter)
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
		requireEqual(t, !v.Find(diffscreenForward), true, "%s",
			"empty view found a match",
		)

		_ = v.Render(nil)
	}
}

func TestDiffscreenHorizontalScrollEdgesAreIdempotent(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenLongContextPatch(),
		diffscreenOptions{},
	)
	v.Resize(30, 10)
	distance := int(^uint(0) >> 1)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	requireEqual(t, before != 0, true, "%s",
		"fixture has no horizontal overflow",
	)

	v.ScrollHorizontal(distance)
	requireEqual(t, v.viewport.LeftColumn, before, "%s",
		"scroll end overflowed",
	)

	v.ScrollHorizontal(-distance)
	requireEqual(t, v.viewport.LeftColumn, 0, "%s", "scroll start not clamped")
}

func TestDiffscreenVerticalScrollKeepsFocusInViewport(t *testing.T) {
	p := diffscreenLongContextPatch()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	for _, split := range []bool{false, true} {
		v := newDiffscreenTestView(p, diffscreenOptions{SideBySide: split})
		v.Resize(120, 10)
		v.ToggleSelection()
		for range 4 {
			v.Move(diffscreenNextLine)
		}
		cursor, selection := v.cursor, *v.selection
		before := v.Render(nil)
		v.ScrollVertical(3)
		requireEqual(t, v.viewport.top, 3, "%s", "scroll did not move content")
		requireEqual(t, v.Render(nil) != before, true, "%s",
			"scroll did not move content",
		)
		requireEqual(t, v.cursor, cursor, "%s",
			"scroll moved focus while it was still visible",
		)
		requireEqual(t, *v.selection, selection, "%s",
			"scroll moved focus while it was still visible",
		)

		v.ScrollVertical(6)
		requireEqual(t, v.cursor.row, v.viewport.top, "%s",
			"scroll did not clamp focus and extend selection",
		)
		requireEqual(t, v.selection.Last, v.cursor, "%s",
			"scroll did not clamp focus and extend selection",
		)
		requireEqual(t, v.selection.First, selection.First, "%s",
			"scroll did not clamp focus and extend selection",
		)

		before = v.Render(nil)
		v.Update(p)
		requireEqual(t, v.Render(nil), before, "%s",
			"refresh reset scrolled content",
		)

		v.Resize(120, 11)
		requireEqual(t, v.viewport.top, 9, "%s",
			"resize reset scrolled content",
		)

		v.Move(diffscreenNextLine)
		requireEqual(
			t,
			diffscreenCursorVisible(v.view, v.viewport, v.cursor),
			true,
			"%s",
			"keyboard navigation did not reveal focus",
		)

		v.ScrollVertical(-8)
		requireEqual(
			t,
			v.cursor.row,
			v.viewport.top+diffscreenTestViewportHeight(v.view, v.viewport)-1,
			"%s",
			"scroll up did not clamp focus to bottom of viewport",
		)
	}
}

func diffscreenTestViewportHeight(
	view *diffscreenDiffView, viewport diffscreenDiffViewport,
) int {
	height := viewport.Height
	if height > 1 && viewport.top >= 0 && viewport.top < len(view.rows) &&
		view.rows[viewport.top].kind != diffscreenFileRow {
		height--
	}
	return max(1, height)
}

func TestDiffscreenVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
	v := newDiffscreenTestView(
		diffscreenTwoFileSelectionPatch(),
		diffscreenOptions{},
	)
	v.Resize(80, 6)
	v.ToggleSelection()
	cursor, viewport, selection := v.cursor, v.viewport, *v.selection
	v.ScrollVertical(100)
	requireEqual(t, v.cursor, cursor, "%s",
		"scroll partially changed selection across hunks",
	)
	requireEqual(t, v.viewport, viewport, "%s",
		"scroll partially changed selection across hunks",
	)
	requireEqual(t, *v.selection, selection, "%s",
		"scroll partially changed selection across hunks",
	)
}

func TestDiffscreenVerticalScrollClampsAtContentEdges(t *testing.T) {
	for _, p := range []patchPatch{
		diffscreenLongContextPatch(),
		diffscreenTwoFileSelectionPatch(),
		{},
	} {
		for _, height := range []int{1, 10, 100} {
			v := newDiffscreenTestView(p, diffscreenOptions{})
			v.Resize(80, height)
			distance := int(^uint(0) >> 1)
			v.ScrollVertical(distance)
			end := v.viewport.top
			requireEqual(t,
				len(v.view.rows) <= v.viewport.Height || end != 0, true, "%s",
				"overflowing content did not scroll",
			)
			requireEqual(
				t,
				len(v.view.rows) <= 0 ||
					end+func(
						v *diffscreenDiffView, viewport diffscreenDiffViewport,
					) int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 &&
							viewport.top < len(
								v.rows,
							) &&
							v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}(
						v.view,
						v.viewport,
					) >= len(v.view.rows),
				true,
				"%s",
				"scroll did not reach final row",
			)

			v.ScrollVertical(distance)
			requireEqual(t, v.viewport.top, end, "%s",
				"scroll past end changed viewport",
			)

			v.ScrollVertical(-distance)
			v.ScrollVertical(-distance)
			requireEqual(t, v.viewport.top, 0, "%s", "scroll start not clamped")
		}
	}
}

func TestDiffscreenSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := diffscreenTwoFileSelectionPatch()
	v := newDiffscreenTestView(p, diffscreenOptions{Dark: true})
	_, origin, _ := v.Current()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	requireEqual(t, strings.Contains(v.Render(nil), "/removed界"),
		true, "%s", "query chunks were not appended",
	)

	v.BackspaceSearch()
	_, found, ok := v.Current()
	requireEqual(t, ok, true, "%s",
		"Unicode backspace did not restore a matching query",
	)
	requireEqual(t, strings.Contains(found.Text, "removed"), true,
		"%s", "Unicode backspace did not restore a matching query",
	)
	requireEqual(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "Unicode backspace did not restore a matching query",
	)

	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(diffscreenOptions{SideBySide: true, Dark: true})
	requireEqual(t, strings.Contains(v.Render(nil), "/removed"), true,
		"%s", "reconfiguration lost the active query",
	)

	v.CancelSearch()
	_, restored, _ := v.Current()
	requireEqual(t, restored, origin, "%s",
		"cancel did not restore source focus",
	)

	v.BeginSearch()
	v.BackspaceSearch()
	requireEqual(t, !strings.Contains(v.Render(nil), "/removed"), true,
		"%s", "new search retained the previous query",
	)
}
