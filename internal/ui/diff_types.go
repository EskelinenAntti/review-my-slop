package ui

type (
	Pane   uint8
	Cursor struct {
		Coordinate int
		Pane       Pane
	}
	Viewport struct {
		Top, LeftColumn, Width, Height int
	}
	Selection struct {
		First, Last Cursor
	}
	// State is the cursor, selection, and viewport associated with a View.
	// Cursor and Selection are nil when the View has no selectable line.
	State struct {
		Cursor    *Cursor
		Selection *Selection
		Viewport  Viewport
	}
	Direction int8
)

const (
	// Pane values identify the left and right side of a split diff.
	Left Pane = iota
	Right
	// Direction values are used for navigation steps.
	Backward Direction = -1
	Forward  Direction = 1
)
