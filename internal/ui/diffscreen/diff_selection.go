package diffscreen

// findCursor searches the file by line numbers, then matching text and kind,
// then source proximity. Ties keep the preferred pane.
type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
}
