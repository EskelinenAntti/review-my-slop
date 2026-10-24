// Package diffscreen presents patches and owns their interactive browsing
// state.
package diffscreen

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
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

// ToggleSelection starts a range at the focused line or clears the current
// range.
func (v *View) ToggleSelection() {
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	v.search.repeatMiss = false
	if selection, ok := v.view.selectionBetween(v.cursor, v.cursor); ok {
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
	selection, ok := v.view.selectionBetween(v.cursor, v.cursor)
	if v.selection != nil {
		selection = *v.selection
	} else if !ok {
		return patch.File{}, nil, false
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

// Render returns the full screen, including its header, status footer, and
// final blank line. A non-nil error replaces the status while retaining the
// position label. Dimensions are configured through Resize.
func (v *View) Render(err error) string {
	counts := frame.Muted(v.view.countsLabel)
	header := frame.Title(appinfo.Name) + "  " + counts
	body := v.renderBody()
	status := v.renderStatus()
	footer := frame.Muted(status)
	if v.search.repeatMiss {
		message := fmt.Sprintf("no matches for %q", v.search.term)
		footer = frame.Error(message)
	}
	if err != nil {
		footer = frame.Error(frame.SingleLine(err.Error()))
	}
	return frame.Render(header, body, v.renderFooter(footer), v.height)
}
func (v *View) renderBody() []string {
	if len(v.view.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if v.view.patch.Kind == patch.Branch {
			empty = "No branch or worktree changes."
		}
		return frame.EmptyBody(v.height, empty)
	}
	return v.view.renderRows(v.viewport, v.cursor, v.selection)
}

func (v *View) renderStatus() string {
	status := "j/k/h/l move  c comment  ? help  q quit"
	if v.search.active {
		status = "/" + v.search.query + cursorStyle.Render(" ")
		if v.search.miss {
			status += frame.Error("  no matches")
		}
	} else if v.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return status
}

func (v *View) renderFooter(value string) string {
	label := "local changes"
	if v.view.patch.Kind == patch.Branch {
		label = "branch changes from " + v.view.patch.Branch
	}
	if v.viewport.top > 0 {
		label += fmt.Sprintf(" (%d%%)", v.view.viewportProgress(v.viewport))
	}
	right := frame.Muted(label)
	width := frame.ScreenWidth(v.width)
	rightWidth := lipgloss.Width(right)
	value = ansi.Truncate(value, max(0, width-rightWidth-1), "")
	return value + strings.Repeat(
		" ",
		max(1, width-lipgloss.Width(value)-rightWidth),
	) + right
}
