package diffscreen

// Motion identifies navigation operations without exposing display coordinates.
type Motion uint8

// Navigation motions move focus between lines, pages, files, and panes.
const (
	PreviousLine Motion = iota
	NextLine
	PreviousPage
	NextPage
	FirstLine
	LastLine
	PreviousFile
	NextFile
	OldPane
	NewPane
	OtherPane
)

// Alignment places the focused line within the visible body.
type Alignment uint8

// Top, Center, and Bottom select the focused line position in the viewport.
const (
	Top Alignment = iota
	Center
	Bottom
)

// Move performs navigation and keeps the resulting focus visible. Line and
// page movement extend active selection only within its original hunk.
func (v *View) Move(motion Motion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case PreviousLine, NextLine:
		v.step(motion.direction())
	case PreviousPage, NextPage:
		v.halfPage(motion.direction())
	case FirstLine, LastLine:
		if cursor, ok := v.view.edge(motion.direction()); ok {
			v.setCursor(cursor)
		}
	case PreviousFile, NextFile:
		v.jumpFile(motion.direction())
	case OldPane:
		v.switchPane(left)
	case NewPane:
		v.switchPane(right)
	case OtherPane:
		v.switchPane(v.cursor.pane.other())
	default:
		return
	}
}

// direction maps paired navigation motions to their traversal direction.
// Endpoint motions scan inward: forward for the first line, backward for
// the last.
func (m Motion) direction() Direction {
	switch m {
	case PreviousLine, PreviousPage, LastLine, PreviousFile:
		return Backward
	default:
		return Forward
	}
}

func (v *View) step(direction Direction) {
	next, ok := v.view.move(v.cursor, direction)
	if !ok {
		return
	}
	if !v.extendSelectionTo(next) {
		return
	}
	v.setCursor(next)
}
func (v *View) setCursor(cursor diffCursor) {
	v.cursor = cursor
	v.viewport = v.view.keepVisible(v.viewport, cursor)
}
func (v *View) halfPage(direction Direction) {
	viewport, cursor := v.view.scrollHalfPage(v.viewport, v.cursor, direction)
	v.applyScroll(viewport, cursor)
}

// applyScroll keeps focus, viewport, and selection changes atomic when a
// proposed scroll would extend the selection beyond its original hunk.
func (v *View) applyScroll(viewport diffViewport, cursor diffCursor) {
	if cursor != v.cursor && !v.extendSelectionTo(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

func (v *View) jumpFile(direction Direction) {
	v.selection = nil
	if cursor, ok := v.view.jumpFile(v.cursor, direction); ok {
		v.setCursor(cursor)
	}
}
func (v *View) switchPane(pane diffPane) {
	cursor, ok := v.view.switchPane(v.cursor, pane)
	if !ok {
		return
	}
	if v.selection != nil {
		selection, ok := v.view.switchSelectionPane(*v.selection, pane)
		if !ok {
			return
		}
		v.selection = &selection
	}
	v.setCursor(cursor)
}

// Align positions the focused line at the requested viewport alignment.
func (v *View) Align(alignment Alignment) {
	v.search.repeatMiss = false
	if _, ok := v.view.line(v.cursor); !ok {
		return
	}
	v.viewport = v.view.align(
		v.viewport,
		v.cursor,
		verticalAlignment(alignment),
	)
}

// ScrollHorizontal shifts the visible content by the requested columns.
func (v *View) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = v.view.scrollHorizontal(v.viewport, columns)
}

// ScrollVertical moves content first, moving focus only when it leaves the
// visible code. Like keyboard movement, it extends selection within its hunk.
func (v *View) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	viewport, cursor := v.view.scrollVertical(v.viewport, v.cursor, rows)
	v.applyScroll(viewport, cursor)
}
