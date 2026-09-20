package view

import "math"

// Command is one decoded navigation transition. Its implementation is private
// so callers can only construct supported navigation intents.
type Command struct {
	apply func(View, State) (State, Outcome)
}

func Move(direction Direction) Command {
	return cursorCommand(
		func(v View, cursor Cursor) (Cursor, bool) { return v.Move(cursor, direction) },
		extendSelection,
		NoOutcome,
	)
}

func First() Command {
	return cursorCommand(
		func(v View, _ Cursor) (Cursor, bool) { return v.First() },
		keepSelection,
		NoOutcome,
	)
}

func Last() Command {
	return cursorCommand(
		func(v View, _ Cursor) (Cursor, bool) { return v.Last() },
		keepSelection,
		NoOutcome,
	)
}

func HalfPage(direction Direction) Command {
	return Command{apply: func(v View, state State) (State, Outcome) {
		viewport, cursor := v.ScrollHalfPage(state.Viewport, *state.Cursor, direction)
		state.Viewport = viewport
		return moveCursor(v, state, cursor, extendSelection), NoOutcome
	}}
}

func JumpFile(direction Direction) Command {
	return Command{apply: func(v View, state State) (State, Outcome) {
		state.Selection = nil
		cursor, ok := v.JumpFile(*state.Cursor, direction)
		if !ok {
			return state, NoOutcome
		}
		return moveCursor(v, state, cursor, keepSelection), NoOutcome
	}}
}

func SwitchPane(pane Pane) Command {
	return Command{apply: func(v View, state State) (State, Outcome) {
		cursor, ok := v.SwitchPane(*state.Cursor, pane)
		if !ok {
			return state, NoOutcome
		}
		state, ok = switchSelection(v, state, pane)
		if !ok {
			return state, NoOutcome
		}
		return moveCursor(v, state, cursor, keepSelection), NoOutcome
	}}
}

func Search(query string, direction Direction) Command {
	return cursorCommand(
		func(v View, cursor Cursor) (Cursor, bool) { return v.Search(query, cursor, direction) },
		clearSelection,
		NoMatch,
	)
}

func ScrollColumns(delta int) Command {
	return Command{apply: func(v View, state State) (State, Outcome) {
		state.Viewport = v.ScrollHorizontal(state.Viewport, delta)
		return state, NoOutcome
	}}
}

func ScrollToStart() Command {
	return Command{apply: func(_ View, state State) (State, Outcome) {
		state.Viewport.LeftColumn = 0
		return state, NoOutcome
	}}
}

func ScrollToEnd() Command {
	return ScrollColumns(math.MaxInt)
}

func Align(alignment VerticalAlignment) Command {
	return Command{apply: func(v View, state State) (State, Outcome) {
		state.Viewport = v.Align(state.Viewport, *state.Cursor, alignment)
		return state, NoOutcome
	}}
}

// Outcome is feedback that the TUI may present after navigation.
type Outcome uint8

const (
	NoOutcome Outcome = iota
	NoMatch
)

// Navigate applies command to state in v. It never mutates its inputs.
func Navigate(v View, state State, command Command) (State, Outcome) {
	if state.Cursor == nil || command.apply == nil {
		return state, NoOutcome
	}
	return command.apply(v, state)
}

type selectionEffect uint8

const (
	keepSelection selectionEffect = iota
	extendSelection
	clearSelection
)

func cursorCommand(target func(View, Cursor) (Cursor, bool), effect selectionEffect, missed Outcome) Command {
	return Command{apply: func(v View, state State) (State, Outcome) {
		cursor, ok := target(v, *state.Cursor)
		if !ok {
			return state, missed
		}
		if effect == clearSelection {
			state.Selection = nil
		}
		return moveCursor(v, state, cursor, effect), NoOutcome
	}}
}

func moveCursor(v View, state State, cursor Cursor, effect selectionEffect) State {
	if effect == extendSelection && state.Selection != nil {
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
