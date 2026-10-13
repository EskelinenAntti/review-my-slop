package diffscreen

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
	row  int
	pane diffPane
}

type diffViewport struct {
	top        int
	LeftColumn int
	Width      int
	Height     int
}

type diffSelection struct {
	First diffCursor
	Last  diffCursor
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
