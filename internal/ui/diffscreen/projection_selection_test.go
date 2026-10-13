package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectionLines(t *testing.T) {
	v := newUnifiedPatchView(twoFileReviewPatch(), true)
	firstCursor, _ := v.firstCursor()
	lastCursor, _ := v.moveCursor(firstCursor, forward)
	lastCursor, _ = v.moveCursor(lastCursor, forward)
	selection := beginRange(firstCursor)
	selection, ok := v.extendRange(selection, lastCursor)
	testutil.RequireEqual(t, ok, true, "selection extension failed")

	lines := v.selectedLines(selection)
	testutil.RequireEqual(t, len(lines), 3,
		fmt.Sprintf("selected lines = %d", len(lines)),
	)

	nextFile, _ := v.jumpToAdjacentFile(firstCursor, forward)
	if _, ok := v.extendRange(selection, nextFile); ok {
		t.Fatal("selection crossed a hunk")
	}
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := newSplitPatchView(twoFileReviewPatch(), true)
	firstCursor := mustFirstCursor(t, v)
	removed, _ := v.searchFromCursor("removed one", firstCursor, forward)
	added, _ := v.searchFromCursor("added one", firstCursor, forward)
	left := beginRange(removed)
	left, ok := v.extendRange(left, removed)
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("left lines = %#v", v.selectedLines(left)),
	)
	testutil.RequireEqual(t, len(v.selectedLines(left)), 1,
		fmt.Sprintf("left lines = %#v", v.selectedLines(left)),
	)
	testutil.RequireEqual(t, v.selectedLines(left)[0].Kind, patch.Deletion,
		fmt.Sprintf("left lines = %#v", v.selectedLines(left)),
	)

	right := beginRange(added)
	testutil.RequireEqual(t, len(v.selectedLines(right)), 1,
		fmt.Sprintf("right lines = %#v", v.selectedLines(right)),
	)
	testutil.RequireEqual(t, v.selectedLines(right)[0].Kind, patch.Addition,
		fmt.Sprintf("right lines = %#v", v.selectedLines(right)),
	)
}
