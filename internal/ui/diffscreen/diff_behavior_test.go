package diffscreen

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{
			{
				Header: "one",
				Lines: []patch.Line{
					{Kind: patch.Deletion, Text: "d1", OldNumber: 1},
					{Kind: patch.Deletion, Text: "d2", OldNumber: 2},
					{Kind: patch.Addition, Text: "a1", NewNumber: 1},
					{Kind: patch.Addition, Text: "a2", NewNumber: 2},
					{Kind: patch.Addition, Text: "a3", NewNumber: 3},
					{
						Kind:      patch.Context,
						Text:      "c",
						OldNumber: 3,
						NewNumber: 4,
					},
				},
			},
			{
				Header: "two",
				Lines: []patch.Line{
					{Kind: patch.Addition, Text: "separate", NewNumber: 5},
				},
			},
		}}},
	}
	v := newSideBySideView(p, true)
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	testutil.RequireEqual(t, len(code), 5, "visual code entries = %d",
		len(code),
	)
	testutil.RequireEqual(t, code[0].leftLine, 0, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[0].rightLine, 2, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[1].leftLine, 1, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[1].rightLine, 3, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[2].leftLine, -1, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[2].rightLine, 4, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[4].hunk != code[3].hunk, true,
		"hunks were paired: %#v", code[4],
	)
	testutil.RequireEqual(t, code[4].leftLine, -1, "hunks were paired: %#v",
		code[4],
	)
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, _ := v.search("removed one", first, forward)
	added, _ := v.search("added one", first, forward)
	left := beginSelection(removed)
	left, ok := v.extendSelection(left, removed)
	testutil.RequireEqual(t, ok, true, "left lines = %#v", v.lines(left))
	testutil.RequireEqual(t, len(v.lines(left)), 1, "left lines = %#v",
		v.lines(left),
	)
	testutil.RequireEqual(t, v.lines(left)[0].Kind, patch.Deletion,
		"left lines = %#v", v.lines(left),
	)

	right := beginSelection(added)
	testutil.RequireEqual(t, len(v.lines(right)), 1, "right lines = %#v",
		v.lines(right),
	)
	testutil.RequireEqual(t, v.lines(right)[0].Kind, patch.Addition,
		"right lines = %#v", v.lines(right),
	)
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
					{Kind: patch.Addition, Text: "right", NewNumber: 1},
					{
						Kind:      patch.Context,
						Text:      "both",
						OldNumber: 1,
						NewNumber: 2,
					},
					{Kind: patch.Deletion, Text: "left", OldNumber: 2},
				}}},
			},
		},
	}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, ok := v.switchPane(first, left)
	testutil.RequireEqual(t, ok, true, "%s", "did not find later left pane")

	line, _ := v.line(left)
	testutil.RequireEqual(t, line.Text, "both", "later left line = %q",
		line.Text,
	)

	last, _ := v.last()
	right, ok := v.switchPane(last, right)
	testutil.RequireEqual(t, ok, true, "%s", "did not find earlier right pane")

	line, _ = v.line(right)
	testutil.RequireEqual(t, line.Text, "both", "earlier right line = %q",
		line.Text,
	)
}

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "one", NewNumber: 1},
							{Kind: patch.Addition, Text: "two", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	if _, ok := v.switchPane(cursor, left); ok {
		t.Fatal("switched to empty pane")
	}
}

func TestSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "one",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patch.Addition, Text: "right", NewNumber: 2},
							{
								Kind:      patch.Context,
								Text:      "two",
								OldNumber: 2,
								NewNumber: 3,
							},
							{Kind: patch.Deletion, Text: "left", OldNumber: 3},
							{
								Kind:      patch.Context,
								Text:      "three",
								OldNumber: 4,
								NewNumber: 4,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, _ := v.switchPane(first, left)
	nextLeft, _ := v.move(left, forward)
	line, _ := v.line(nextLeft)
	testutil.RequireEqual(t, line.Text, "two", "left movement = %q", line.Text)

	rightAtContext, _ := v.switchPane(nextLeft, right)
	nextRight, _ := v.move(rightAtContext, forward)
	line, _ = v.line(nextRight)
	testutil.RequireEqual(t, line.Text, "three", "right movement = %q",
		line.Text,
	)
}

func TestSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patch.Line{
		{
			Kind:      patch.Deletion,
			Text:      "d1",
			OldNumber: 1,
		}, {
			Kind:      patch.Addition,
			Text:      "a1",
			NewNumber: 1,
		}, {Kind: patch.Context, Text: "c1", OldNumber: 2, NewNumber: 2},
		{
			Kind:      patch.Deletion,
			Text:      "d2",
			OldNumber: 3,
		}, {
			Kind:      patch.Addition,
			Text:      "a2",
			NewNumber: 3,
		}, {Kind: patch.Context, Text: "c2", OldNumber: 4, NewNumber: 4},
		{
			Kind:      patch.Deletion,
			Text:      "d3",
			OldNumber: 5,
		}, {
			Kind:      patch.Addition,
			Text:      "a3",
			NewNumber: 5,
		}, {Kind: patch.Context, Text: "c3", OldNumber: 6, NewNumber: 6},
	}
	v := newSideBySideView(
		patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "file",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		},
		true,
	)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(120, 4)
	viewport = v.keepVisible(viewport, cursor)
	originalTop := viewport.top
	viewport, moved := v.scrollHalfPage(viewport, cursor, forward)
	testutil.RequireEqual(t, viewport.top > originalTop, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)
	testutil.RequireEqual(t, moved.row > cursor.row, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)

	viewport, moved = v.scrollHalfPage(viewport, moved, backward)
	testutil.RequireEqual(t, viewport.top, originalTop,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
	testutil.RequireEqual(t, moved.row, cursor.row,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
}

func TestFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := stickyHeaderPatch()
	v := newUnifiedView(p, true)
	viewport := v.newViewport(60, 3)
	viewport.top = 3

	rendered := strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, len(rendered), viewport.height,
		"sticky render = %#v", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[0], "first.go"), true,
		"sticky render = %#v", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[1], "first two"), true,
		"sticky header covered content rows: %#v", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[2], "first three"), true,
		"sticky header covered content rows: %#v", rendered,
	)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, strings.Count(
		strings.Join(rendered, "\n"),
		"second.go",
	), 1,
		"file header was duplicated at its natural position: %#v", rendered)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, strings.Contains(rendered[0], "second.go"), true,
		"sticky header did not change with the file: %#v", rendered,
	)
}

func stickyHeaderPatch() patch.Patch {
	p := patch.Patch{Files: []patch.File{
		{
			DisplayPath: "first.go",
			Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "first one",
					OldNumber: 1,
					NewNumber: 1,
				},
				{
					Kind:      patch.Context,
					Text:      "first two",
					OldNumber: 2,
					NewNumber: 2,
				},
				{
					Kind:      patch.Context,
					Text:      "first three",
					OldNumber: 3,
					NewNumber: 3,
				},
			}}},
		},
		{
			DisplayPath: "second.go",
			Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "second one",
					OldNumber: 1,
					NewNumber: 1,
				},
			}}},
		},
	}}
	return p
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	cursor, _ := v.last()
	viewport := v.keepVisible(v.newViewport(50, 4), cursor)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	line, _ := v.line(cursor)
	testutil.RequireEqual(t,
		cursor.row < viewport.top+v.contentHeight(viewport), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
	testutil.RequireEqual(t,
		strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
}

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "\t\tlong line",
								OldNumber: 1,
								NewNumber: 1,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	rendered := ansi.Strip(renderOne(v, cursor, 120, nil))
	testutil.RequireEqual(t, !strings.ContainsRune(rendered, '\t'), true,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	testutil.RequireEqual(t, strings.Index(rendered, "│"), 59,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	testutil.RequireEqual(t, lipgloss.Width(rendered), 120,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
}
