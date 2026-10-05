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
	top           int
	LeftColumn    int
	Width, Height int
}

type diffSelection struct {
	First diffCursor
	Last  diffCursor
}
