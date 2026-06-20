package view

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

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

type View interface {
	First() (Cursor, bool)
	Last() (Cursor, bool)
	Move(Cursor, Direction) (Cursor, bool)
	Search(string, Cursor, Direction) (Cursor, bool)
	JumpFile(Cursor, Direction) (Cursor, bool)
	SwitchPane(Cursor, Pane) (Cursor, bool)

	NewViewport(width, height int) Viewport
	Resize(Viewport, int, int) Viewport
	KeepVisible(Viewport, Cursor) Viewport
	Align(Viewport, Cursor, VerticalAlignment) Viewport
	ScrollHorizontal(Viewport, int) Viewport
	ScrollHalfPage(Viewport, Cursor, Direction) (Viewport, Cursor)

	BeginSelection(Cursor) Selection
	ExtendSelection(Selection, Cursor) (Selection, bool)
	Lines(Selection) []patch.Line
	Anchor(Selection) (review.Anchor, error)

	File(Cursor) (patch.File, bool)
	Hunk(Cursor) (patch.Hunk, bool)
	Line(Cursor) (patch.Line, bool)

	FindCursor(patch.File, patch.Hunk, patch.Line, Coordinate, Pane) (Cursor, bool)
	Render(Viewport, Cursor, *Selection) string
}
