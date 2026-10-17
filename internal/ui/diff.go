package ui

import (
	"fmt"
	"regexp"

	"charm.land/lipgloss/v2"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

type cursorIdentity struct {
	file   patches.File
	hunk   patches.Hunk
	line   patches.Line
	cursor diffCursor
	valid  bool
}

// BeginDrag focuses a code line at terminal coordinates and starts a drag.
func (v *Diff) BeginDrag(x, y int) bool {
	v.EndDrag()
	cursor, ok := pointerCursor(
		v.view,
		v.viewport,
		x,
		y,
		v.width,
		v.height,
	)
	if !ok {
		return false
	}
	v.ClearSelection()
	v.cursor = cursor
	v.drag = &cursor
	return true
}

// DragTo extends the active drag selection to terminal coordinates.
func (v *Diff) DragTo(x, y int) {
	if v.drag == nil {
		return
	}
	cursor, ok := pointerCursor(
		v.view,
		v.viewport,
		x,
		y,
		v.width,
		v.height,
	)
	if !ok || cursor.pane != v.drag.pane {
		return
	}
	if cursor == *v.drag && v.selection == nil {
		return
	}
	selection, ok := selectionForDrag(v.view, *v.drag, cursor)
	if !ok {
		return
	}
	v.cursor, v.selection = cursor, &selection
}

// EndDrag ends pointer dragging without clearing the selection.
func (v *Diff) EndDrag() { v.drag = nil }

// Options controls diff layout and syntax colors.
type Options struct {
	SideBySide bool
	Dark       bool
}

// Motion identifies a line, page, file, or pane navigation operation.
type Motion uint8

// Navigation motions move through source lines, pages, files, and panes.
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

// Alignment positions the current line within the viewport.
type Alignment uint8

// Viewport alignments position the current line.
const (
	Top Alignment = iota
	Center
	Bottom
)

// Direction specifies the order in which search visits matches.
type Direction int8

// Search directions visit earlier or later matches.
const (
	Backward Direction = -1
	Forward  Direction = 1
)

// Diff owns diff projection, navigation, selection, search, and rendering.
// Create a Diff with NewDiff before using its methods.
type Diff struct {
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

// Update replaces the patch while preserving cursor, selection, and search.
func (v *Diff) Update(p patches.Patch) {
	v.EndDrag()
	v.search.repeatMiss = false
	next := buildDiffView(p, Options{
		SideBySide: v.options.SideBySide && v.width >= 100,
		Dark:       v.options.Dark,
	})
	old := v.view
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := cursorIdentityAt(old, v.cursor)
	origin := cursorIdentityAt(old, v.search.from)
	v.viewport = initialViewport(
		next,
		v.viewport.Width,
		v.viewport.Height,
	)
	v.viewport = preservedViewport(next, v.viewport, leftColumn)
	translated, ok := findCursorMatch(next, cursor)
	if !ok {
		translated, _ = firstCursor(next)
	}
	v.cursor = translated
	v.selection = translatedSelection(next, old, v)
	v.view = next
	if cursorLine(next, v.cursor) >= 0 {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = cursorViewport(next, v.viewport, v.cursor)
	}

	preserveSearch(v, next, origin)
}

// Configure applies layout and color options while preserving navigation.
func (v *Diff) Configure(options Options) {
	active := v.options.SideBySide && v.width >= 100
	dark := v.options.Dark
	v.options = options
	if active != (v.options.SideBySide && v.width >= 100) ||
		dark != options.Dark {
		v.Update(v.view.patch)
	}
}

// Resize sets terminal dimensions and keeps the current line visible.
func (v *Diff) Resize(width, height int) {
	v.EndDrag()
	wasActive := v.options.SideBySide && v.width >= 100
	v.width, v.height = width, height
	v.viewport.Width = max(1, width)
	v.viewport.Height = max(1, height-3)
	v.viewport = clampViewport(v.view, v.viewport)
	if wasActive != (v.options.SideBySide && v.width >= 100) {
		v.Update(v.view.patch)
		return
	}
	v.viewport = cursorViewport(v.view, v.viewport, v.cursor)
}

// Move navigates the diff using the requested motion.
func (v *Diff) Move(motion Motion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case PreviousLine:
		moveLine(v, scanBackward)
	case NextLine:
		moveLine(v, scanForward)
	case PreviousPage:
		movePage(v, scanBackward)
	case NextPage:
		movePage(v, scanForward)
	case FirstLine:
		if cursor, ok := firstCursor(v.view); ok {
			v.cursor = cursor
			v.viewport = cursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
		}
	case LastLine:
		if cursor, ok := lastCursor(v.view); ok {
			v.cursor = cursor
			v.viewport = cursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
		}
	default:
		moveFileOrPane(v, motion)
	}
}

// Align positions the current line at the requested viewport alignment.
func (v *Diff) Align(alignment Alignment) {
	v.search.repeatMiss = false
	if cursorLine(v.view, v.cursor) < 0 {
		return
	}
	viewport := v.viewport
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	requestedOffset := alignmentOffset(alignment, viewport.Height)
	viewport.top = v.cursor.row - max(0, requestedOffset-headerHeight)
	if !viewportHasFileHeader(v.view, viewport) {
		viewport.top = v.cursor.row - requestedOffset
	}
	v.viewport = clampViewport(v.view, viewport)
}

// ScrollHorizontal shifts visible content by columns, clamping to its bounds.
func (v *Diff) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	maxColumn := viewportLeft(v.view, v.viewport)
	columns = max(-v.viewport.LeftColumn, min(
		columns,
		maxColumn-v.viewport.LeftColumn,
	))
	v.viewport.LeftColumn += columns
	v.viewport = clampViewport(v.view, v.viewport)
}

// ScrollVertical scrolls by rows while keeping a valid cursor and selection.
func (v *Diff) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	initialViewport := v.viewport
	maxTop := viewportBottom(v.view, v.viewport.Height)
	rows = max(-v.viewport.top, min(rows, maxTop-v.viewport.top))
	v.viewport.top += rows
	v.viewport = clampViewport(v.view, v.viewport)
	if cursorVisible(v.view, v.viewport, v.cursor) {
		return
	}
	height := viewportHeight(v.view, v.viewport)
	target := max(v.viewport.top, min(v.cursor.row, v.viewport.top+height-1))
	direction := scrollDirection(v.cursor.row, target)
	candidate, ok := nearestScrollCursor(
		v.view, v.viewport, v.cursor, target, direction,
	)
	if !ok {
		return
	}
	selection, valid := scrollSelection(
		v.view,
		v.selection,
		candidate,
	)
	if !valid {
		v.viewport = initialViewport
		return
	}
	v.selection = selection
	v.cursor = candidate
}

// ToggleSelection starts a range at the current line or clears an active range.
func (v *Diff) ToggleSelection() {
	v.search.repeatMiss = false
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	if _, _, ok := patchLineAtCursor(v.view, v.cursor); !ok {
		return
	}
	selection := diffSelection{First: v.cursor, Last: v.cursor}
	v.selection = &selection
}

// ClearSelection clears the selected range and ends pointer dragging.
func (v *Diff) ClearSelection() {
	v.EndDrag()
	v.selection = nil
	v.search.repeatMiss = false
}

// Selected returns the selected source range, or the current line if no range
// exists.
func (v *Diff) Selected() (patches.File, []patches.Line, bool) {
	selection := diffSelection{First: v.cursor, Last: v.cursor}
	if v.selection != nil {
		selection = *v.selection
	}
	file, valid := selectionFile(v.view, selection)
	if !valid {
		return file, nil, false
	}
	lines := linesInSelection(v.view, selection)
	return file, lines, len(lines) > 0
}

// Current returns the source file and line under the cursor, when available.
func (v *Diff) Current() (patches.File, patches.Line, bool) {
	return patchLineAtCursor(v.view, v.cursor)
}

// BeginSearch starts incremental search and remembers the current position.
func (v *Diff) BeginSearch() {
	v.ClearSelection()
	v.search.active = true
	v.search.query, v.search.miss, v.search.repeatMiss = "", false, false
	v.search.from = v.cursor
}

// InsertSearch appends text to the incremental search query.
func (v *Diff) InsertSearch(text string) {
	if !v.search.active {
		v.BeginSearch()
	}
	previewSearch(v, v.search.query+text)
}

// BackspaceSearch removes the last rune from the incremental search query.
func (v *Diff) BackspaceSearch() {
	if !v.search.active {
		return
	}
	query := []rune(v.search.query)
	if len(query) > 0 {
		query = query[:len(query)-1]
	}
	previewSearch(v, string(query))
}

// AcceptSearch saves a successful query for repeated searches.
func (v *Diff) AcceptSearch() {
	if !v.search.active {
		return
	}
	if v.search.query != "" && !v.search.miss {
		v.search.term = v.search.query
	}
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// CancelSearch exits incremental search and restores its starting position.
func (v *Diff) CancelSearch() {
	if !v.search.active {
		return
	}
	v.cursor = v.search.from
	v.viewport = cursorViewport(v.view, v.viewport, v.cursor)
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

// Find visits the next match of the accepted search in the given direction.
func (v *Diff) Find(d Direction) bool {
	if v.search.term == "" {
		return false
	}
	if d != Forward && d != Backward {
		return false
	}
	cursor, ok := findSearchCursorDirection(
		v.view, v.search.term, v.cursor, scanDirection(d),
	)
	v.search.repeatMiss = !ok
	if ok {
		v.cursor = cursor
		v.viewport = cursorVisibleViewport(cursor, v.view, v.viewport)
	}
	return ok
}

// Render draws the diff and an optional error in its footer.
func (v *Diff) Render(err error) string {
	added, removed := patchChangeCounts(v.view.patch)
	counts := mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	header := titleStyle.Render("review-my-slop") + "  " + counts
	body := renderBody(v)
	footer := renderFooter(v, err)
	return renderFrame(header, body, footer, v.height)
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Red).
			Bold(true)
)

type foregroundPolicy uint8

const (
	preserveForeground foregroundPolicy = iota
	discardForeground
)

var (
	ansiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	fileStyle      = lipgloss.NewStyle().Bold(true)
	metadataStyle  = lipgloss.NewStyle().Faint(true)
	hunkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	contextStyle   = lipgloss.NewStyle()
	addedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Green)
	removedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Red)
	cursorStyle    = lipgloss.NewStyle().Reverse(true)
)

type cursorMatch struct {
	cursor         diffCursor
	rank, distance int
}

type diffPane uint8

const (
	left diffPane = iota
	right
)

type diffCursor struct {
	row  int
	pane diffPane
}

type diffViewport struct {
	top        int
	LeftColumn int
	Width      int
	Height     int
}

type diffSelection struct {
	First diffCursor
	Last  diffCursor
}

type scanDirection int8

const (
	scanBackward scanDirection = -1
	scanForward  scanDirection = 1
)

type verticalAlignment uint8

const (
	alignTop verticalAlignment = iota
	middle
	alignBottom
)

type rowKind uint8

const (
	fileRow rowKind = iota
	metadataRow
	hunkRow
	lineRow
)

type entry struct {
	kind              rowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type diffView struct {
	patch patches.Patch
	rows  []entry
	split bool
	dark  bool
}

type pair struct {
	Old []string
	New []string
}

const (
	lightTheme = "catppuccin-latte"
	darkTheme  = "catppuccin-mocha"
)
