// Package diffscreen presents patches and owns their interactive browsing state.
package diffscreen

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

const (
	Top Alignment = iota
	Center
	Bottom
)

// Direction chooses the order in which search matches are visited.
type Direction int8

const (
	Backward Direction = -1
	Forward  Direction = 1
)

// View owns focus, selection, search, and scrolling for a patch presentation.
// It has no knowledge of keys, editors, or comment persistence.
type View struct {
	patch         patch.Patch
	view          *diffView
	cursor        diffCursor
	viewport      diffViewport
	selection     *diffSelection
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

// New creates a view with default dimensions of 80 columns and 30 rows.
func New(p patch.Patch, options Options) *View {
	v := &View{patch: p, options: options, width: 80, height: 30}
	v.view = v.newReviewView(p)
	v.viewport = v.view.newViewport(v.width, frame.BodyHeight(v.height))
	v.cursor, _ = v.view.first()
	return v
}

// Update replaces the patch while preserving meaningful focus, selection, and
// relative screen position. Missing targets fall back to nearby available code.
func (v *View) Update(p patch.Patch) {
	v.search.repeatMiss = false
	old := v.view
	state := viewState{cursor: &v.cursor, selection: v.selection, viewport: v.viewport}
	origin := identify(old, &v.search.from)
	v.patch = p
	v.view = v.newReviewView(p)
	preserved := preserve(old, state, v.view)
	v.viewport, v.selection = preserved.viewport, preserved.selection
	v.cursor = diffCursor{}
	if preserved.cursor != nil {
		v.cursor = *preserved.cursor
	}
	if v.search.active {
		v.search.from = v.cursor
		if origin.valid {
			if translated, ok := v.view.findCursor(origin.file, origin.hunk, origin.line, origin.cursor.coordinate, origin.cursor.pane); ok {
				v.search.from = translated
			}
		}
		v.PreviewSearch(v.search.query)
	}
}

// Configure changes presentation options while preserving meaningful position.
func (v *View) Configure(options Options) {
	active := v.sideBySideActive()
	dark := v.options.Dark
	v.options = options
	if active != v.sideBySideActive() || dark != options.Dark {
		v.Update(v.patch)
	}
}

// Resize accepts full screen dimensions, reserving space for the header/footer.
func (v *View) Resize(width, height int) {
	active := v.sideBySideActive()
	v.width, v.height = width, height
	v.viewport = v.view.resize(v.viewport, width, frame.BodyHeight(height))
	if active != v.sideBySideActive() {
		v.Update(v.patch)
	} else {
		v.viewport = v.view.keepVisible(v.viewport, v.cursor)
	}
}

func (v *View) sideBySideActive() bool { return v.options.SideBySide && v.width >= 100 }
func (v *View) newReviewView(p patch.Patch) *diffView {
	if v.sideBySideActive() {
		return newSideBySideView(p, v.options.Dark)
	}
	return newUnifiedView(p, v.options.Dark)
}

// Move performs navigation and keeps the resulting focus visible. Line and
// page movement extend active selection only within its original hunk.
func (v *View) Move(motion Motion) {
	v.search.repeatMiss = false
	switch motion {
	case PreviousLine:
		v.move(backward)
	case NextLine:
		v.move(forward)
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
	}
}

func (v *View) move(direction direction) {
	next, ok := v.view.move(v.cursor, direction)
	if !ok {
		return
	}
	if v.selection != nil {
		selection, ok := v.view.extendSelection(*v.selection, next)
		if !ok {
			return
		}
		v.selection = &selection
	}
	v.setCursor(next)
}
func (v *View) setCursor(cursor diffCursor) {
	v.cursor = cursor
	v.viewport = v.view.keepVisible(v.viewport, cursor)
}
func (v *View) halfPage(direction direction) {
	viewport, cursor := v.view.scrollHalfPage(v.viewport, v.cursor, direction)
	if v.selection != nil {
		selection, ok := v.view.extendSelection(*v.selection, cursor)
		if !ok {
			return
		}
		v.selection = &selection
	}
	v.viewport, v.cursor = viewport, cursor
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

func (v *View) Align(alignment Alignment) {
	v.search.repeatMiss = false
	if _, ok := v.view.line(v.cursor); !ok {
		return
	}
	v.viewport = v.view.align(v.viewport, v.cursor, verticalAlignment(alignment))
}
func (v *View) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = v.view.scrollHorizontal(v.viewport, columns)
}
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
func (v *View) ClearSelection() { v.selection = nil; v.search.repeatMiss = false }

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

func (v *View) BeginSearch() {
	v.ClearSelection()
	v.search.active = true
	v.search.query, v.search.miss, v.search.repeatMiss = "", false, false
	v.search.from = v.cursor
}

// PreviewSearch searches from the session origin, starting a session if needed.
// An empty query restores its origin; a miss retains the last successful focus.
func (v *View) PreviewSearch(query string) bool {
	if !v.search.active {
		v.BeginSearch()
	}
	v.search.query = query
	if query == "" {
		v.setCursor(v.search.from)
		v.search.miss = false
		return true
	}
	cursor, ok := v.view.search(query, v.search.from, forward)
	v.search.miss = !ok
	if ok {
		v.setCursor(cursor)
	}
	return ok
}
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
func (v *View) CancelSearch() {
	if !v.search.active {
		return
	}
	v.setCursor(v.search.from)
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}
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

// Render returns the full screen, including its header, status footer, and
// final blank line. Dimensions are configured through Resize.
func (v *View) Render() string {
	added, removed := patchLineCounts(v.patch)
	header := titleStyle.Render("review-my-slop") + "  " + mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	var body []string
	if len(v.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if v.patch.Kind == patch.Branch {
			empty = "No branch or worktree changes."
		}
		body = make([]string, frame.BodyHeight(v.height))
		body[min(1, len(body)-1)] = mutedStyle.Render(empty)
	} else {
		body = strings.Split(v.view.render(v.viewport, v.cursor, v.selection), "\n")
	}
	status := "j/k/h/l move  c comment  ? help  q quit"
	if v.search.active {
		status = "/" + v.search.query + cursorStyle.Render(" ")
		if v.search.miss {
			status += errorStyle.Render("  no matches")
		}
	} else if v.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	footer := mutedStyle.Render(status)
	if v.search.repeatMiss {
		footer = errorStyle.Render(fmt.Sprintf("no matches for %q", v.search.term))
	}
	return frame.Render(header, body, v.renderFooter(footer), v.height)
}
func (v *View) renderFooter(value string) string {
	label := "local changes"
	if v.patch.Kind == patch.Branch {
		label = "branch changes from " + v.patch.Branch
	}
	if v.viewport.top.Y > 0 {
		label += fmt.Sprintf(" (%d%%)", v.view.viewportProgress(v.viewport))
	}
	right := mutedStyle.Render(label)
	width := max(20, v.width)
	value = ansi.Truncate(value, max(0, width-lipgloss.Width(right)-1), "")
	return value + strings.Repeat(" ", max(1, width-lipgloss.Width(value)-lipgloss.Width(right))) + right
}
func patchLineCounts(p patch.Patch) (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind == patch.Addition {
					added++
				}
				if line.Kind == patch.Deletion {
					removed++
				}
			}
		}
	}
	return
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
