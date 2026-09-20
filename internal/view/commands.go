package view

import "math"

type commandKind uint8

const (
	moveCommand commandKind = iota
	firstCommand
	lastCommand
	halfPageCommand
	jumpFileCommand
	switchPaneCommand
	searchCommand
	scrollColumnsCommand
	scrollStartCommand
	scrollEndCommand
	alignCommand
)

// Command describes one decoded navigation intent.
type Command struct {
	kind      commandKind
	direction Direction
	pane      Pane
	query     string
	delta     int
	alignment VerticalAlignment
}

func Move(direction Direction) Command { return Command{kind: moveCommand, direction: direction} }
func First() Command                   { return Command{kind: firstCommand} }
func Last() Command                    { return Command{kind: lastCommand} }
func HalfPage(direction Direction) Command {
	return Command{kind: halfPageCommand, direction: direction}
}
func JumpFile(direction Direction) Command {
	return Command{kind: jumpFileCommand, direction: direction}
}
func SwitchPane(pane Pane) Command { return Command{kind: switchPaneCommand, pane: pane} }
func Search(query string, direction Direction) Command {
	return Command{kind: searchCommand, query: query, direction: direction}
}
func ScrollColumns(delta int) Command { return Command{kind: scrollColumnsCommand, delta: delta} }
func ScrollToStart() Command          { return Command{kind: scrollStartCommand} }
func ScrollToEnd() Command            { return Command{kind: scrollEndCommand} }
func Align(alignment VerticalAlignment) Command {
	return Command{kind: alignCommand, alignment: alignment}
}

// Outcome is feedback that the TUI may present after navigation.
type Outcome uint8

const (
	NoOutcome Outcome = iota
	NoMatch
)

// Navigate applies command to state in v. It never mutates its inputs.
func Navigate(v View, state State, command Command) (State, Outcome) {
	if state.Cursor == nil {
		return state, NoOutcome
	}
	cursor := *state.Cursor

	switch command.kind {
	case moveCommand:
		next, ok := v.Move(cursor, command.direction)
		if !ok {
			return state, NoOutcome
		}
		return moveCursor(v, state, next, true), NoOutcome
	case firstCommand:
		next, ok := v.First()
		if !ok {
			return state, NoOutcome
		}
		return moveCursor(v, state, next, false), NoOutcome
	case lastCommand:
		next, ok := v.Last()
		if !ok {
			return state, NoOutcome
		}
		return moveCursor(v, state, next, false), NoOutcome
	case halfPageCommand:
		viewport, next := v.ScrollHalfPage(state.Viewport, cursor, command.direction)
		nextState := state
		nextState.Viewport = viewport
		return moveCursor(v, nextState, next, true), NoOutcome
	case jumpFileCommand:
		nextState := state
		nextState.Selection = nil
		next, ok := v.JumpFile(cursor, command.direction)
		if !ok {
			return nextState, NoOutcome
		}
		return moveCursor(v, nextState, next, false), NoOutcome
	case switchPaneCommand:
		next, ok := v.SwitchPane(cursor, command.pane)
		if !ok {
			return state, NoOutcome
		}
		nextState, ok := switchSelection(v, state, command.pane)
		if !ok {
			return state, NoOutcome
		}
		return moveCursor(v, nextState, next, false), NoOutcome
	case searchCommand:
		next, ok := v.Search(command.query, cursor, command.direction)
		if !ok {
			return state, NoMatch
		}
		nextState := state
		nextState.Selection = nil
		return moveCursor(v, nextState, next, false), NoOutcome
	case scrollColumnsCommand:
		nextState := state
		nextState.Viewport = v.ScrollHorizontal(state.Viewport, command.delta)
		return nextState, NoOutcome
	case scrollStartCommand:
		nextState := state
		nextState.Viewport.LeftColumn = 0
		return nextState, NoOutcome
	case scrollEndCommand:
		nextState := state
		nextState.Viewport = v.ScrollHorizontal(state.Viewport, math.MaxInt)
		return nextState, NoOutcome
	case alignCommand:
		nextState := state
		nextState.Viewport = v.Align(state.Viewport, cursor, command.alignment)
		return nextState, NoOutcome
	default:
		return state, NoOutcome
	}
}

func moveCursor(v View, state State, cursor Cursor, extendSelection bool) State {
	if extendSelection && state.Selection != nil {
		selection, ok := v.ExtendSelection(*state.Selection, cursor)
		if !ok {
			return state
		}
		state.Selection = &selection
	}
	state.Cursor = &cursor
	state.Viewport = v.KeepVisible(state.Viewport, cursor)
	return state
}

func switchSelection(v View, state State, pane Pane) (State, bool) {
	if state.Selection == nil {
		return state, true
	}
	first, firstOK := v.SwitchPane(state.Selection.First, pane)
	last, lastOK := v.SwitchPane(state.Selection.Last, pane)
	if !firstOK || !lastOK {
		return state, false
	}
	selection := v.BeginSelection(first)
	selection, ok := v.ExtendSelection(selection, last)
	if !ok {
		return state, false
	}
	state.Selection = &selection
	return state, true
}
