package diffscreen

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

// viewState is the cursor, selection, and viewport associated with a diff projection.
// diffCursor and diffSelection are nil when the diff projection has no selectable line.
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
