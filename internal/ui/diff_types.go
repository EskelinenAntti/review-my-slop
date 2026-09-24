package ui

type Coordinate struct {
	Y int
}

type Pane uint8

const (
	Left Pane = iota
	Right
)

func (pane Pane) Other() Pane {
	if pane == Left {
		return Right
	}
	return Left
}

type Cursor struct {
	Coordinate Coordinate
	Pane       Pane
}

type Viewport struct {
	Top        Coordinate
	LeftColumn int
	Width      int
	Height     int
}

type Selection struct {
	First Cursor
	Last  Cursor
}

// State is the cursor, selection, and viewport associated with a View.
// Cursor and Selection are nil when the View has no selectable line.
type State struct {
	Cursor    *Cursor
	Selection *Selection
	Viewport  Viewport
}

type Direction int8

const (
	Backward Direction = -1
	Forward  Direction = 1
)

type VerticalAlignment uint8

const (
	Top VerticalAlignment = iota
	Middle
	Bottom
)

// View is the repository's concrete diff view.
type View = *diffView
