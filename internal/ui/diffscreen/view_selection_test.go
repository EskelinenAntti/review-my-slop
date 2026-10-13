package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
	v := New(twoFileReviewPatch(), Options{Dark: true})
	file, lines, ok := v.Selected()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("default selection=%#v %#v %v", file, lines, ok),
	)
	testutil.RequireEqual(t, file.NewPath, "first.go",
		fmt.Sprintf("default selection=%#v %#v %v", file, lines, ok),
	)
	testutil.RequireEqual(t, len(lines), 1,
		fmt.Sprintf("default selection=%#v %#v %v", file, lines, ok),
	)
	testutil.RequireEqual(t, lines[0].Text, "before",
		fmt.Sprintf("default selection=%#v %#v %v", file, lines, ok),
	)

	v.ToggleSelection()
	v.Move(NextLine)
	_, lines, ok = v.Selected()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("extended selection=%#v", lines),
	)
	testutil.RequireEqual(t, len(lines), 2,
		fmt.Sprintf("extended selection=%#v", lines),
	)

	before := v.cursor
	for range 20 {
		v.Move(NextLine)
	}
	testutil.RequireEqual(t, v.cursor.row >= before.row, true,
		"selection moved backward",
	)

	file, _, _ = v.Selected()
	testutil.RequireEqual(t, file.NewPath, "first.go",
		"selection crossed file",
	)

	v.ClearSelection()
	_, lines, ok = v.Selected()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("cleared selection=%#v", lines),
	)
	testutil.RequireEqual(t, len(lines), 1,
		fmt.Sprintf("cleared selection=%#v", lines),
	)
}

func TestPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
	v := New(twoFileReviewPatch(), Options{Dark: true})
	v.Resize(80, 30)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(NextPage)
	testutil.RequireEqual(t, v.cursor, beforeCursor,
		"rejected selection partially changed state",
	)
	testutil.RequireEqual(t, v.viewport, beforeViewport,
		"rejected selection partially changed state",
	)
}
