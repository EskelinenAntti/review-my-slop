// Package diffscreen presents patches and owns their interactive browsing
// state.
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

// NewChangesView creates a view with default dimensions of 80 columns and
// 30 rows.
func NewChangesView(p patch.Patch, options Options) *View {
	v := &View{options: options, width: 80, height: 30}
	v.view = v.createChangesView(p)
	v.viewport = v.view.newVisibleArea(v.width, frame.ContentHeight(v.height))
	v.cursor, _ = v.view.firstSelectableLine()
	return v
}

// ReplaceChanges replaces the patch while preserving focus, selection, and
// relative screen position. Missing targets fall back to nearby available code.
func (v *View) ReplaceChanges(p patch.Patch) {
	v.EndDrag()
	v.search.repeatMiss = false
	v.replaceViewPreservingPosition(v.createChangesView(p))
}

// SetOptions changes presentation options while preserving the current focus.
func (v *View) SetOptions(options Options) {
	active := v.sideBySideActive()
	dark := v.options.Dark
	v.options = options
	if active != v.sideBySideActive() || dark != options.Dark {
		v.ReplaceChanges(v.view.patch)
	}
}

// Resize accepts full screen dimensions, reserving space for the header/footer.
func (v *View) Resize(width, height int) {
	v.EndDrag()
	active := v.sideBySideActive()
	v.width, v.height = width, height
	v.viewport = v.view.resize(v.viewport, width, frame.ContentHeight(height))
	if active != v.sideBySideActive() {
		v.ReplaceChanges(v.view.patch)
	} else {
		v.viewport = v.view.keepFocusedLineVisible(v.viewport, v.cursor)
	}
}

func (v *View) sideBySideActive() bool {
	return v.options.SideBySide && v.width >= 100
}
func (v *View) createChangesView(p patch.Patch) *diffView {
	return createChangesView(
		p,
		Options{SideBySide: v.sideBySideActive(), Dark: v.options.Dark},
	)
}

// MoveFocusWithinChanges moves among lines, files, and panes while keeping the
// focused line visible. Line and page movement extend selection only within
// its original hunk.
func (v *View) MoveFocusWithinChanges(motion Motion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case PreviousLine:
		v.moveFocusOneLine(backward)
	case NextLine:
		v.moveFocusOneLine(forward)
	case PreviousPage:
		v.moveFocusHalfPage(backward)
	case NextPage:
		v.moveFocusHalfPage(forward)
	case FirstLine:
		if cursor, ok := v.view.firstSelectableLine(); ok {
			v.setFocusedLine(cursor)
		}
	case LastLine:
		if cursor, ok := v.view.lastSelectableLine(); ok {
			v.setFocusedLine(cursor)
		}

	default:
		v.moveBetweenFilesAndPanes(motion)
	}
}

func (v *View) moveBetweenFilesAndPanes(motion Motion) {
	switch motion {
	case PreviousFile:
		v.moveFocusToAdjacentFile(backward)
	case NextFile:
		v.moveFocusToAdjacentFile(forward)
	case OldPane:
		v.switchToPane(left)
	case NewPane:
		v.switchToPane(right)
	case OtherPane:
		v.switchToPane(v.cursor.pane.oppositePane())
	default:
		return
	}
}

func (v *View) moveFocusOneLine(direction direction) {
	next, ok := v.view.moveInDirection(v.cursor, direction)
	if !ok {
		return
	}
	if !v.extendSelectionThroughLine(next) {
		return
	}
	v.setFocusedLine(next)
}
func (v *View) setFocusedLine(cursor diffCursor) {
	v.cursor = cursor
	v.viewport = v.view.keepFocusedLineVisible(v.viewport, cursor)
}
func (v *View) moveFocusHalfPage(direction direction) {
	viewport, cursor := v.view.moveFocusedLineHalfPage(
		v.viewport,
		v.cursor,
		direction,
	)
	if !v.extendSelectionThroughLine(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

// extendSelectionThroughLine accepts movement when no range is active.
// An active range changes only when its endpoint remains in the original hunk.
func (v *View) extendSelectionThroughLine(cursor diffCursor) bool {
	if v.selection == nil {
		return true
	}
	selection, ok := v.view.extendSelection(*v.selection, cursor)
	if ok {
		v.selection = &selection
	}
	return ok
}
func (v *View) moveFocusToAdjacentFile(direction direction) {
	v.ClearSelection()
	if cursor, ok := v.view.moveFocusToAdjacentFile(v.cursor, direction); ok {
		v.setFocusedLine(cursor)
	}
}
func (v *View) switchToPane(pane diffPane) {
	cursor, ok := v.view.switchToPane(v.cursor, pane)
	if !ok {
		return
	}
	if v.selection != nil {
		first, firstOK := v.view.switchToPane(v.selection.First, pane)
		last, lastOK := v.view.switchToPane(v.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := beginSelection(first)
		selection, ok = v.view.extendSelection(selection, last)
		if !ok {
			return
		}
		v.selection = &selection
	}
	v.setFocusedLine(cursor)
}

// PositionFocusedLine places the focused line at the requested screen position.
func (v *View) PositionFocusedLine(alignment Alignment) {
	v.search.repeatMiss = false
	if _, ok := v.view.patchLineForPosition(v.cursor); !ok {
		return
	}
	v.viewport = v.view.positionFocusedLine(
		v.viewport,
		v.cursor,
		verticalAlignment(alignment),
	)
}

// ScrollHorizontal shifts the visible content by the requested columns.
func (v *View) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = v.view.scrollVisibleAreaSideways(v.viewport, columns)
}

// ScrollVertical moves content first, moving focus only when it leaves the
// visible code. Like keyboard movement, it extends selection within its hunk.
func (v *View) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	viewport, cursor := v.view.scrollVisibleAreaUpDown(
		v.viewport,
		v.cursor,
		rows,
	)
	if cursor != v.cursor && !v.extendSelectionThroughLine(cursor) {
		return
	}
	v.viewport, v.cursor = viewport, cursor
}

// ToggleSelection starts a range at the focused line or clears the current
// range.
func (v *View) ToggleSelection() {
	v.search.repeatMiss = false
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	if _, ok := v.view.patchLineForPosition(v.cursor); ok {
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

// SelectedFileAndLines returns the focused line when no range is selected.
// The bool is false when the patch has no selectable code.
func (v *View) SelectedFileAndLines() (patch.File, []patch.Line, bool) {
	selection := v.selection
	if selection == nil {
		current := beginSelection(v.cursor)
		selection = &current
	}
	file, ok := v.view.fileForLine(selection.First)
	lines := v.view.selectedLines(*selection)
	return file, lines, ok && len(lines) > 0
}

// FocusedFileAndLine returns the focused source line, regardless of selection.
func (v *View) FocusedFileAndLine() (patch.File, patch.Line, bool) {
	file, fileOK := v.view.fileForLine(v.cursor)
	line, lineOK := v.view.patchLineForPosition(v.cursor)
	return file, line, fileOK && lineOK
}

// BeginSearch starts an empty search query at the current focus.
func (v *View) BeginSearch() {
	v.ClearSelection()
	v.search.active = true
	v.search.query, v.search.miss, v.search.repeatMiss = "", false, false
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
		v.setFocusedLine(v.search.from)
		v.search.miss = false
		return
	}
	cursor, ok := v.view.findTextInChanges(query, v.search.from, forward)
	v.search.miss = !ok
	if ok {
		v.setFocusedLine(cursor)
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
	v.setFocusedLine(v.search.from)
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// FindNextMatch moves to the next remembered match in the requested direction.
func (v *View) FindNextMatch(d Direction) bool {
	if v.search.term == "" {
		return false
	}
	if d != Forward && d != Backward {
		return false
	}
	cursor, ok := v.view.findTextInChanges(
		v.search.term,
		v.cursor,
		direction(d),
	)
	v.search.repeatMiss = !ok
	if ok {
		v.setFocusedLine(cursor)
	}
	return ok
}

// FormatReviewScreen returns the complete screen, including its header and
// footer. A non-nil error replaces the status while retaining the
// position label. Dimensions are configured through Resize.
func (v *View) FormatReviewScreen(err error) string {
	added, removed := countAddedAndRemovedLines(v.view.patch)
	counts := mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	header := titleStyle.Render("review-my-slop") + "  " + counts
	body := v.formatBodyRows()
	status := v.formatStatusLine()
	footer := mutedStyle.Render(status)
	if v.search.repeatMiss {
		message := fmt.Sprintf("no matches for %q", v.search.term)
		footer = errorStyle.Render(message)
	}
	if err != nil {
		footer = errorStyle.Render(strings.ReplaceAll(err.Error(), "\n", " "))
	}
	return frame.ComposeScreen(
		header,
		body,
		v.formatFooterText(footer),
		v.height,
	)
}
func (v *View) formatBodyRows() []string {
	var body []string
	if len(v.view.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if v.view.patch.Kind == patch.Branch {
			empty = "No branch or worktree changes."
		}
		body = make([]string, frame.ContentHeight(v.height))
		body[min(1, len(body)-1)] = mutedStyle.Render(empty)
	} else {
		body = strings.Split(
			v.view.formatVisibleChanges(v.viewport, v.cursor, v.selection),
			"\n",
		)
	}
	return body
}

func (v *View) formatStatusLine() string {
	status := "j/k/h/l move  c comment  ? help  q quit"
	if v.search.active {
		status = "/" + v.search.query + cursorStyle.Render(" ")
		if v.search.miss {
			status += errorStyle.Render("  no matches")
		}
	} else if v.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return status
}

func (v *View) formatFooterText(value string) string {
	label := "local changes"
	if v.view.patch.Kind == patch.Branch {
		label = "branch changes from " + v.view.patch.Branch
	}
	if v.viewport.top > 0 {
		label += fmt.Sprintf(
			" (%d%%)",
			v.view.visibleContentProgressPercent(v.viewport),
		)
	}
	right := mutedStyle.Render(label)
	width := max(20, v.width)
	value = ansi.Truncate(value, max(0, width-lipgloss.Width(right)-1), "")
	return value + strings.Repeat(
		" ",
		max(1, width-lipgloss.Width(value)-lipgloss.Width(right)),
	) + right
}
func countAddedAndRemovedLines(p patch.Patch) (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			counts := countLinesInChangeBlock(hunk)
			added += counts[0]
			removed += counts[1]
		}
	}
	return added, removed
}

func countLinesInChangeBlock(hunk patch.Hunk) [2]int {
	counts := [2]int{}
	for _, line := range hunk.Lines {
		switch line.Kind {
		case patch.Addition:
			counts[0]++
		case patch.Deletion:
			counts[1]++
		default:
			continue
		}
	}
	return counts
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
