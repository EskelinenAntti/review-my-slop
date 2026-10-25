// Package diffscreen presents patches and owns their interactive browsing
// state.
package diffscreen

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
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

// Direction chooses the order in which search matches are visited.
type Direction int8

// Backward and Forward select the search traversal direction.
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

type searchState struct {
	active           bool
	query, term      string
	from             diffCursor
	miss, repeatMiss bool
}

// New creates a view with the shared default screen dimensions.
func New(p patch.Patch, options Options) *View {
	v := &View{
		options: options,
		width:   ui.DefaultScreenWidth,
		height:  ui.DefaultScreenHeight,
	}
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
	if active != v.sideBySideActive() || dark != options.Dark {
		v.Update(v.view.patch)
	}
}

// Resize accepts full screen dimensions, reserving space for the header/footer.
func (v *View) Resize(width, height int) {
	active := v.sideBySideActive()
	v.width, v.height = width, height
	v.viewport = v.view.resize(v.viewport, width, frame.BodyHeight(height))
	if active != v.sideBySideActive() {
		v.Update(v.view.patch)
	} else {
		v.EndDrag()
		v.viewport = v.view.keepVisible(v.viewport, v.cursor)
	}
}

func (v *View) sideBySideActive() bool {
	return v.options.SideBySide && v.width >= 100
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
		v.step(backward)
	case NextLine:
		v.step(forward)
	case PreviousPage:
		v.halfPage(backward)
	case NextPage:
		v.halfPage(forward)
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

func (v *View) step(direction direction) {
	next, ok := v.view.move(v.cursor, direction)
	if !ok {
		return
	}
	v.applyMovement(v.view.keepVisible(v.viewport, next), next)
}
func (v *View) setCursor(cursor diffCursor) {
	v.cursor = cursor
	v.viewport = v.view.keepVisible(v.viewport, cursor)
}
func (v *View) halfPage(direction direction) {
	viewport, cursor := v.view.scrollHalfPage(v.viewport, v.cursor, direction)
	v.applyMovement(viewport, cursor)
}

// applyMovement commits focus and scrolling together only when the active
// selection can follow the proposed cursor within its original hunk.
func (v *View) applyMovement(viewport diffViewport, cursor diffCursor) {
	if v.selection != nil {
		selection, ok := v.view.selectionBetween(v.selection.First, cursor)
		if !ok {
			return
		}
		v.selection = &selection
	}
	v.viewport, v.cursor = viewport, cursor
}
func (v *View) jumpFile(direction direction) {
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
		selection, ok := v.view.translateSelection(*v.selection,
			func(cursor diffCursor) (diffCursor, bool) {
				return v.view.switchPane(cursor, pane)
			})
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
	if cursor == v.cursor {
		v.viewport = viewport
		return
	}
	v.applyMovement(viewport, cursor)
}

// ToggleSelection starts a range at the focused line or clears the current
// range.
func (v *View) ToggleSelection() {
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	v.search.repeatMiss = false
	if _, ok := v.view.line(v.cursor); ok {
		selection := beginSelection(v.cursor)
		v.selection = &selection
	}
}

// ClearSelection clears the selected range and ends any drag.
func (v *View) ClearSelection() {
	v.EndDrag()
	v.selection = nil
	v.search.repeatMiss = false
}

// Selected returns the focused line when no explicit range is selected.
// The bool is false when the patch has no selectable code.
func (v *View) Selected() (patch.File, []patch.Line, bool) {
	selection := beginSelection(v.cursor)
	if v.selection != nil {
		selection = *v.selection
	}
	file, ok := v.view.file(selection.First)
	lines := v.view.lines(selection)
	return file, lines, ok && len(lines) > 0
}

// Current returns the focused source line, independent of any selected range.
func (v *View) Current() (patch.File, patch.Line, bool) {
	file, fileOK := v.view.file(v.cursor)
	line, lineOK := v.view.line(v.cursor)
	return file, line, fileOK && lineOK
}

// BeginSearch starts an empty search query at the current focus.
func (v *View) BeginSearch() {
	v.ClearSelection()
	v.search.active = true
	v.search.query, v.search.miss = "", false
	v.search.from = v.cursor
}

// InsertSearch appends text to the query and previews from the search origin.
// It starts a search session if needed; a miss retains the last successful
// focus.
func (v *View) InsertSearch(text string) {
	if !v.search.active {
		v.BeginSearch()
	}
	v.previewSearch(v.search.query + text)
}

// BackspaceSearch removes the last Unicode code point from the active query.
// An empty query restores the search origin. Outside a search it does nothing.
func (v *View) BackspaceSearch() {
	if !v.search.active {
		return
	}
	query := []rune(v.search.query)
	if len(query) > 0 {
		query = query[:len(query)-1]
	}
	v.previewSearch(string(query))
}

func (v *View) previewSearch(query string) {
	v.search.query = query
	if query == "" {
		v.setCursor(v.search.from)
		v.search.miss = false
		return
	}
	cursor, ok := v.view.search(query, v.search.from, forward)
	v.search.miss = !ok
	if ok {
		v.setCursor(cursor)
	}
}

// AcceptSearch remembers a successful query for subsequent searches.
func (v *View) AcceptSearch() {
	if !v.search.active {
		return
	}
	if v.search.query != "" && !v.search.miss {
		v.search.term = v.search.query
	}
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// CancelSearch restores the focus from before the active search.
func (v *View) CancelSearch() {
	if !v.search.active {
		return
	}
	v.setCursor(v.search.from)
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// Find moves to the next remembered search match in the requested direction.
func (v *View) Find(d Direction) bool {
	if v.search.term == "" {
		return false
	}
	if d != Forward && d != Backward {
		return false
	}
	cursor, ok := v.view.search(v.search.term, v.cursor, direction(d))
	v.search.repeatMiss = !ok
	if ok {
		v.setCursor(cursor)
	}
	return ok
}
