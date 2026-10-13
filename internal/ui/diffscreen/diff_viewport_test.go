package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestViewportAlignmentResizeAndScrolling(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.first()
	cursor := first
	for range 8 {
		cursor, _ = v.move(cursor, forward)
	}
	viewport := v.newViewport(30, 5)
	viewport = v.keepVisible(viewport, cursor)
	testutil.RequireEqual(t, cursor.row >= viewport.top, true,
		fmt.Sprintf("cursor not visible: %#v %#v", cursor, viewport),
	)
	testutil.RequireEqual(t, cursor.row < viewport.top+viewport.Height, true,
		fmt.Sprintf("cursor not visible: %#v %#v", cursor, viewport),
	)

	viewport = v.align(viewport, cursor, middle)
	headerHeight := 0
	if v.hasStickyHeader(viewport.top, viewport.Height) {
		headerHeight = 1
	}
	testutil.RequireEqual(t, headerHeight+cursor.row-viewport.top,
		viewport.Height/2, fmt.Sprintf("middle alignment = %#v", viewport),
	)

	viewport = v.scrollHorizontal(viewport, 4)
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true,
		"horizontal scroll did not move",
	)

	viewport = v.resize(viewport, 20, 3)
	testutil.RequireEqual(t, viewport.Width, 20,
		fmt.Sprintf("resize = %#v", viewport),
	)
	testutil.RequireEqual(t, viewport.Height, 3,
		fmt.Sprintf("resize = %#v", viewport),
	)

	before := cursor
	viewport, cursor = v.scrollHalfPage(viewport, cursor, forward)
	testutil.RequireEqual(t, cursor.row >= before.row, true,
		fmt.Sprintf("half page moved backward: %#v -> %#v", before, cursor),
	)
}

func TestViewportProgressUsesVisibleBottom(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	viewport := v.newViewport(30, 5)
	progress := v.viewportProgress(viewport)
	if progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}
	last, _ := v.last()
	viewport = v.keepVisible(viewport, last)
	if progress := v.viewportProgress(viewport); progress != 100 {
		t.Fatalf("final progress=%d", progress)
	}
}

func TestHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	first, _ := v.first()
	last, _ := v.last()
	viewport := v.newViewport(30, 5)
	cursor := first

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.scrollHalfPage(viewport, cursor, forward)
	}
	testutil.RequireEqual(t, cursor, last,
		fmt.Sprintf("cursor after scrolling down = %#v, want %#v", cursor,
			last,
		),
	)

	for range len(longPatch().Files[0].Hunks[0].Lines) {
		viewport, cursor = v.scrollHalfPage(viewport, cursor, backward)
	}
	testutil.RequireEqual(t, cursor, first,
		fmt.Sprintf("cursor after scrolling up = %#v, want %#v", cursor, first),
	)
}
