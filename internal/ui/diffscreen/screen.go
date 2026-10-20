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
// Endpoint motions scan inward: forward for the first line, backward for the last.
func (m Motion) direction() direction {
	switch m {
	case PreviousLine, PreviousPage, LastLine, PreviousFile:
		return backward
	default:
		return forward
	}
}

func (v *View) step(direction direction) {
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

// Current returns the focused source line, independent of any selected range.
func (v *View) Current() (patch.File, patch.Line, bool) {
	file, fileOK := v.view.file(v.cursor)
	line, lineOK := v.view.line(v.cursor)
	return file, line, fileOK && lineOK
}
