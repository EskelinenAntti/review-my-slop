package ui

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func highlightedSource(path, source, theme string) []string {
	if source == "" {
		return nil
	}
	var output bytes.Buffer
	err := quick.Highlight(
		&output,
		source,
		path,
		"terminal16m",
		theme,
	)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	rendered, err := io.ReadAll(&output)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}

func highlightPair(file patches.File,
	options Options) highlightedSources {
	theme := lightTheme
	if options.Dark {
		theme = darkTheme
	}
	return highlightedSources{
		Old: highlightedSource(file.OldPath, file.OldSource, theme),
		New: highlightedSource(file.NewPath, file.NewSource, theme),
	}
}

func hunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}

// linePair holds hunk line indices; -1 means that side has no line.
type linePair struct {
	left, right int
}

func hunkLinePairs(lines []patches.Line, options Options) []linePair {
	pairs := make([]linePair, 0, len(lines))
	for index := 0; index < len(lines); {
		line := lines[index]
		switch {
		case !options.SideBySide || line.Kind == patches.Context:
			pairs = append(pairs, linePair{left: index, right: index})
			index++
		case line.Kind == patches.Addition:
			pairs = append(pairs, linePair{left: -1, right: index})
			index++
		default:
			block, next := pairedChangeBlock(lines, index)
			pairs = append(pairs, block...)
			index = next
		}
	}
	return pairs
}

// pairedChangeBlock pairs consecutive deletions with the additions that follow.
// A missing line on either side is represented by -1.
func pairedChangeBlock(lines []patches.Line, start int) ([]linePair, int) {
	deletedEnd := start
	for deletedEnd < len(lines) && lines[deletedEnd].Kind == patches.Deletion {
		deletedEnd++
	}
	addedEnd := deletedEnd
	for addedEnd < len(lines) && lines[addedEnd].Kind == patches.Addition {
		addedEnd++
	}
	count := max(deletedEnd-start, addedEnd-deletedEnd)
	pairs := make([]linePair, count)
	for offset := range pairs {
		pairs[offset] = linePair{left: -1, right: -1}
	}
	for offset := range lines[start:deletedEnd] {
		pairs[offset].left = start + offset
	}
	for offset := range lines[deletedEnd:addedEnd] {
		pairs[offset].right = deletedEnd + offset
	}
	return pairs, addedEnd
}

func highlightedLine(line patches.Line,
	highlighted highlightedSources) string {
	lines := highlighted.New
	number := line.NewNumber
	if line.Kind == patches.Deletion {
		lines = highlighted.Old
		number = line.OldNumber
	}
	if number <= 0 || int(number) > len(lines) {
		return line.Text
	}
	return lines[int(number)-1]
}

func projectedLineRow(fileIndex, hunkIndex int, hunk patches.Hunk,
	pair linePair, options Options,
	highlighted highlightedSources) entry {
	row := entry{
		kind: lineRow, file: fileIndex, hunk: hunkIndex,
		leftLine: pair.left, rightLine: pair.right,
	}
	if !options.SideBySide {
		row.text = highlightedLine(
			hunk.Lines[row.rightLine],
			highlighted,
		)
		return row
	}
	if row.leftLine >= 0 {
		row.left = highlightedLine(
			hunk.Lines[row.leftLine],
			highlighted,
		)
	}
	if row.rightLine >= 0 {
		row.right = highlightedLine(
			hunk.Lines[row.rightLine],
			highlighted,
		)
	}
	return row
}

func projectedFileRows(fileIndex int, file patches.File,
	options Options) []entry {
	rows := []entry{{
		kind: fileRow, file: fileIndex, hunk: -1,
		leftLine: -1, rightLine: -1, text: file.DisplayPath,
	}}
	for _, metadata := range file.Metadata {
		rows = append(rows, entry{
			kind: metadataRow, file: fileIndex, hunk: -1,
			leftLine: -1, rightLine: -1, text: metadata,
		})
	}
	highlighted := highlightPair(file, options)
	for hunkIndex, hunk := range file.Hunks {
		rows = append(rows, entry{
			kind: hunkRow, file: fileIndex, hunk: hunkIndex,
			leftLine: -1, rightLine: -1, text: hunkHeader(
				hunk.Header,
			),
		})
		for _, pair := range hunkLinePairs(hunk.Lines, options) {
			rows = append(rows, projectedLineRow(
				fileIndex,
				hunkIndex,
				hunk,
				pair,
				options,
				highlighted,
			))
		}
	}
	return rows
}

func buildDiffView(patch patches.Patch,
	options Options) *diffView {
	view := &diffView{
		patch: patch,
		split: options.SideBySide,
		dark:  options.Dark,
	}
	for fileIndex, file := range patch.Files {
		view.rows = append(view.rows, projectedFileRows(
			fileIndex,
			file,
			options,
		)...)
	}
	return view
}

func cursorIdentityAt(view *diffView,
	cursor diffCursor) cursorIdentity {
	lineIndex := cursorLine(view, cursor)
	if lineIndex < 0 {
		return cursorIdentity{}
	}
	row := view.rows[cursor.row]
	file := view.patch.Files[row.file]
	return cursorIdentity{
		file:   file,
		hunk:   file.Hunks[row.hunk],
		line:   file.Hunks[row.hunk].Lines[lineIndex],
		cursor: cursor,
		valid:  true,
	}
}

func initialViewport(view *diffView, width,
	height int) diffViewport {
	viewport := diffViewport{Width: max(
		1,
		width,
	), Height: max(
		1,
		height,
	)}
	return clampViewport(view, viewport)
}

func preservedViewport(view *diffView,
	viewport diffViewport, leftColumn int) diffViewport {
	maxColumn := maxViewportColumn(view, viewport)
	columns := max(-viewport.LeftColumn, min(
		leftColumn,
		maxColumn-viewport.LeftColumn,
	))
	viewport.LeftColumn += columns
	return clampViewport(view, viewport)
}

func cursorMatchRank(
	candidate, original patches.Line,
) cursorMatchQuality {
	if candidate.Kind != original.Kind {
		return fileCursorMatch
	}
	if candidate.OldNumber == original.OldNumber &&
		candidate.NewNumber == original.NewNumber {
		return numberCursorMatch
	}
	if candidate.Text == original.Text {
		return textCursorMatch
	}
	return fileCursorMatch
}

func absoluteDistance(distance int) int {
	if distance < 0 {
		return -distance
	}
	return distance
}

func cursorMatchDistance(candidate, original patches.Line,
	rowDistance int) int {
	if candidate.NewNumber > 0 && original.NewNumber > 0 {
		return absoluteDistance(int(
			candidate.NewNumber - original.NewNumber,
		))
	}
	if candidate.OldNumber > 0 && original.OldNumber > 0 {
		return absoluteDistance(int(
			candidate.OldNumber - original.OldNumber,
		))
	}
	return absoluteDistance(rowDistance)
}

func betterCursorMatch(candidate,
	best cursorMatch) cursorMatch {
	if candidate.rank > best.rank ||
		candidate.rank == best.rank && candidate.distance < best.distance {
		return candidate
	}
	return best
}

func fileMatchesIdentity(file patches.File, identity patches.File) bool {
	oldPathMatches := file.OldPath != "" && file.OldPath == identity.OldPath
	newPathMatches := file.NewPath != "" && file.NewPath == identity.NewPath
	return oldPathMatches || newPathMatches
}

func cursorMatchAt(view *diffView,
	identity cursorIdentity, row int,
	pane diffPane) (cursorMatch, bool) {
	cursor := diffCursor{row: row, pane: pane}
	lineIndex := cursorLine(view, cursor)
	if lineIndex < 0 {
		return cursorMatch{}, false
	}
	entry := view.rows[row]
	file := view.patch.Files[entry.file]
	line := file.Hunks[entry.hunk].Lines[lineIndex]
	return cursorMatch{
		cursor: cursor,
		rank:   cursorMatchRank(line, identity.line),
		distance: cursorMatchDistance(
			line,
			identity.line,
			row-identity.cursor.row,
		),
	}, true
}

func findCursorMatch(view *diffView,
	identity cursorIdentity) (diffCursor, bool) {
	if !identity.valid {
		return diffCursor{}, false
	}
	best := searchCursorRows(view, identity)
	return best.cursor, best.rank > noCursorMatch
}

func searchCursorRows(view *diffView,
	identity cursorIdentity) cursorMatch {
	best := cursorMatch{}
	for row, entry := range view.rows {
		if !rowMatchesIdentity(view, entry, identity) {
			continue
		}
		candidate := bestCursorMatchAtRow(view, identity, row)
		if candidate.rank == numberCursorMatch {
			return candidate
		}
		best = betterCursorMatch(candidate, best)
	}
	return best
}

func rowMatchesIdentity(view *diffView,
	entry entry, identity cursorIdentity) bool {
	return entry.hunk >= 0 && fileMatchesIdentity(
		view.patch.Files[entry.file], identity.file,
	)
}

func bestCursorMatchAtRow(
	view *diffView,
	identity cursorIdentity,
	row int,
) cursorMatch {
	best := cursorMatch{}
	for _, pane := range searchPanes(view, identity.cursor.pane) {
		candidate, ok := cursorMatchAt(view, identity, row, pane)
		if !ok {
			continue
		}
		if candidate.rank == numberCursorMatch {
			return candidate
		}
		best = betterCursorMatch(candidate, best)
	}
	return best
}

func translatedSelection(next, old *diffView,
	screen *Diff) *diffSelection {
	selection := screen.selection
	if selection == nil {
		return nil
	}
	firstIdentity := cursorIdentityAt(old, selection.First)
	lastIdentity := cursorIdentityAt(old, selection.Last)
	if !firstIdentity.valid || !lastIdentity.valid ||
		!fileMatchesIdentity(firstIdentity.file, lastIdentity.file) ||
		firstIdentity.hunk.Header != lastIdentity.hunk.Header {
		return nil
	}
	first, firstOK := findCursorMatch(next, firstIdentity)
	last, lastOK := findCursorMatch(next, lastIdentity)
	if !firstOK || !lastOK {
		return nil
	}
	firstRow := next.rows[first.row]
	lastRow := next.rows[last.row]
	if firstRow.file != lastRow.file || firstRow.hunk != lastRow.hunk {
		return nil
	}
	return &diffSelection{First: first, Last: last}
}

func preserveSearch(screen *Diff, next *diffView,
	origin cursorIdentity) {
	if !screen.search.active {
		return
	}
	screen.search.from = screen.cursor
	if translated, ok := findCursorMatch(next, origin); ok {
		screen.search.from = translated
	}
	previewSearch(screen, screen.search.query)
}

// NewDiff creates a diff view with default terminal dimensions.
func NewDiff(
	p patches.Patch,
	options Options,
) *Diff {
	v := &Diff{options: options, width: 80, height: 30}
	v.view = buildDiffView(p, Options{
		SideBySide: v.options.SideBySide && v.width >= 100,
		Dark:       v.options.Dark,
	})
	v.viewport = initialViewport(
		v.view,
		v.width,
		max(1, v.height-3),
	)
	v.cursor, _ = firstCursor(v.view)
	return v
}
