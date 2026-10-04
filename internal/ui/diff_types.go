package ui

import (
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type coordinate struct {
	Y int
}

type diffPane uint8

const (
	left diffPane = iota
	right
)

func (pane diffPane) other() diffPane {
	if pane == left {
		return right
	}
	return left
}

type diffCursor struct {
	coordinate coordinate
	pane       diffPane
}

type diffViewport struct {
	top        coordinate
	LeftColumn int
	Width      int
	Height     int
}

type diffSelection struct {
	First diffCursor
	Last  diffCursor
}

// viewState is the cursor, selection, and viewport associated with a reviewView.
// diffCursor and diffSelection are nil when the reviewView has no selectable line.
type viewState struct {
	cursor    *diffCursor
	selection *diffSelection
	viewport  diffViewport
}

type direction int8

const (
	backward direction = -1
	forward  direction = 1
)

type verticalAlignment uint8

const (
	top verticalAlignment = iota
	middle
	bottom
)

type reviewView interface {
	first() (diffCursor, bool)
	last() (diffCursor, bool)
	move(diffCursor, direction) (diffCursor, bool)
	search(string, diffCursor, direction) (diffCursor, bool)
	jumpFile(diffCursor, direction) (diffCursor, bool)
	switchPane(diffCursor, diffPane) (diffCursor, bool)

	newViewport(width, height int) diffViewport
	resize(diffViewport, int, int) diffViewport
	keepVisible(diffViewport, diffCursor) diffViewport
	align(diffViewport, diffCursor, verticalAlignment) diffViewport
	scrollHorizontal(diffViewport, int) diffViewport
	scrollHalfPage(diffViewport, diffCursor, direction) (diffViewport, diffCursor)
	viewportProgress(diffViewport) int

	beginSelection(diffCursor) diffSelection
	extendSelection(diffSelection, diffCursor) (diffSelection, bool)
	lines(diffSelection) []patch.Line
	anchor(diffSelection) (comments.Anchor, error)

	file(diffCursor) (patch.File, bool)
	hunk(diffCursor) (patch.Hunk, bool)
	line(diffCursor) (patch.Line, bool)

	findCursor(patch.File, patch.Hunk, patch.Line, coordinate, diffPane) (diffCursor, bool)
	render(diffViewport, diffCursor, *diffSelection) string
}
