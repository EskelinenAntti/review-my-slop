package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(t *testing.T) {
	p := unequalChangePatch()
	v := newSideBySideView(p, true)
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	requireEqual(t, len(code), 5, "visual code entries = %d", len(code))
	requireEqual(
		t,
		code[0].leftLine == 0 && code[0].rightLine == 2 &&
			code[1].leftLine == 1 &&
			code[1].rightLine == 3 &&
			code[2].leftLine == -1 &&
			code[2].rightLine == 4,
		true,
		"pairing = %#v",
		code[:3],
	)
	requireEqual(
		t,
		code[4].hunk != code[3].hunk && code[4].leftLine == -1,
		true,
		"hunks were paired: %#v",
		code[4],
	)
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first := mustFirst(t, v)
	removed, _ := v.search(fixtureRemovedOne, first, forward)
	added, _ := v.search("added one", first, forward)
	left := v.beginSelection(removed)
	left, ok := v.extendSelection(left, removed)
	requireEqual(
		t,
		ok && len(v.lines(left)) == 1 &&
			v.lines(left)[0].Kind == patch.Deletion,
		true,
		"left lines = %#v",
		v.lines(left),
	)
	right := v.beginSelection(added)
	requireEqual(
		t,
		len(v.lines(right)) == 1 &&
			v.lines(right)[0].Kind == patch.Addition,
		true,
		"right lines = %#v",
		v.lines(right),
	)
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,

				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,

						Lines: []patch.Line{
							{
								Kind:      patch.Addition,
								Text:      "right",
								NewNumber: 1,
							},

							{
								Kind:      patch.Context,
								Text:      fixtureBoth,
								OldNumber: 1,
								NewNumber: 2,
							},

							{
								Kind:      patch.Deletion,
								Text:      "left",
								OldNumber: 2,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	first := mustFirst(t, v)
	left, ok := v.switchPane(first, left)
	requireEqual(t, ok, true, fixtureV, "did not find later left pane")
	line, _ := v.line(left)
	requireEqual(t, line.Text, fixtureBoth, "later left line = %q", line.Text)
	last, _ := v.last()
	right, ok := v.switchPane(last, right)
	requireEqual(t, ok, true, fixtureV, "did not find earlier right pane")
	line, _ = v.line(right)
	requireEqual(
		t,
		line.Text, fixtureBoth,
		"earlier right line = %q",
		line.Text,
	)
}

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Addition,
								Text:      fixtureOne,
								NewNumber: 1,
							},
							{
								Kind:      patch.Addition,
								Text:      fixtureTwo,
								NewNumber: 2,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	{
		_, ok := v.switchPane(cursor, left)
		requireEqual(t, !ok, true, fixtureV, "switched to empty pane")
	}
}

func TestSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      fixtureOne,
								OldNumber: 1,
								NewNumber: 1,
							},
							{
								Kind:      patch.Addition,
								Text:      "right",
								NewNumber: 2,
							},
							{
								Kind:      patch.Context,
								Text:      fixtureTwo,
								OldNumber: 2,
								NewNumber: 3,
							},
							{
								Kind:      patch.Deletion,
								Text:      "left",
								OldNumber: 3,
							},
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
	requireEqual(t, line.Text, fixtureTwo, "left movement = %q", line.Text)
	rightAtContext, _ := v.switchPane(nextLeft, right)
	nextRight, _ := v.move(rightAtContext, forward)
	line, _ = v.line(nextRight)
	requireEqual(t, line.Text, "three", "right movement = %q", line.Text)
}

func TestSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Deletion, Text: "d1", OldNumber: 1},
		{Kind: patch.Addition, Text: "a1", NewNumber: 1},
		{Kind: patch.Context, Text: "c1", OldNumber: 2, NewNumber: 2},

		{Kind: patch.Deletion, Text: "d2", OldNumber: 3},
		{Kind: patch.Addition, Text: "a2", NewNumber: 3},
		{Kind: patch.Context, Text: "c2", OldNumber: 4, NewNumber: 4},

		{Kind: patch.Deletion, Text: "d3", OldNumber: 5},
		{Kind: patch.Addition, Text: "a3", NewNumber: 5},
		{Kind: patch.Context, Text: "c3", OldNumber: 6, NewNumber: 6},
	}
	v := newSideBySideView(

		patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: fixtureFile,

					Hunks: []patch.Hunk{
						{
							Header: hunkMarker,
							Lines:  lines,
						},
					},
				},
			},
		},

		true,
	)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(wideTestWidth, 4)
	viewport = v.keepVisible(viewport, cursor)
	originalTop := viewport.top
	viewport, moved := v.scrollHalfPage(viewport, cursor, forward)
	requireEqual(
		t,
		viewport.top > originalTop && moved.row > cursor.row, true,
		"viewport=%#v cursor=%#v",
		viewport,
		moved,
	)
	viewport, moved = v.scrollHalfPage(viewport, moved, backward)
	requireEqual(
		t,
		viewport.top == originalTop && moved.row == cursor.row, true,
		"round trip viewport=%#v cursor=%#v",
		viewport,
		moved,
	)
}
