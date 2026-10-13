package diffscreen

type patchPane uint8

const (
	left patchPane = iota
	right
)

func (pane patchPane) other() patchPane {
	if pane == left {
		return right
	}
	return left
}

type patchCursor struct {
	row  int
	pane patchPane
}

type patchViewport struct {
	top        int
	LeftColumn int
	Width      int
	Height     int
}

type patchRange struct {
	First patchCursor
	Last  patchCursor
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
