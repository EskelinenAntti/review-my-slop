package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
	v := newSplitPatchView(
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
	cursor := mustFirstCursor(t, v)
	viewport := v.newViewport(120, 4)
	viewport = v.keepCursorVisible(viewport, cursor)
	originalTop := viewport.top
	viewport, moved := v.scrollHalfPage(viewport, cursor, forward)
	testutil.RequireEqual(t, viewport.top > originalTop, true,
		fmt.Sprintf("viewport=%#v cursor=%#v", viewport, moved),
	)
	testutil.RequireEqual(t, moved.row > cursor.row, true,
		fmt.Sprintf("viewport=%#v cursor=%#v", viewport, moved),
	)

	viewport, moved = v.scrollHalfPage(viewport, moved, backward)
	testutil.RequireEqual(t, viewport.top, originalTop,
		fmt.Sprintf("round trip viewport=%#v cursor=%#v", viewport, moved),
	)
	testutil.RequireEqual(t, moved.row, cursor.row,
		fmt.Sprintf("round trip viewport=%#v cursor=%#v", viewport, moved),
	)
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
	v := newSplitPatchView(p, true)
	firstCursor := mustFirstCursor(t, v)
	left, _ := v.switchCursorPane(firstCursor, left)
	nextLeft, _ := v.moveCursor(left, forward)
	line, _ := v.lineAtCursor(nextLeft)
	testutil.RequireEqual(t, line.Text, "two",
		fmt.Sprintf("left movement = %q", line.Text),
	)

	rightAtContext, _ := v.switchCursorPane(nextLeft, right)
	nextRight, _ := v.moveCursor(rightAtContext, forward)
	line, _ = v.lineAtCursor(nextRight)
	testutil.RequireEqual(t, line.Text, "three",
		fmt.Sprintf("right movement = %q", line.Text),
	)
}
