// Package diffscreen presents patches and owns their interactive browsing
// state.
package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/internal/frame"
)

// Options controls preferred layout and syntax colors. Narrow screens use a
// unified layout while retaining the side-by-side preference.
type Options struct {
	SideBySide bool
	Dark       bool
}

// Motion identifies navigation operations without exposing display coordinates.
type Motion uint8

// PreviousLine starts the supported cursor movement commands.
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

// Top starts the supported viewport alignment choices.
const (
	Top Alignment = iota
	Center
	Bottom
)

// Direction chooses the order in which search matches are visited.
type Direction int8

// Backward starts the supported search directions.
const (
	Backward Direction = -1
	Forward  Direction = 1
)

// View owns focus, selection, search, and scrolling for a patch presentation.
// It has no knowledge of keys, editors, or comment persistence.
type View struct {
	view          *diffView
	cursor        diffCursor
	viewport      diffViewport
	selection     *diffSelection
	drag          *diffCursor
	options       Options
	width, height int
	search        searchState
}

// New creates a view with default dimensions of 80 columns and 30 rows.
func New(p patch.Patch, options Options) *View {
	v := &View{options: options, width: defaultWidth, height: defaultHeight}
	v.view = v.newReviewView(p)
	v.viewport = v.view.newViewport(v.width, frame.BodyHeight(v.height))
	v.cursor, _ = v.view.first()
	return v
}

// Update replaces the patch while preserving meaningful focus, selection, and
// relative screen position. Missing targets fall back to nearby available code.
func (v *View) Update(p patch.Patch) {
	v.EndDrag()
	v.search.repeatMiss = false
	v.replaceView(v.newReviewView(p))
}

// Configure changes presentation options while preserving meaningful position.
func (v *View) Configure(options Options) {
	active := v.sideBySideActive()
	dark := v.options.Dark
	v.options = options
	if dark != options.Dark || active != v.sideBySideActive() {
		v.Update(v.view.patch)
	}
}

// Resize accepts full screen dimensions, reserving space for the header/footer.
func (v *View) Resize(width, height int) {
	v.EndDrag()
	active := v.sideBySideActive()
	v.width, v.height = width, height
	v.viewport = v.view.resize(v.viewport, width, frame.BodyHeight(height))
	if active != v.sideBySideActive() {
		v.Update(v.view.patch)
	} else {
		v.viewport = v.view.keepVisible(v.viewport, v.cursor)
	}
}

func (v *View) sideBySideActive() bool {
	return v.options.SideBySide && v.width >= minimumSplitWidth
}

func (v *View) newReviewView(p patch.Patch) *diffView {
	return newDiffView(
		p,
		Options{SideBySide: v.sideBySideActive(), Dark: v.options.Dark},
	)
}

// Move performs navigation and keeps the resulting focus visible. Line and
// page movement extend active selection only within its original hunk.
func (v *View) Move(motion Motion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case PreviousLine:
		v.moveLine(backward)
	case NextLine:
		v.moveLine(forward)
	case PreviousPage:
		v.halfPage(backward)
	case NextPage:
		v.halfPage(forward)

	default:
		v.movePosition(motion)
	}
}

func (v *View) moveLine(direction direction) {
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

func (v *View) halfPage(direction direction) {
	viewport, cursor := v.view.scrollHalfPage(v.viewport, v.cursor, direction)
	if !v.extendSelectionTo(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

// extendSelectionTo accepts unrestricted movement when no range is active.
// An active range changes only when its endpoint remains in the original hunk.
func (v *View) extendSelectionTo(cursor diffCursor) bool {
	if v.selection == nil {
		return true
	}
	selection, ok := v.view.extendSelection(*v.selection, cursor)
	if ok {
		v.selection = &selection
	}
	return ok
}

func (v *View) jumpFile(direction direction) {
	v.ClearSelection()
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
		first, firstOK := v.view.switchPane(v.selection.First, pane)
		last, lastOK := v.view.switchPane(v.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := v.view.beginSelection(first)
		selection, ok = v.view.extendSelection(selection, last)
		if !ok {
			return
		}
		v.selection = &selection
	}
	v.setCursor(cursor)
}

// Align positions the cursor at the requested viewport alignment.
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

// ScrollHorizontal moves the content window by the requested columns.
func (v *View) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = v.view.scrollHorizontal(v.viewport, columns)
}

// ScrollVertical moves content first, moving focus only when it leaves the
// visible code. Like keyboard movement, it extends selection within its hunk.
func (v *View) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	viewport, cursor := v.view.scrollVertical(v.viewport, v.cursor, rows)
	if cursor != v.cursor && !v.extendSelectionTo(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

// ToggleSelection starts or ends a range selection at the cursor.
func (v *View) ToggleSelection() {
	v.search.repeatMiss = false
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	if _, ok := v.view.line(v.cursor); ok {
		selection := v.view.beginSelection(v.cursor)
		v.selection = &selection
	}
}

// ClearSelection removes the current range selection.
func (v *View) ClearSelection() {
	v.EndDrag()
	v.selection = nil
	v.search.repeatMiss = false
}

// Selected returns the focused line when no explicit range is selected.
// The bool is false when the patch has no selectable code.
func (v *View) Selected() (patch.File, []patch.Line, bool) {
	selection := v.selection
	if selection == nil {
		current := v.view.beginSelection(v.cursor)
		selection = &current
	}
	file, ok := v.view.file(selection.First)
	lines := v.view.lines(*selection)
	return file, lines, ok && len(lines) > 0
}

// Current returns the focused source line, independent of any selected range.
func (v *View) Current() (patch.File, patch.Line, bool) {
	file, fileOK := v.view.file(v.cursor)
	line, lineOK := v.view.line(v.cursor)
	return file, line, fileOK && lineOK
}

func (v *View) movePosition(motion Motion) {
	switch motion {
	case FirstLine:
		if cursor, ok := v.view.first(); ok {
			v.setCursor(cursor)
		}
	case LastLine:
		if cursor, ok := v.view.last(); ok {
			v.setCursor(cursor)
		}
	case PreviousFile:
		v.jumpFile(backward)
	case NextFile:
		v.jumpFile(forward)

	default:
		v.movePane(motion)
	}
}

func (v *View) movePane(motion Motion) {
	switch motion {
	case OldPane:
		v.switchPane(left)
	case NewPane:
		v.switchPane(right)
	case OtherPane:
		v.switchPane(v.cursor.pane.other())
	default:
		// Unknown motions leave the pane unchanged.
	}
}
