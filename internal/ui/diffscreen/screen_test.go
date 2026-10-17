package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
	v := New(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, Options{Dark: true})
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
	p := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}
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
	p := (func() patch.Patch {
		lines := make([]patch.Line, 20)
		for index := range lines {
			lines[index] = patch.Line{
				Kind:      patch.Context,
				Text:      strings.Repeat("long", 20),
				OldNumber: patch.LineNumber(index + 1),
				NewNumber: patch.LineNumber(index + 1),
			}
		}
		return patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "long.go",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		}
	})()
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

func TestPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	v := New(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, Options{Dark: true})
	v.Resize(80, 30)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(NextPage)
	testutil.RequireEqual(t, v.cursor, beforeCursor, "%s",
		"rejected selection partially changed state",
	)
	testutil.RequireEqual(t, v.viewport, beforeViewport, "%s",
		"rejected selection partially changed state",
	)
}

func TestSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
	v := New(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, Options{SideBySide: true, Dark: true})
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
	p := (func() patch.Patch {
		return patch.Patch{Root: "/repo", Files: []patch.File{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
					{
						Kind:      patch.Context,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
					{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
					{Kind: patch.Addition, Text: "added one", NewNumber: 2},
					{
						Kind:      patch.Context,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})()
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
	v := New((func() patch.Patch {
		lines := make([]patch.Line, 20)
		for index := range lines {
			lines[index] = patch.Line{
				Kind:      patch.Context,
				Text:      strings.Repeat("long", 20),
				OldNumber: patch.LineNumber(index + 1),
				NewNumber: patch.LineNumber(index + 1),
			}
		}
		return patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "long.go",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		}
	})(), Options{})
	v.Resize(30, 10)
	distance := int(^uint(0) >> 1)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	testutil.RequireEqual(t, before != 0, true, "%s",
		"fixture has no horizontal overflow",
	)

	v.ScrollHorizontal(distance)
	testutil.RequireEqual(t, v.viewport.LeftColumn, before, "%s",
		"scroll end overflowed",
	)

	v.ScrollHorizontal(-distance)
	testutil.RequireEqual(t, v.viewport.LeftColumn, 0, "%s",
		"scroll start not clamped",
	)
}

func TestVerticalScrollKeepsFocusInViewport(t *testing.T) {
	p := (func() patch.Patch {
		lines := make([]patch.Line, 20)
		for index := range lines {
			lines[index] = patch.Line{
				Kind:      patch.Context,
				Text:      strings.Repeat("long", 20),
				OldNumber: patch.LineNumber(index + 1),
				NewNumber: patch.LineNumber(index + 1),
			}
		}
		return patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "long.go",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		}
	})()
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
		testutil.RequireEqual(t, func(v *diffView, viewport diffViewport, cursor diffCursor) bool {
			return cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0 && cursor.row >= viewport.top && cursor.row < viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}()
		}(v.view, v.viewport, v.cursor),
			true, "%s", "keyboard navigation did not reveal focus",
		)

		v.ScrollVertical(-8)
		testutil.RequireEqual(t, v.cursor.row,
			v.viewport.top+func(v *diffView, viewport diffViewport) int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}(v.view, v.viewport)-1, "%s",
			"scroll up did not clamp focus to bottom of viewport",
		)
	}
}

func TestVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
	v := New((func() patch.Patch {
		return patch.Patch{Root: "/repo", Files: []patch.File{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
					{
						Kind:      patch.Context,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
					{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
					{Kind: patch.Addition, Text: "added one", NewNumber: 2},
					{
						Kind:      patch.Context,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})(), Options{})
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
	for _, p := range []patch.Patch{(func() patch.Patch {
		lines := make([]patch.Line, 20)
		for index := range lines {
			lines[index] = patch.Line{
				Kind:      patch.Context,
				Text:      strings.Repeat("long", 20),
				OldNumber: patch.LineNumber(index + 1),
				NewNumber: patch.LineNumber(index + 1),
			}
		}
		return patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "long.go",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		}
	})(), (func() patch.Patch {
		return patch.Patch{Root: "/repo", Files: []patch.File{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
					{
						Kind:      patch.Context,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
					{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
					{Kind: patch.Addition, Text: "added one", NewNumber: 2},
					{
						Kind:      patch.Context,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})(), {}} {
		for _, height := range []int{1, 10, 100} {
			v := New(p, Options{})
			v.Resize(80, height)
			distance := int(^uint(0) >> 1)
			v.ScrollVertical(distance)
			end := v.viewport.top
			testutil.RequireEqual(t,
				len(v.view.rows) <= v.viewport.Height || end != 0, true, "%s",
				"overflowing content did not scroll",
			)
			testutil.RequireEqual(
				t,
				len(v.view.rows) <= 0 ||
					end+func(v *diffView, viewport diffViewport) int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}(v.view, v.viewport) >= len(v.view.rows),
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
	p := (func() patch.Patch {
		return patch.Patch{Root: "/repo", Files: []patch.File{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{
					{
						Kind:      patch.Context,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patch.Deletion, Text: "removed one", OldNumber: 2},
					{Kind: patch.Deletion, Text: "removed two", OldNumber: 3},
					{Kind: patch.Addition, Text: "added one", NewNumber: 2},
					{
						Kind:      patch.Context,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})()
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
