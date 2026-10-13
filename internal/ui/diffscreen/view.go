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
	projection    *patchProjection
	cursor        patchCursor
	viewport      patchViewport
	selection     *patchRange
	drag          *patchCursor
	options       Options
	width, height int
	search        searchState
}

// New creates a view with default dimensions of 80 columns and 30 rows.
func New(p patch.Patch, options Options) *View {
	v := &View{options: options, width: 80, height: 30}
	v.projection = v.buildProjection(p)
	v.viewport = v.projection.newViewport(v.width, frame.BodyHeight(v.height))
	v.cursor, _ = v.projection.firstCursor()
	return v
}

// Update replaces the patch while preserving meaningful focus, selection, and
// relative screen position. Missing targets fall back to nearby available code.
func (v *View) Update(p patch.Patch) {
	v.EndDrag()
	v.search.repeatMiss = false
	v.replaceProjection(v.buildProjection(p))
}

// Configure changes presentation options while preserving meaningful position.
func (v *View) Configure(options Options) {
	active := v.sideBySideActive()
	dark := v.options.Dark
	v.options = options
	if active != v.sideBySideActive() || dark != options.Dark {
		v.Update(v.projection.patch)
	}
}

// Resize accepts full screen dimensions, reserving space for the header/footer.
func (v *View) Resize(width, height int) {
	v.EndDrag()
	active := v.sideBySideActive()
	v.width, v.height = width, height
	v.viewport = v.projection.resize(
		v.viewport,
		width,
		frame.BodyHeight(height),
	)
	if active != v.sideBySideActive() {
		v.Update(v.projection.patch)
	} else {
		v.viewport = v.projection.keepCursorVisible(v.viewport, v.cursor)
	}
}

func (v *View) sideBySideActive() bool {
	return v.options.SideBySide && v.width >= 100
}

func (v *View) buildProjection(p patch.Patch) *patchProjection {
	return newPatchProjection(
		p,
		Options{SideBySide: v.sideBySideActive(), Dark: v.options.Dark},
	)
}
