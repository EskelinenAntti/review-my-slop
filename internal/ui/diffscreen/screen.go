// Package diffscreen presents patches and owns their interactive browsing
// state.
package diffscreen

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"
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

// New creates a view with default dimensions of 80 columns and 30 rows.
func New(p patch.Patch, options Options) *View {
	v := &View{options: options, width: 80, height: 30}
	v.view = func() *diffView {
		{
			var options Options = Options{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
			v := &diffView{patch: p, split: options.SideBySide, dark: options.Dark}
			for fileIndex, file := range p.Files {
				var file patch.File = file
				v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
				for _, metadata := range file.Metadata {
					v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
				}
				highlighted := func() pair {
					var file *patch.File = &file
					theme := lightTheme
					if v.dark {
						theme = darkTheme
					}
					return pair{Old: func() []string {
						var filename, source, theme string = file.OldPath, file.OldSource, theme
						if source == "" {
							return nil
						}
						var buf bytes.Buffer
						highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
						if highlightErr != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						rendered, err := io.ReadAll(&buf)
						if err != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
					}(), New: func() []string {
						var filename, source, theme string = file.NewPath, file.NewSource, theme
						if source == "" {
							return nil
						}
						var buf bytes.Buffer
						highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
						if highlightErr != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						rendered, err := io.ReadAll(&buf)
						if err != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
					}()}
				}()
				for hunkIndex, hunk := range file.Hunks {
					v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
						if strings.HasPrefix(hunk.Header, "@@") {
							return hunk.Header
						}
						return "@@ " + hunk.Header
					}()})
					for _, indices := range func() [][2]int {
						rows := make([][2]int, 0, len(hunk.Lines))
						for index := 0; index < len(hunk.Lines); {
							if !v.split || hunk.Lines[index].Kind == patch.Context {
								rows = append(rows, [2]int{index, index})
								index++
								continue
							}
							if hunk.Lines[index].Kind == patch.Addition {
								rows = append(rows, [2]int{-1, index})
								index++
								continue
							}
							addedStart := func() int {
								var start int = index
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patch.Deletion {
									start++
								}
								return start
							}()
							end := func() int {
								var start int = addedStart
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patch.Addition {
									start++
								}
								return start
							}()
							rows = append(rows, func() [][2]int {
								count := max(addedStart-index, end-addedStart)
								rows := make([][2]int, 0, count)
								for offset := 0; offset < count; offset++ {
									inlineLocalPair2 := [2]int{-1, -1}
									if index+offset < addedStart {
										inlineLocalPair2[0] = index + offset
									}
									if addedStart+offset < end {
										inlineLocalPair2[1] = addedStart + offset
									}
									rows = append(rows, inlineLocalPair2)
								}
								return rows
							}()...)
							index = end
						}
						return rows
					}() {
						row := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
						func() {
							var row *entry = &row
							if !v.split {
								row.text = func() string {
									var line patch.Line = hunk.Lines[row.rightLine]
									if line.Kind == patch.Deletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
								return
							}
							if row.leftLine >= 0 {
								row.left = func() string {
									var line patch.Line = hunk.Lines[row.leftLine]
									if line.Kind == patch.Deletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
							}
							if row.rightLine >= 0 {
								row.right = func() string {
									var line patch.Line = hunk.Lines[row.rightLine]
									if line.Kind == patch.Deletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
							}
						}()
						v.rows = append(v.rows, row)
					}
				}
			}
			return v
		}
	}()
	v.viewport = func(v *diffView, width, height int) diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, width), max(1, height)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}(v.view, v.width, frame.BodyHeight(v.height))
	v.cursor, _ = func() (diffCursor, bool) {
		var v *diffView = v.view
		if cursor, ok := func() (diffCursor, bool) {
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: right}
					return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return cursor, true
		}
		step := int(forward)
		for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: left}
				return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0
			}(); ok {
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	return v
}

// Update replaces the patch while preserving meaningful focus, selection, and
// relative screen position. Missing targets fall back to nearby available code.
func (v *View) Update(p patch.Patch) {
	v.EndDrag()
	v.search.repeatMiss = false
	var next *diffView = func() *diffView {
		{
			var options Options = Options{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
			v := &diffView{patch: p, split: options.SideBySide, dark: options.Dark}
			for fileIndex, file := range p.Files {
				var file patch.File = file
				v.rows = append(v.rows, entry{kind: fileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
				for _, metadata := range file.Metadata {
					v.rows = append(v.rows, entry{kind: metadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
				}
				highlighted := func() pair {
					var file *patch.File = &file
					theme := lightTheme
					if v.dark {
						theme = darkTheme
					}
					return pair{Old: func() []string {
						var filename, source, theme string = file.OldPath, file.OldSource, theme
						if source == "" {
							return nil
						}
						var buf bytes.Buffer
						highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
						if highlightErr != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						rendered, err := io.ReadAll(&buf)
						if err != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
					}(), New: func() []string {
						var filename, source, theme string = file.NewPath, file.NewSource, theme
						if source == "" {
							return nil
						}
						var buf bytes.Buffer
						highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
						if highlightErr != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						rendered, err := io.ReadAll(&buf)
						if err != nil {
							return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
						}
						return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
					}()}
				}()
				for hunkIndex, hunk := range file.Hunks {
					v.rows = append(v.rows, entry{kind: hunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
						if strings.HasPrefix(hunk.Header, "@@") {
							return hunk.Header
						}
						return "@@ " + hunk.Header
					}()})
					for _, indices := range func() [][2]int {
						rows := make([][2]int, 0, len(hunk.Lines))
						for index := 0; index < len(hunk.Lines); {
							if !v.split || hunk.Lines[index].Kind == patch.Context {
								rows = append(rows, [2]int{index, index})
								index++
								continue
							}
							if hunk.Lines[index].Kind == patch.Addition {
								rows = append(rows, [2]int{-1, index})
								index++
								continue
							}
							addedStart := func() int {
								var start int = index
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patch.Deletion {
									start++
								}
								return start
							}()
							end := func() int {
								var start int = addedStart
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patch.Addition {
									start++
								}
								return start
							}()
							rows = append(rows, func() [][2]int {
								count := max(addedStart-index, end-addedStart)
								rows := make([][2]int, 0, count)
								for offset := 0; offset < count; offset++ {
									inlineLocalPair1 := [2]int{-1, -1}
									if index+offset < addedStart {
										inlineLocalPair1[0] = index + offset
									}
									if addedStart+offset < end {
										inlineLocalPair1[1] = addedStart + offset
									}
									rows = append(rows, inlineLocalPair1)
								}
								return rows
							}()...)
							index = end
						}
						return rows
					}() {
						row := entry{kind: lineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
						func() {
							var row *entry = &row
							if !v.split {
								row.text = func() string {
									var line patch.Line = hunk.Lines[row.rightLine]
									if line.Kind == patch.Deletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
								return
							}
							if row.leftLine >= 0 {
								row.left = func() string {
									var line patch.Line = hunk.Lines[row.leftLine]
									if line.Kind == patch.Deletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
							}
							if row.rightLine >= 0 {
								row.right = func() string {
									var line patch.Line = hunk.Lines[row.rightLine]
									if line.Kind == patch.Deletion {
										if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
											return line.Text
										}
										return highlighted.Old[int(line.OldNumber)-1]
									}
									if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
										return line.Text
									}
									return highlighted.New[int(line.NewNumber)-1]
								}()
							}
						}()
						v.rows = append(v.rows, row)
					}
				}
			}
			return v
		}
	}()
	old := v.view
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := func() cursorIdentity {
		var cursor diffCursor = v.cursor
		if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
			var current entry = old.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !old.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return cursorIdentity{}
		}
		row := old.rows[cursor.row]
		file := old.patch.Files[row.file]
		hunk := file.Hunks[row.hunk]
		line := hunk.Lines[func() int {
			if row.kind != lineRow {
				return -1
			}
			if !old.split || cursor.pane == right {
				return row.rightLine
			}
			return row.leftLine
		}()]
		return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
	}()
	origin := func() cursorIdentity {
		if !(v.search.from.row >= 0 && v.search.from.row < len(old.rows) && func() int {
			var current entry = old.rows[v.search.from.row]
			if current.kind != lineRow {
				return -1
			}
			if !old.split || v.search.from.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return cursorIdentity{}
		}
		row := old.rows[v.search.from.row]
		file := old.patch.Files[row.file]
		hunk := file.Hunks[row.hunk]
		line := hunk.Lines[func() int {
			if row.kind != lineRow {
				return -1
			}
			if !old.split || v.search.from.pane == right {
				return row.rightLine
			}
			return row.leftLine
		}()]
		return cursorIdentity{file: file, hunk: hunk, line: line, cursor: v.search.from, valid: true}
	}()
	v.viewport = func() diffViewport {
		var width, height int = v.viewport.Width, v.viewport.Height
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, width), max(1, height)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(next.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if next.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range next.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	v.viewport = func() diffViewport {
		var (
			viewport diffViewport = v.viewport
			columns  int          = leftColumn
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if next.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range next.rows {
				if current.kind != lineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(next.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if next.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range next.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	translated, ok := func() (diffCursor, bool) {
		if !cursor.valid {
			return diffCursor{}, false
		}
		best := cursorMatch{}
		for y := range next.rows {
			candidate := func() cursorMatch {
				current := next.rows[y]
				if !(current.hunk >= 0 && func() bool {
					var candidate patch.File = next.patch.Files[current.file]
					return candidate.OldPath != "" && candidate.OldPath == cursor.file.OldPath || candidate.NewPath != "" && candidate.NewPath == cursor.file.NewPath
				}()) {
					return cursorMatch{}
				}
				best := cursorMatch{}
				for _, pane := range [2]diffPane{cursor.cursor.pane, func() diffPane {
					if cursor.cursor.pane == left {
						return right
					}
					return left
				}()} {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
							var current entry = next.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !next.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
							var current entry = next.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !next.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := next.rows[candidate.row]
						return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !next.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					match := cursorMatch{cursor: candidate, rank: func() int {
						if line.Kind != cursor.line.Kind {
							return 1
						}
						if line.OldNumber == cursor.line.OldNumber && line.NewNumber == cursor.line.NewNumber {
							return 3
						}
						if line.Text == cursor.line.Text {
							return 2
						}
						return 1
					}(), distance: func() int {
						var rowDistance int = y - cursor.cursor.row
						if line.NewNumber > 0 && cursor.line.NewNumber > 0 {
							if int(line.NewNumber-cursor.line.NewNumber) < 0 {
								return -int(line.NewNumber - cursor.line.NewNumber)
							}
							return int(line.NewNumber - cursor.line.NewNumber)
						}
						if line.OldNumber > 0 && cursor.line.OldNumber > 0 {
							if int(line.OldNumber-cursor.line.OldNumber) < 0 {
								return -int(line.OldNumber - cursor.line.OldNumber)
							}
							return int(line.OldNumber - cursor.line.OldNumber)
						}
						if rowDistance < 0 {
							return -rowDistance
						}
						return rowDistance
					}()}
					if match.rank == 3 {
						return match
					}
					best = func() cursorMatch {
						if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
							return match
						}
						return best
					}()
				}
				return best
			}()
			if candidate.rank == 3 {
				return candidate.cursor, true
			}
			if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
				best = candidate
			}
		}
		return best.cursor, best.rank > 0
	}()
	if !ok {
		translated, _ = func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
						return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
							var current entry = next.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !next.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
					return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
						var current entry = next.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !next.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
	}
	v.cursor = translated
	v.selection = func() *diffSelection {
		var selection *diffSelection = v.selection
		if selection == nil {
			return nil
		}
		first := func() cursorIdentity {
			var cursor diffCursor = selection.First
			if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
				var current entry = old.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !old.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return cursorIdentity{}
			}
			row := old.rows[cursor.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != lineRow {
					return -1
				}
				if !old.split || cursor.pane == right {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		last := func() cursorIdentity {
			var cursor diffCursor = selection.Last
			if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
				var current entry = old.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !old.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return cursorIdentity{}
			}
			row := old.rows[cursor.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != lineRow {
					return -1
				}
				if !old.split || cursor.pane == right {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		if !first.valid || !last.valid {
			return nil
		}
		translatedFirst, firstOK := func() (diffCursor, bool) {
			if !first.valid {
				return diffCursor{}, false
			}
			best := cursorMatch{}
			for y := range next.rows {
				candidate := func() cursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patch.File = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == first.file.OldPath || candidate.NewPath != "" && candidate.NewPath == first.file.NewPath
					}()) {
						return cursorMatch{}
					}
					best := cursorMatch{}
					for _, pane := range [2]diffPane{first.cursor.pane, func() diffPane {
						if first.cursor.pane == left {
							return right
						}
						return left
					}()} {
						candidate, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current entry = next.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !next.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patch.Line, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current entry = next.rows[candidate.row]
								if current.kind != lineRow {
									return -1
								}
								if !next.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.Line{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != lineRow {
									return -1
								}
								if !next.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := cursorMatch{cursor: candidate, rank: func() int {
							if line.Kind != first.line.Kind {
								return 1
							}
							if line.OldNumber == first.line.OldNumber && line.NewNumber == first.line.NewNumber {
								return 3
							}
							if line.Text == first.line.Text {
								return 2
							}
							return 1
						}(), distance: func() int {
							var rowDistance int = y - first.cursor.row
							if line.NewNumber > 0 && first.line.NewNumber > 0 {
								if int(line.NewNumber-first.line.NewNumber) < 0 {
									return -int(line.NewNumber - first.line.NewNumber)
								}
								return int(line.NewNumber - first.line.NewNumber)
							}
							if line.OldNumber > 0 && first.line.OldNumber > 0 {
								if int(line.OldNumber-first.line.OldNumber) < 0 {
									return -int(line.OldNumber - first.line.OldNumber)
								}
								return int(line.OldNumber - first.line.OldNumber)
							}
							if rowDistance < 0 {
								return -rowDistance
							}
							return rowDistance
						}()}
						if match.rank == 3 {
							return match
						}
						best = func() cursorMatch {
							if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
								return match
							}
							return best
						}()
					}
					return best
				}()
				if candidate.rank == 3 {
					return candidate.cursor, true
				}
				if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
					best = candidate
				}
			}
			return best.cursor, best.rank > 0
		}()
		translatedLast, lastOK := func() (diffCursor, bool) {
			if !last.valid {
				return diffCursor{}, false
			}
			best := cursorMatch{}
			for y := range next.rows {
				candidate := func() cursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patch.File = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == last.file.OldPath || candidate.NewPath != "" && candidate.NewPath == last.file.NewPath
					}()) {
						return cursorMatch{}
					}
					best := cursorMatch{}
					for _, pane := range [2]diffPane{last.cursor.pane, func() diffPane {
						if last.cursor.pane == left {
							return right
						}
						return left
					}()} {
						candidate, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current entry = next.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !next.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patch.Line, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current entry = next.rows[candidate.row]
								if current.kind != lineRow {
									return -1
								}
								if !next.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.Line{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != lineRow {
									return -1
								}
								if !next.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := cursorMatch{cursor: candidate, rank: func() int {
							if line.Kind != last.line.Kind {
								return 1
							}
							if line.OldNumber == last.line.OldNumber && line.NewNumber == last.line.NewNumber {
								return 3
							}
							if line.Text == last.line.Text {
								return 2
							}
							return 1
						}(), distance: func() int {
							var rowDistance int = y - last.cursor.row
							if line.NewNumber > 0 && last.line.NewNumber > 0 {
								if int(line.NewNumber-last.line.NewNumber) < 0 {
									return -int(line.NewNumber - last.line.NewNumber)
								}
								return int(line.NewNumber - last.line.NewNumber)
							}
							if line.OldNumber > 0 && last.line.OldNumber > 0 {
								if int(line.OldNumber-last.line.OldNumber) < 0 {
									return -int(line.OldNumber - last.line.OldNumber)
								}
								return int(line.OldNumber - last.line.OldNumber)
							}
							if rowDistance < 0 {
								return -rowDistance
							}
							return rowDistance
						}()}
						if match.rank == 3 {
							return match
						}
						best = func() cursorMatch {
							if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
								return match
							}
							return best
						}()
					}
					return best
				}()
				if candidate.rank == 3 {
					return candidate.cursor, true
				}
				if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
					best = candidate
				}
			}
			return best.cursor, best.rank > 0
		}()
		if !firstOK || !lastOK || !(first.file.OldPath != "" && first.file.OldPath == last.file.OldPath || first.file.NewPath != "" && first.file.NewPath == last.file.NewPath) || first.hunk.Header != last.hunk.Header {
			return nil
		}
		translated := diffSelection{First: translatedFirst, Last: translatedFirst}
		translated, ok := func() (diffSelection, bool) {
			var selection diffSelection = translated
			if !(selection.First.row >= 0 && selection.First.row < len(next.rows) && func() int {
				var (
					current entry    = next.rows[selection.First.row]
					pane    diffPane = selection.First.pane
				)
				if current.kind != lineRow {
					return -1
				}
				if !next.split || pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(translatedLast.row >= 0 && translatedLast.row < len(next.rows) && func() int {
				var current entry = next.rows[translatedLast.row]
				if current.kind != lineRow {
					return -1
				}
				if !next.split || translatedLast.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return selection, false
			}
			first := next.rows[selection.First.row]
			last := next.rows[translatedLast.row]
			if first.file != last.file || first.hunk != last.hunk {
				return selection, false
			}
			selection.Last = translatedLast
			return selection, true
		}()
		if !ok {
			return nil
		}
		return &translated
	}()
	v.view = next
	if func() bool {
		var cursor diffCursor = v.cursor
		return cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
			var current entry = next.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !next.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0
	}() {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = func() diffViewport {
			var (
				viewport diffViewport = v.viewport
				cursor   diffCursor   = v.cursor
			)
			if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
				var current entry = next.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !next.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if next.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range next.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if next.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range next.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			for range 2 {
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(next.rows) && next.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}()
				if cursor.row < viewport.top {
					viewport.top = cursor.row
				}
				if cursor.row >= viewport.top+height {
					viewport.top = cursor.row - height + 1
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if next.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range next.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
			}
			return viewport
		}()
	}
	if v.search.active {
		v.search.from = v.cursor
		if translated, ok := func() (diffCursor, bool) {
			if !origin.valid {
				return diffCursor{}, false
			}
			best := cursorMatch{}
			for y := range next.rows {
				candidate := func() cursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patch.File = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == origin.file.OldPath || candidate.NewPath != "" && candidate.NewPath == origin.file.NewPath
					}()) {
						return cursorMatch{}
					}
					best := cursorMatch{}
					for _, pane := range [2]diffPane{origin.cursor.pane, func() diffPane {
						if origin.cursor.pane == left {
							return right
						}
						return left
					}()} {
						candidate, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current entry = next.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !next.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patch.Line, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current entry = next.rows[candidate.row]
								if current.kind != lineRow {
									return -1
								}
								if !next.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.Line{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != lineRow {
									return -1
								}
								if !next.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := cursorMatch{cursor: candidate, rank: func() int {
							if line.Kind != origin.line.Kind {
								return 1
							}
							if line.OldNumber == origin.line.OldNumber && line.NewNumber == origin.line.NewNumber {
								return 3
							}
							if line.Text == origin.line.Text {
								return 2
							}
							return 1
						}(), distance: func() int {
							var rowDistance int = y - origin.cursor.row
							if line.NewNumber > 0 && origin.line.NewNumber > 0 {
								if int(line.NewNumber-origin.line.NewNumber) < 0 {
									return -int(line.NewNumber - origin.line.NewNumber)
								}
								return int(line.NewNumber - origin.line.NewNumber)
							}
							if line.OldNumber > 0 && origin.line.OldNumber > 0 {
								if int(line.OldNumber-origin.line.OldNumber) < 0 {
									return -int(line.OldNumber - origin.line.OldNumber)
								}
								return int(line.OldNumber - origin.line.OldNumber)
							}
							if rowDistance < 0 {
								return -rowDistance
							}
							return rowDistance
						}()}
						if match.rank == 3 {
							return match
						}
						best = func() cursorMatch {
							if match.rank > best.rank || match.rank == best.rank && match.distance < best.distance {
								return match
							}
							return best
						}()
					}
					return best
				}()
				if candidate.rank == 3 {
					return candidate.cursor, true
				}
				if candidate.rank > best.rank || candidate.rank == best.rank && candidate.distance < best.distance {
					best = candidate
				}
			}
			return best.cursor, best.rank > 0
		}(); ok {
			v.search.from = translated
		}
		func() {
			var query string = v.search.query
			v.search.query = query
			if query == "" {
				var cursor diffCursor = v.search.from
				v.cursor = cursor
				v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
					for range 2 {
						height := func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								height--
							}
							return max(1, height)
						}()
						if cursor.row < viewport.top {
							viewport.top = cursor.row
						}
						if cursor.row >= viewport.top+height {
							viewport.top = cursor.row - height + 1
						}
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
					}
					return viewport
				}(v.view, v.viewport)
				v.search.miss = false
				return
			}
			cursor, ok := func(v *diffView, query string, cursor diffCursor) (diffCursor, bool) {
				if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffCursor{}, false
				}
				query = strings.ToLower(query)
				y := cursor.row
				for count := 0; count < len(v.rows)-1; count++ {
					y = (y + int(forward) + len(v.rows)) % len(v.rows)
					if candidate, ok := func() (diffCursor, bool) {
						var query string = query
						if v.rows[y].kind != lineRow {
							if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
								var (
									y    int      = y
									pane diffPane = cursor.pane
								)
								for distance := 1; distance <= len(v.rows); distance++ {
									for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
										if candidate, ok := func() (diffCursor, bool) {
											if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
												return diffCursor{}, false
											}
											if candidate, ok := func() (diffCursor, bool) {
												cursor := diffCursor{row: candidateY, pane: pane}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current entry = v.rows[cursor.row]
													if current.kind != lineRow {
														return -1
													}
													if !v.split || cursor.pane == right {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}(); ok {
												return candidate, true
											}
											if v.split {
												cursor := diffCursor{row: candidateY, pane: func() diffPane {
													if pane == left {
														return right
													}
													return left
												}()}
												return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
													var current entry = v.rows[cursor.row]
													if current.kind != lineRow {
														return -1
													}
													if !v.split || cursor.pane == right {
														return current.rightLine
													}
													return current.leftLine
												}() >= 0
											}
											return diffCursor{}, false
										}(); ok {
											return candidate, true
										}
									}
								}
								return diffCursor{}, false
							}
							return diffCursor{}, false
						}
						panes := []diffPane{cursor.pane}
						if v.split {
							panes = append(panes, func() diffPane {
								if cursor.pane == left {
									return right
								}
								return left
							}())
						}
						for _, candidatePane := range panes {
							candidate, ok := func() (diffCursor, bool) {
								cursor := diffCursor{row: y, pane: candidatePane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patch.Line, bool) {
								if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
									var current entry = v.rows[candidate.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || candidate.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patch.Line{}, false
								}
								current := v.rows[candidate.row]
								return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != lineRow {
										return -1
									}
									if !v.split || candidate.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							if strings.Contains(strings.ToLower(line.Text), query) {
								return candidate, true
							}
						}
						return diffCursor{}, false
					}(); ok {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(v.view, query, v.search.from)
			v.search.miss = !ok
			if ok {
				v.cursor = cursor
				v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
					for range 2 {
						height := func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								height--
							}
							return max(1, height)
						}()
						if cursor.row < viewport.top {
							viewport.top = cursor.row
						}
						if cursor.row >= viewport.top+height {
							viewport.top = cursor.row - height + 1
						}
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
					}
					return viewport
				}(v.view, v.viewport)
			}
		}()
	}
}

// Configure changes presentation options while preserving meaningful position.
func (v *View) Configure(options Options) {
	active := v.options.SideBySide && v.width >= 100
	dark := v.options.Dark
	v.options = options
	if active != (v.options.SideBySide && v.width >= 100) || dark != options.Dark {
		v.Update(v.view.patch)
	}
}

// Resize accepts full screen dimensions, reserving space for the header/footer.
func (v *View) Resize(width, height int) {
	v.EndDrag()
	active := v.options.SideBySide && v.width >= 100
	v.width, v.height = width, height
	v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
		viewport.Width, viewport.Height = max(1, width), max(1, frame.BodyHeight(height))
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}(v.view, v.viewport)
	if active != (v.options.SideBySide && v.width >= 100) {
		v.Update(v.view.patch)
	} else {
		v.viewport = func(v *diffView, viewport diffViewport, cursor diffCursor) diffViewport {
			if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			for range 2 {
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}()
				if cursor.row < viewport.top {
					viewport.top = cursor.row
				}
				if cursor.row >= viewport.top+height {
					viewport.top = cursor.row - height + 1
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
			}
			return viewport
		}(v.view, v.viewport, v.cursor)
	}
}

// Move performs navigation and keeps the resulting focus visible. Line and
// page movement extend active selection only within its original hunk.
func (v *View) Move(motion Motion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case PreviousLine:
		func() {
			next, ok := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffCursor{}, false
				}
				var pane diffPane = cursor.pane
				step := int(backward)
				for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(v.view, v.cursor)
			if !ok {
				return
			}
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffView, selection diffSelection) (diffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current entry    = v.rows[selection.First.row]
							pane    diffPane = selection.First.pane
						)
						if current.kind != lineRow {
							return -1
						}
						if !v.split || pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(next.row >= 0 && next.row < len(v.rows) && func() int {
						var current entry = v.rows[next.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || next.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[next.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = next
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.cursor = next
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(next.row >= 0 && next.row < len(v.rows) && func() int {
					var current entry = v.rows[next.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || next.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if next.row < viewport.top {
						viewport.top = next.row
					}
					if next.row >= viewport.top+height {
						viewport.top = next.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}()
	case NextLine:
		func() {
			next, ok := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffCursor{}, false
				}
				var pane diffPane = cursor.pane
				step := int(forward)
				for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(v.view, v.cursor)
			if !ok {
				return
			}
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffView, selection diffSelection) (diffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current entry    = v.rows[selection.First.row]
							pane    diffPane = selection.First.pane
						)
						if current.kind != lineRow {
							return -1
						}
						if !v.split || pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(next.row >= 0 && next.row < len(v.rows) && func() int {
						var current entry = v.rows[next.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || next.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[next.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = next
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.cursor = next
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(next.row >= 0 && next.row < len(v.rows) && func() int {
					var current entry = v.rows[next.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || next.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if next.row < viewport.top {
						viewport.top = next.row
					}
					if next.row >= viewport.top+height {
						viewport.top = next.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}()
	case PreviousPage:
		func() {
			viewport, cursor := func(v *diffView, viewport diffViewport, cursor diffCursor) (diffViewport, diffCursor) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var (
						current entry    = v.rows[cursor.row]
						pane    diffPane = cursor.pane
					)
					if current.kind != lineRow {
						return -1
					}
					if !v.split || pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return viewport, cursor
				}
				distance := int(backward) * max(1, viewport.Height/2)
				viewport.top += distance
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				target := min(len(v.rows)-1, max(0, cursor.row+distance))
				candidate, ok := func() (diffCursor, bool) {
					var (
						pane     diffPane     = cursor.pane
						viewport diffViewport = viewport
					)
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					for distance := 0; distance < height; distance++ {
						for _, y := range []int{target + int(backward)*distance, target - int(backward)*distance} {
							if cursor, ok := func() (diffCursor, bool) {
								if y < viewport.top || y >= viewport.top+func() int {
									height := viewport.Height
									if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
										height--
									}
									return max(1, height)
								}() {
									return diffCursor{}, false
								}
								cursor := diffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
					}
					return diffCursor{}, false
				}()
				if ok {
					cursor = candidate
				}
				return viewport, cursor
			}(v.view, v.viewport, v.cursor)
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffView, selection diffSelection) (diffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current entry    = v.rows[selection.First.row]
							pane    diffPane = selection.First.pane
						)
						if current.kind != lineRow {
							return -1
						}
						if !v.split || pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[cursor.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = cursor
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.viewport, v.cursor = viewport, cursor
		}()
	case NextPage:
		func() {
			viewport, cursor := func(v *diffView, viewport diffViewport, cursor diffCursor) (diffViewport, diffCursor) {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var (
						current entry    = v.rows[cursor.row]
						pane    diffPane = cursor.pane
					)
					if current.kind != lineRow {
						return -1
					}
					if !v.split || pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return viewport, cursor
				}
				distance := int(forward) * max(1, viewport.Height/2)
				viewport.top += distance
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				target := min(len(v.rows)-1, max(0, cursor.row+distance))
				candidate, ok := func() (diffCursor, bool) {
					var (
						pane     diffPane     = cursor.pane
						viewport diffViewport = viewport
					)
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					for distance := 0; distance < height; distance++ {
						for _, y := range []int{target + int(forward)*distance, target - int(forward)*distance} {
							if cursor, ok := func() (diffCursor, bool) {
								if y < viewport.top || y >= viewport.top+func() int {
									height := viewport.Height
									if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
										height--
									}
									return max(1, height)
								}() {
									return diffCursor{}, false
								}
								cursor := diffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
					}
					return diffCursor{}, false
				}()
				if ok {
					cursor = candidate
				}
				return viewport, cursor
			}(v.view, v.viewport, v.cursor)
			if !func() bool {
				if v.selection == nil {
					return true
				}
				selection, ok := func(v *diffView, selection diffSelection) (diffSelection, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current entry    = v.rows[selection.First.row]
							pane    diffPane = selection.First.pane
						)
						if current.kind != lineRow {
							return -1
						}
						if !v.split || pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return selection, false
					}
					first := v.rows[selection.First.row]
					last := v.rows[cursor.row]
					if first.file != last.file || first.hunk != last.hunk {
						return selection, false
					}
					selection.Last = cursor
					return selection, true
				}(v.view, *v.selection)
				if ok {
					v.selection = &selection
				}
				return ok
			}() {
				return
			}
			v.viewport, v.cursor = viewport, cursor
		}()
	case FirstLine:
		if cursor, ok := func() (diffCursor, bool) {
			var v *diffView = v.view
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
					return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			v.cursor = cursor
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}
	case LastLine:
		if cursor, ok := func() (diffCursor, bool) {
			var v *diffView = v.view
			if cursor, ok := func() (diffCursor, bool) {
				var start int = len(v.rows)
				step := int(backward)
				for y := start + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			var start int = len(v.rows)
			step := int(backward)
			for y := start + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
					return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			v.cursor = cursor
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}

	default:
		func() {
			switch motion {
			case PreviousFile:
				v.ClearSelection()
				if cursor, ok := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return diffCursor{}, false
					}
					file, _ := func() (patch.File, bool) {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.File{}, false
						}
						return v.patch.Files[v.rows[cursor.row].file], true
					}()
					y := cursor.row
					for {
						next, ok := func() (diffCursor, bool) {
							var (
								start int      = y
								pane  diffPane = cursor.pane
							)
							step := int(backward)
							for y := start + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}()
						if !ok {
							return diffCursor{}, false
						}
						nextFile, _ := func() (patch.File, bool) {
							if !(next.row >= 0 && next.row < len(v.rows) && func() int {
								var current entry = v.rows[next.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || next.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.File{}, false
							}
							return v.patch.Files[v.rows[next.row].file], true
						}()
						if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
							return next, true
						}
						y = next.row
					}
				}(v.view, v.cursor); ok {
					v.cursor = cursor
					v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffViewport {
								var viewport diffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != lineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}
			case NextFile:
				v.ClearSelection()
				if cursor, ok := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
					if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current entry = v.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || cursor.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return diffCursor{}, false
					}
					file, _ := func() (patch.File, bool) {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.File{}, false
						}
						return v.patch.Files[v.rows[cursor.row].file], true
					}()
					y := cursor.row
					for {
						next, ok := func() (diffCursor, bool) {
							var (
								start int      = y
								pane  diffPane = cursor.pane
							)
							step := int(forward)
							for y := start + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}()
						if !ok {
							return diffCursor{}, false
						}
						nextFile, _ := func() (patch.File, bool) {
							if !(next.row >= 0 && next.row < len(v.rows) && func() int {
								var current entry = v.rows[next.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || next.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.File{}, false
							}
							return v.patch.Files[v.rows[next.row].file], true
						}()
						if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
							return next, true
						}
						y = next.row
					}
				}(v.view, v.cursor); ok {
					v.cursor = cursor
					v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffViewport {
								var viewport diffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != lineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}
			case OldPane:
				func() {
					cursor, ok := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
						if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return diffCursor{}, false
						}
						if candidate, ok := func() (diffCursor, bool) {
							var y int = cursor.row
							cursor := diffCursor{row: y, pane: left}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return candidate, true
						}
						if candidate, ok := func() (diffCursor, bool) {
							for y := cursor.row - 1; y >= 0; y-- {
								if v.rows[y].file != v.rows[cursor.row].file {
									break
								}
								if candidate, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: left}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return candidate, true
								}
							}
							return diffCursor{}, false
						}(); ok {
							return candidate, true
						}
						step := int(forward)
						for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
							if cursor, ok := func() (diffCursor, bool) {
								cursor := diffCursor{row: y, pane: left}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
						return diffCursor{}, false
					}(v.view, v.cursor)
					if !ok {
						return
					}
					if v.selection != nil {
						first, firstOK := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffCursor{}, false
							}
							if candidate, ok := func() (diffCursor, bool) {
								var y int = cursor.row
								cursor := diffCursor{row: y, pane: left}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: y, pane: left}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(forward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: left}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}(v.view, v.selection.First)
						last, lastOK := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffCursor{}, false
							}
							if candidate, ok := func() (diffCursor, bool) {
								var y int = cursor.row
								cursor := diffCursor{row: y, pane: left}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: y, pane: left}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(forward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: left}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}(v.view, v.selection.Last)
						if !firstOK || !lastOK {
							return
						}
						selection := diffSelection{First: first, Last: first}
						selection, ok = func() (diffSelection, bool) {
							var (
								v         *diffView     = v.view
								selection diffSelection = selection
								cursor    diffCursor    = last
							)
							if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
								var (
									current entry    = v.rows[selection.First.row]
									pane    diffPane = selection.First.pane
								)
								if current.kind != lineRow {
									return -1
								}
								if !v.split || pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return selection, false
							}
							first := v.rows[selection.First.row]
							last := v.rows[cursor.row]
							if first.file != last.file || first.hunk != last.hunk {
								return selection, false
							}
							selection.Last = cursor
							return selection, true
						}()
						if !ok {
							return
						}
						v.selection = &selection
					}
					v.cursor = cursor
					v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffViewport {
								var viewport diffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != lineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}()
			case NewPane:
				func() {
					cursor, ok := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
						if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return diffCursor{}, false
						}
						if candidate, ok := func() (diffCursor, bool) {
							var y int = cursor.row
							cursor := diffCursor{row: y, pane: right}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return candidate, true
						}
						if candidate, ok := func() (diffCursor, bool) {
							for y := cursor.row - 1; y >= 0; y-- {
								if v.rows[y].file != v.rows[cursor.row].file {
									break
								}
								if candidate, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: right}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return candidate, true
								}
							}
							return diffCursor{}, false
						}(); ok {
							return candidate, true
						}
						step := int(forward)
						for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
							if cursor, ok := func() (diffCursor, bool) {
								cursor := diffCursor{row: y, pane: right}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
						return diffCursor{}, false
					}(v.view, v.cursor)
					if !ok {
						return
					}
					if v.selection != nil {
						first, firstOK := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffCursor{}, false
							}
							if candidate, ok := func() (diffCursor, bool) {
								var y int = cursor.row
								cursor := diffCursor{row: y, pane: right}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: y, pane: right}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(forward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: right}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}(v.view, v.selection.First)
						last, lastOK := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffCursor{}, false
							}
							if candidate, ok := func() (diffCursor, bool) {
								var y int = cursor.row
								cursor := diffCursor{row: y, pane: right}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: y, pane: right}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(forward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: right}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}(v.view, v.selection.Last)
						if !firstOK || !lastOK {
							return
						}
						selection := diffSelection{First: first, Last: first}
						selection, ok = func() (diffSelection, bool) {
							var (
								v         *diffView     = v.view
								selection diffSelection = selection
								cursor    diffCursor    = last
							)
							if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
								var (
									current entry    = v.rows[selection.First.row]
									pane    diffPane = selection.First.pane
								)
								if current.kind != lineRow {
									return -1
								}
								if !v.split || pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return selection, false
							}
							first := v.rows[selection.First.row]
							last := v.rows[cursor.row]
							if first.file != last.file || first.hunk != last.hunk {
								return selection, false
							}
							selection.Last = cursor
							return selection, true
						}()
						if !ok {
							return
						}
						v.selection = &selection
					}
					v.cursor = cursor
					v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffViewport {
								var viewport diffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != lineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}()
			case OtherPane:
				func() {
					var pane diffPane = func() diffPane {
						if v.cursor.pane == left {
							return right
						}
						return left
					}()
					cursor, ok := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
						if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return diffCursor{}, false
						}
						if candidate, ok := func() (diffCursor, bool) {
							var y int = cursor.row
							cursor := diffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return candidate, true
						}
						if candidate, ok := func() (diffCursor, bool) {
							for y := cursor.row - 1; y >= 0; y-- {
								if v.rows[y].file != v.rows[cursor.row].file {
									break
								}
								if candidate, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return candidate, true
								}
							}
							return diffCursor{}, false
						}(); ok {
							return candidate, true
						}
						step := int(forward)
						for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
							if cursor, ok := func() (diffCursor, bool) {
								cursor := diffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return cursor, true
							}
						}
						return diffCursor{}, false
					}(v.view, v.cursor)
					if !ok {
						return
					}
					if v.selection != nil {
						first, firstOK := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffCursor{}, false
							}
							if candidate, ok := func() (diffCursor, bool) {
								var y int = cursor.row
								cursor := diffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: y, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(forward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}(v.view, v.selection.First)
						last, lastOK := func(v *diffView, cursor diffCursor) (diffCursor, bool) {
							if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return diffCursor{}, false
							}
							if candidate, ok := func() (diffCursor, bool) {
								var y int = cursor.row
								cursor := diffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
									var current entry = v.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !v.split || cursor.pane == right {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}(); ok {
								return candidate, true
							}
							if candidate, ok := func() (diffCursor, bool) {
								for y := cursor.row - 1; y >= 0; y-- {
									if v.rows[y].file != v.rows[cursor.row].file {
										break
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: y, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
								}
								return diffCursor{}, false
							}(); ok {
								return candidate, true
							}
							step := int(forward)
							for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffCursor, bool) {
									cursor := diffCursor{row: y, pane: pane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current entry = v.rows[cursor.row]
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffCursor{}, false
						}(v.view, v.selection.Last)
						if !firstOK || !lastOK {
							return
						}
						selection := diffSelection{First: first, Last: first}
						selection, ok = func() (diffSelection, bool) {
							var (
								v         *diffView     = v.view
								selection diffSelection = selection
								cursor    diffCursor    = last
							)
							if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
								var (
									current entry    = v.rows[selection.First.row]
									pane    diffPane = selection.First.pane
								)
								if current.kind != lineRow {
									return -1
								}
								if !v.split || pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return selection, false
							}
							first := v.rows[selection.First.row]
							last := v.rows[cursor.row]
							if first.file != last.file || first.hunk != last.hunk {
								return selection, false
							}
							selection.Last = cursor
							return selection, true
						}()
						if !ok {
							return
						}
						v.selection = &selection
					}
					v.cursor = cursor
					v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
									end++
								}
								return end
							}()))
							viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
								contentWidth := max(1, viewport.Width-14)
								extra := 0
								if v.split {
									contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
								}
								longest := 0
								for _, current := range v.rows {
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						for range 2 {
							height := func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}()
							if cursor.row < viewport.top {
								viewport.top = cursor.row
							}
							if cursor.row >= viewport.top+height {
								viewport.top = cursor.row - height + 1
							}
							viewport = func() diffViewport {
								var viewport diffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != lineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
						}
						return viewport
					}(v.view, v.viewport)
				}()
			default:
				return
			}
		}()
	}
}

// extendSelectionTo accepts unrestricted movement when no range is active.
// An active range changes only when its endpoint remains in the original hunk.
// Align positions the focused line at the requested viewport alignment.
func (v *View) Align(alignment Alignment) {
	v.search.repeatMiss = false
	if _, ok := func(v *diffView, cursor diffCursor) (patch.Line, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[cursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}(v.view, v.cursor); !ok {
		return
	}
	v.viewport = func(v *diffView, viewport diffViewport, cursor diffCursor) diffViewport {
		headerHeight := 0
		if viewport.Height > 1 {
			headerHeight = 1
		}
		offset := max(0, func() int {
			var height int = viewport.Height
			if verticalAlignment(alignment) == middle {
				return height / 2
			}
			if verticalAlignment(alignment) == bottom {
				return height - 1
			}
			return 0
		}()-headerHeight)
		viewport.top = cursor.row - offset
		if !(viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow) {
			viewport.top = cursor.row - func() int {
				var height int = viewport.Height
				if verticalAlignment(alignment) == middle {
					return height / 2
				}
				if verticalAlignment(alignment) == bottom {
					return height - 1
				}
				return 0
			}()
		}
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}(v.view,
		v.viewport,
		v.cursor)
}

// ScrollHorizontal shifts the visible content by the requested columns.
func (v *View) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	v.viewport = func(v *diffView, viewport diffViewport, columns int) diffViewport {
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if v.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range v.rows {
				if current.kind != lineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}(v.view, v.viewport, columns)
}

// ScrollVertical moves content first, moving focus only when it leaves the
// visible code. Like keyboard movement, it extends selection within its hunk.
func (v *View) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	viewport, cursor := func(v *diffView, viewport diffViewport, cursor diffCursor, rows int) (diffViewport, diffCursor) {
		rows = max(-viewport.top, min(rows, func() int {
			end := max(0, len(v.rows)-viewport.Height)
			if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
				end++
			}
			return end
		}()-viewport.top))
		viewport.top += rows
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}()
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var (
				current entry    = v.rows[cursor.row]
				pane    diffPane = cursor.pane
			)
			if current.kind != lineRow {
				return -1
			}
			if !v.split || pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || func() bool {
			var viewport diffViewport = viewport
			return cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0 && cursor.row >= viewport.top && cursor.row < viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}()
		}() {
			return viewport, cursor
		}
		target := max(viewport.top, min(cursor.row, viewport.top+func() int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				height--
			}
			return max(1, height)
		}()-1))
		d := forward
		if cursor.row > target {
			d = backward
		}
		candidate, ok := func() (diffCursor, bool) {
			var (
				pane     diffPane     = cursor.pane
				dir      direction    = d
				viewport diffViewport = viewport
			)
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}()
			for distance := 0; distance < height; distance++ {
				for _, y := range []int{target + int(dir)*distance, target - int(dir)*distance} {
					if cursor, ok := func() (diffCursor, bool) {
						if y < viewport.top || y >= viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								height--
							}
							return max(1, height)
						}() {
							return diffCursor{}, false
						}
						cursor := diffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
			}
			return diffCursor{}, false
		}()
		if !ok && v.split {
			candidate, ok = func() (diffCursor, bool) {
				var (
					pane diffPane = func() diffPane {
						if cursor.pane == left {
							return right
						}
						return left
					}()
					dir      direction    = d
					viewport diffViewport = viewport
				)
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}()
				for distance := 0; distance < height; distance++ {
					for _, y := range []int{target + int(dir)*distance, target - int(dir)*distance} {
						if cursor, ok := func() (diffCursor, bool) {
							if y < viewport.top || y >= viewport.top+func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}() {
								return diffCursor{}, false
							}
							cursor := diffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return cursor, true
						}
					}
				}
				return diffCursor{}, false
			}()
		}
		if ok {
			cursor = candidate
		}
		return viewport, cursor
	}(v.view, v.viewport, v.cursor, rows)
	if cursor != v.cursor && !func() bool {
		if v.selection == nil {
			return true
		}
		selection, ok := func(v *diffView, selection diffSelection) (diffSelection, bool) {
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current entry    = v.rows[selection.First.row]
					pane    diffPane = selection.First.pane
				)
				if current.kind != lineRow {
					return -1
				}
				if !v.split || pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return selection, false
			}
			first := v.rows[selection.First.row]
			last := v.rows[cursor.row]
			if first.file != last.file || first.hunk != last.hunk {
				return selection, false
			}
			selection.Last = cursor
			return selection, true
		}(v.view, *v.selection)
		if ok {
			v.selection = &selection
		}
		return ok
	}() {
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
	if _, ok := func(v *diffView, cursor diffCursor) (patch.Line, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[cursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}(v.view, v.cursor); ok {
		selection := func() diffSelection {
			var cursor diffCursor = v.cursor
			return diffSelection{First: cursor, Last: cursor}
		}()
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
	selection := v.selection
	if selection == nil {
		current := func() diffSelection {
			var cursor diffCursor = v.cursor
			return diffSelection{First: cursor, Last: cursor}
		}()
		selection = &current
	}
	file, ok := func() (patch.File, bool) {
		var (
			v      *diffView  = v.view
			cursor diffCursor = selection.First
		)
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.File{}, false
		}
		return v.patch.Files[v.rows[cursor.row].file], true
	}()
	lines := func() []patch.Line {
		var (
			v         *diffView     = v.view
			selection diffSelection = *selection
		)
		if _, ok := func() (diffSelection, bool) {
			var selection diffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current entry    = v.rows[selection.First.row]
					pane    diffPane = selection.First.pane
				)
				if current.kind != lineRow {
					return -1
				}
				if !v.split || pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current entry = v.rows[selection.Last.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || selection.Last.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return selection, false
			}
			first := v.rows[selection.First.row]
			last := v.rows[selection.Last.row]
			if first.file != last.file || first.hunk != last.hunk {
				return selection, false
			}
			selection.Last = selection.Last
			return selection, true
		}(); !ok {
			return nil
		}
		first, last := min(selection.First.row, selection.Last.row), max(selection.First.row, selection.Last.row)
		if first == last && selection.First.pane != selection.Last.pane {
			first, _ := func() (patch.Line, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.First.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patch.Line, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patch.Line{first}
			}
			return []patch.Line{first, last}
		}
		lines := make([]patch.Line, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patch.Line, bool) {
				var cursor diffCursor = diffCursor{row: y, pane: pane}
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[cursor.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}()
	return file, lines, ok && len(lines) > 0
}

// Current returns the focused source line, independent of any selected range.
func (v *View) Current() (patch.File, patch.Line, bool) {
	file, fileOK := func(v *diffView, cursor diffCursor) (patch.File, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.File{}, false
		}
		return v.patch.Files[v.rows[cursor.row].file], true
	}(v.view, v.cursor)
	line, lineOK := func(v *diffView, cursor diffCursor) (patch.Line, bool) {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[cursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}(v.view, v.cursor)
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
	func() {
		var query string = v.search.query + text
		v.search.query = query
		if query == "" {
			var cursor diffCursor = v.search.from
			v.cursor = cursor
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
			v.search.miss = false
			return
		}
		cursor, ok := func(v *diffView, query string, cursor diffCursor) (diffCursor, bool) {
			if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffCursor{}, false
			}
			query = strings.ToLower(query)
			y := cursor.row
			for count := 0; count < len(v.rows)-1; count++ {
				y = (y + int(forward) + len(v.rows)) % len(v.rows)
				if candidate, ok := func() (diffCursor, bool) {
					var query string = query
					if v.rows[y].kind != lineRow {
						if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
							var (
								y    int      = y
								pane diffPane = cursor.pane
							)
							for distance := 1; distance <= len(v.rows); distance++ {
								for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
									if candidate, ok := func() (diffCursor, bool) {
										if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
											return diffCursor{}, false
										}
										if candidate, ok := func() (diffCursor, bool) {
											cursor := diffCursor{row: candidateY, pane: pane}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current entry = v.rows[cursor.row]
												if current.kind != lineRow {
													return -1
												}
												if !v.split || cursor.pane == right {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}(); ok {
											return candidate, true
										}
										if v.split {
											cursor := diffCursor{row: candidateY, pane: func() diffPane {
												if pane == left {
													return right
												}
												return left
											}()}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current entry = v.rows[cursor.row]
												if current.kind != lineRow {
													return -1
												}
												if !v.split || cursor.pane == right {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}
										return diffCursor{}, false
									}(); ok {
										return candidate, true
									}
								}
							}
							return diffCursor{}, false
						}
						return diffCursor{}, false
					}
					panes := []diffPane{cursor.pane}
					if v.split {
						panes = append(panes, func() diffPane {
							if cursor.pane == left {
								return right
							}
							return left
						}())
					}
					for _, candidatePane := range panes {
						candidate, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: candidatePane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patch.Line, bool) {
							if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
								var current entry = v.rows[candidate.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.Line{}, false
							}
							current := v.rows[candidate.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != lineRow {
									return -1
								}
								if !v.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						if strings.Contains(strings.ToLower(line.Text), query) {
							return candidate, true
						}
					}
					return diffCursor{}, false
				}(); ok {
					return candidate, true
				}
			}
			return diffCursor{}, false
		}(v.view, query, v.search.from)
		v.search.miss = !ok
		if ok {
			v.cursor = cursor
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}
	}()
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
	func() {
		v.search.query = string(query)
		if string(query) == "" {
			var cursor diffCursor = v.search.from
			v.cursor = cursor
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
			v.search.miss = false
			return
		}
		cursor, ok := func(v *diffView, query string, cursor diffCursor) (diffCursor, bool) {
			if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffCursor{}, false
			}
			query = strings.ToLower(query)
			y := cursor.row
			for count := 0; count < len(v.rows)-1; count++ {
				y = (y + int(forward) + len(v.rows)) % len(v.rows)
				if candidate, ok := func() (diffCursor, bool) {
					var query string = query
					if v.rows[y].kind != lineRow {
						if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
							var (
								y    int      = y
								pane diffPane = cursor.pane
							)
							for distance := 1; distance <= len(v.rows); distance++ {
								for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
									if candidate, ok := func() (diffCursor, bool) {
										if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
											return diffCursor{}, false
										}
										if candidate, ok := func() (diffCursor, bool) {
											cursor := diffCursor{row: candidateY, pane: pane}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current entry = v.rows[cursor.row]
												if current.kind != lineRow {
													return -1
												}
												if !v.split || cursor.pane == right {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}(); ok {
											return candidate, true
										}
										if v.split {
											cursor := diffCursor{row: candidateY, pane: func() diffPane {
												if pane == left {
													return right
												}
												return left
											}()}
											return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
												var current entry = v.rows[cursor.row]
												if current.kind != lineRow {
													return -1
												}
												if !v.split || cursor.pane == right {
													return current.rightLine
												}
												return current.leftLine
											}() >= 0
										}
										return diffCursor{}, false
									}(); ok {
										return candidate, true
									}
								}
							}
							return diffCursor{}, false
						}
						return diffCursor{}, false
					}
					panes := []diffPane{cursor.pane}
					if v.split {
						panes = append(panes, func() diffPane {
							if cursor.pane == left {
								return right
							}
							return left
						}())
					}
					for _, candidatePane := range panes {
						candidate, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: candidatePane}
							return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
								var current entry = v.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || cursor.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patch.Line, bool) {
							if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
								var current entry = v.rows[candidate.row]
								if current.kind != lineRow {
									return -1
								}
								if !v.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.Line{}, false
							}
							current := v.rows[candidate.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != lineRow {
									return -1
								}
								if !v.split || candidate.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						if strings.Contains(strings.ToLower(line.Text), query) {
							return candidate, true
						}
					}
					return diffCursor{}, false
				}(); ok {
					return candidate, true
				}
			}
			return diffCursor{}, false
		}(v.view, string(query), v.search.from)
		v.search.miss = !ok
		if ok {
			v.cursor = cursor
			v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
				if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
					var current entry = v.rows[cursor.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || cursor.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				for range 2 {
					height := func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}()
					if cursor.row < viewport.top {
						viewport.top = cursor.row
					}
					if cursor.row >= viewport.top+height {
						viewport.top = cursor.row - height + 1
					}
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
								end++
							}
							return end
						}()))
						viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
							contentWidth := max(1, viewport.Width-14)
							extra := 0
							if v.split {
								contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
							}
							longest := 0
							for _, current := range v.rows {
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
				}
				return viewport
			}(v.view, v.viewport)
		}
	}()
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
	var cursor diffCursor = v.search.from
	v.cursor = cursor
	v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if v.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range v.rows {
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}()
		for range 2 {
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}()
			if cursor.row < viewport.top {
				viewport.top = cursor.row
			}
			if cursor.row >= viewport.top+height {
				viewport.top = cursor.row - height + 1
			}
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
		}
		return viewport
	}(v.view, v.viewport)
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
	cursor, ok := func(v *diffView, query string, cursor diffCursor) (diffCursor, bool) {
		if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := cursor.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(direction(d)) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if v.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var (
							y    int      = y
							pane diffPane = cursor.pane
						)
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(direction(d))*distance, y - int(direction(d))*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
									if v.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if pane == left {
												return right
											}
											return left
										}()}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current entry = v.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !v.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}
									return diffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffCursor{}, false
					}
					return diffCursor{}, false
				}
				panes := []diffPane{cursor.pane}
				if v.split {
					panes = append(panes, func() diffPane {
						if cursor.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
						return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current entry = v.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
							var current entry = v.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := v.rows[candidate.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !v.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffCursor{}, false
	}(v.view, v.search.term, v.cursor)
	v.search.repeatMiss = !ok
	if ok {
		v.cursor = cursor
		v.viewport = func(v *diffView, viewport diffViewport) diffViewport {
			if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current entry = v.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if v.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range v.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			for range 2 {
				height := func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}()
				if cursor.row < viewport.top {
					viewport.top = cursor.row
				}
				if cursor.row >= viewport.top+height {
					viewport.top = cursor.row - height + 1
				}
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if v.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range v.rows {
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
			}
			return viewport
		}(v.view, v.viewport)
	}
	return ok
}

// Render returns the full screen, including its header, status footer, and
// final blank line. A non-nil error replaces the status while retaining the
// position label. Dimensions are configured through Resize.
func (v *View) Render(err error) string {
	added, removed := func() (added, removed int) {
		for _, file := range v.view.patch.Files {
			for _, hunk := range file.Hunks {
				counts := func() [2]int {
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
				}()
				added += counts[0]
				removed += counts[1]
			}
		}
		return added, removed
	}()
	counts := mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	header := titleStyle.Render("review-my-slop") + "  " + counts
	body := (func(v *View) func() []string {
		return func() []string {
			var body []string
			if len(v.view.patch.Files) == 0 {
				empty := "No unstaged or untracked changes."
				if v.view.patch.Kind == patch.Branch {
					empty = "No branch or worktree changes."
				}
				body = make([]string, frame.BodyHeight(v.height))
				body[min(1, len(body)-1)] = mutedStyle.Render(empty)
			} else {
				body = strings.Split(
					(func(v *diffView) func(
						viewport diffViewport,
						cursor diffCursor,
						selection *diffSelection,
					) string {
						return func(
							viewport diffViewport,
							cursor diffCursor,
							selection *diffSelection,
						) string {
							viewport = func() diffViewport {
								var viewport diffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
										end++
									}
									return end
								}()))
								viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
									contentWidth := max(1, viewport.Width-14)
									extra := 0
									if v.split {
										contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
									}
									longest := 0
									for _, current := range v.rows {
										if current.kind != lineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
							lines := make([]string, 0, viewport.Height)
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								current := v.rows[viewport.top]
								lines = append(
									lines,
									func() string {
										var path string = v.patch.Files[current.file].DisplayPath
										return fileStyle.Width(max(20, viewport.Width)).Render(path)
									}(),
								)
							}
							end := min(len(v.rows), viewport.top+func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}())
							for y := viewport.top; y < end; y++ {
								current := v.rows[y]
								if v.split && current.kind == lineRow {
									lines = append(
										lines,
										(func(v *diffView) func(
											current entry,
											y int,
											viewport diffViewport,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											return func(
												current entry,
												y int,
												viewport diffViewport,
												cursor diffCursor,
												selection *diffSelection,
											) string {
												leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
													inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
													return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
												}()
												inlineLocalLeft1 := func() string {
													index := func() int {
														if current.kind != lineRow {
															return -1
														}
														if !v.split || left == right {
															return current.rightLine
														}
														return current.leftLine
													}()
													if index < 0 {
														return strings.Repeat(" ", leftWidth)
													}
													line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
													text := current.right
													numberValue := line.NewNumber
													if left == left {
														text, numberValue = current.left, line.OldNumber
													}
													prefix := "  "
													if line.Kind == patch.Addition {
														prefix = addedStyle.Render("+") + " "
													}
													if line.Kind == patch.Deletion {
														prefix = removedStyle.Render("-") + " "
													}
													gutter := fmt.Sprintf("%5s ", func() string {
														if numberValue == 0 {
															return ""
														}
														return strconv.Itoa(int(numberValue))
													}())
													value := gutter + func() string {
														var (
															value string = prefix + text
															width int    = leftWidth - lipgloss.Width(gutter)
														)
														if width <= 0 {
															return ""
														}
														value = strings.ReplaceAll(value, "\t", "    ")
														if viewport.LeftColumn > 0 {
															value = ansi.TruncateLeft(value, viewport.LeftColumn, "")
														}
														value = ansi.Truncate(value, width, "")
														if padding := width - lipgloss.Width(value); padding > 0 {
															value += strings.Repeat(" ", padding)
														}
														return value
													}()
													style := func() lipgloss.Style {
														lightDark := lipgloss.LightDark(v.dark)
														switch line.Kind {
														case patch.Addition:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
														case patch.Deletion:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
														default:
															return contextStyle
														}
													}()
													colors := preserveForeground
													candidate := diffCursor{row: y, pane: left}
													if func() bool {
														if selection == nil {
															return false
														}
														first, last := selection.First, selection.Last
														if first.row == last.row && first.pane != last.pane {
															return candidate.row == first.row && (candidate.pane == first.pane || candidate.pane == last.pane)
														}
														if first.pane != candidate.pane {
															return false
														}
														return candidate.row >= min(first.row, last.row) && candidate.row <= max(first.row, last.row)
													}() {
														style, colors = func() lipgloss.Style {
															var dark bool = v.dark
															return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
														}(), discardForeground
													}
													if cursor == candidate {
														style, colors = cursorStyle, discardForeground
													}
													{
														var (
															style  lipgloss.Style   = style
															value  string           = value
															colors foregroundPolicy = colors
														)
														value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
															parameters := sequence[2 : len(sequence)-1]
															if parameters == "" {
																return sequence
															}
															parts := strings.Split(parameters, ";")
															filtered := func() []string {
																filtered := make([]string, 0, len(parts))
																for index := 0; index < len(parts); index++ {
																	code, err := strconv.Atoi(parts[index])
																	if err != nil {
																		filtered = append(filtered, parts[index])
																		continue
																	}
																	if code == 48 || colors == discardForeground && code == 38 {
																		index = func() int {
																			if index+1 >= len(parts) {
																				return index
																			}
																			switch parts[index+1] {
																			case "2":
																				return min(index+4, len(parts)-1)
																			case "5":
																				return min(index+2, len(parts)-1)
																			default:
																				return index
																			}
																		}()
																		continue
																	}
																	if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
																		continue
																	}
																	filtered = append(filtered, parts[index])
																}
																return filtered
															}()
															if len(filtered) == 0 {
																return ""
															}
															return "\x1b[" + strings.Join(filtered, ";") + "m"
														})
														fitted := func() string {
															var value string = value
															if leftWidth <= 0 {
																return ""
															}
															value = strings.ReplaceAll(value, "\t", "    ")
															if 0 > 0 {
																value = ansi.TruncateLeft(value, 0, "")
															}
															value = ansi.Truncate(value, leftWidth, "")
															if padding := leftWidth - lipgloss.Width(value); padding > 0 {
																value += strings.Repeat(" ", padding)
															}
															return value
														}()
														prefix := func() string {
															const marker = "\x00"
															rendered := style.Render(marker)
															index := strings.Index(rendered, marker)
															if index < 0 {
																return ""
															}
															return rendered[:index]
														}()
														if prefix != "" {
															fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
															fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
														}
														return style.Render(fitted)
													}
												}()
												inlineLocalRight1 := (func(v *diffView) func(
													current entry,
													y int,
													pane diffPane,
													width, offset int,
													cursor diffCursor,
													selection *diffSelection,
												) string {
													return func(
														current entry,
														y int,
														pane diffPane,
														width, offset int,
														cursor diffCursor,
														selection *diffSelection,
													) string {
														index := func() int {
															if current.kind != lineRow {
																return -1
															}
															if !v.split || pane == right {
																return current.rightLine
															}
															return current.leftLine
														}()
														if index < 0 {
															return strings.Repeat(" ", width)
														}
														line := v.patch.Files[current.file].Hunks[current.hunk].Lines[index]
														text := current.right
														numberValue := line.NewNumber
														if pane == left {
															text, numberValue = current.left, line.OldNumber
														}
														prefix := "  "
														if line.Kind == patch.Addition {
															prefix = addedStyle.Render("+") + " "
														}
														if line.Kind == patch.Deletion {
															prefix = removedStyle.Render("-") + " "
														}
														gutter := fmt.Sprintf("%5s ", func() string {
															if numberValue == 0 {
																return ""
															}
															return strconv.Itoa(int(numberValue))
														}())
														value := gutter + func() string {
															var (
																value string = prefix + text
																width int    = width - lipgloss.Width(gutter)
															)
															if width <= 0 {
																return ""
															}
															value = strings.ReplaceAll(value, "\t", "    ")
															if offset > 0 {
																value = ansi.TruncateLeft(value, offset, "")
															}
															value = ansi.Truncate(value, width, "")
															if padding := width - lipgloss.Width(value); padding > 0 {
																value += strings.Repeat(" ", padding)
															}
															return value
														}()
														style := func() lipgloss.Style {
															lightDark := lipgloss.LightDark(v.dark)
															switch line.Kind {
															case patch.Addition:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
															case patch.Deletion:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
															default:
																return contextStyle
															}
														}()
														colors := preserveForeground
														candidate := diffCursor{row: y, pane: pane}
														if func() bool {
															if selection == nil {
																return false
															}
															first, last := selection.First, selection.Last
															if first.row == last.row && first.pane != last.pane {
																return candidate.row == first.row && (candidate.pane == first.pane || candidate.pane == last.pane)
															}
															if first.pane != candidate.pane {
																return false
															}
															return candidate.row >= min(first.row, last.row) && candidate.row <= max(first.row, last.row)
														}() {
															style, colors = func() lipgloss.Style {
																var dark bool = v.dark
																return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
															}(), discardForeground
														}
														if cursor == candidate {
															style, colors = cursorStyle, discardForeground
														}
														{
															var (
																style  lipgloss.Style   = style
																value  string           = value
																colors foregroundPolicy = colors
															)
															value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
																parameters := sequence[2 : len(sequence)-1]
																if parameters == "" {
																	return sequence
																}
																parts := strings.Split(parameters, ";")
																filtered := func() []string {
																	filtered := make([]string, 0, len(parts))
																	for index := 0; index < len(parts); index++ {
																		code, err := strconv.Atoi(parts[index])
																		if err != nil {
																			filtered = append(filtered, parts[index])
																			continue
																		}
																		if code == 48 || colors == discardForeground && code == 38 {
																			index = func() int {
																				if index+1 >= len(parts) {
																					return index
																				}
																				switch parts[index+1] {
																				case "2":
																					return min(index+4, len(parts)-1)
																				case "5":
																					return min(index+2, len(parts)-1)
																				default:
																					return index
																				}
																			}()
																			continue
																		}
																		if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
																			continue
																		}
																		filtered = append(filtered, parts[index])
																	}
																	return filtered
																}()
																if len(filtered) == 0 {
																	return ""
																}
																return "\x1b[" + strings.Join(filtered, ";") + "m"
															})
															fitted := func() string {
																var value string = value
																if width <= 0 {
																	return ""
																}
																value = strings.ReplaceAll(value, "\t", "    ")
																if 0 > 0 {
																	value = ansi.TruncateLeft(value, 0, "")
																}
																value = ansi.Truncate(value, width, "")
																if padding := width - lipgloss.Width(value); padding > 0 {
																	value += strings.Repeat(" ", padding)
																}
																return value
															}()
															prefix := func() string {
																const marker = "\x00"
																rendered := style.Render(marker)
																index := strings.Index(rendered, marker)
																if index < 0 {
																	return ""
																}
																return rendered[:index]
															}()
															if prefix != "" {
																fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
																fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
															}
															return style.Render(fitted)
														}
													}
												}(v))(
													current,
													y,
													right,
													rightWidth,
													viewport.LeftColumn,
													cursor,
													selection,
												)
												return inlineLocalLeft1 + " │ " + inlineLocalRight1
											}
										}(v))(current, y, viewport, cursor, selection),
									)
								} else {
									lines = append(
										lines,
										func() string {
											var y int = y
											width := max(20, viewport.Width)
											switch current.kind {
											case fileRow:
												return fileStyle.Width(max(20, width)).Render(current.text)
											case metadataRow:
												return metadataStyle.Render("  " + current.text)
											case hunkRow:
												return hunkStyle.Render(current.text)
											case lineRow:
												hunk := v.patch.Files[current.file].Hunks[current.hunk]
												line := hunk.Lines[current.rightLine]
												prefix := " "
												if line.Kind == patch.Addition {
													prefix = addedStyle.Render("+")
												}
												if line.Kind == patch.Deletion {
													prefix = removedStyle.Render("-")
												}
												gutter := fmt.Sprintf("%5s %5s %s ", func() string {
													if line.OldNumber == 0 {
														return ""
													}
													return strconv.Itoa(int(line.OldNumber))
												}(), func() string {
													if line.NewNumber == 0 {
														return ""
													}
													return strconv.Itoa(int(line.NewNumber))
												}(), prefix)
												value := gutter + func() string {
													var (
														value string = current.text
														width int    = width - lipgloss.Width(gutter)
													)
													if width <= 0 {
														return ""
													}
													value = strings.ReplaceAll(value, "\t", "    ")
													if viewport.LeftColumn > 0 {
														value = ansi.TruncateLeft(value, viewport.LeftColumn, "")
													}
													value = ansi.Truncate(value, width, "")
													if padding := width - lipgloss.Width(value); padding > 0 {
														value += strings.Repeat(" ", padding)
													}
													return value
												}()
												style := func() lipgloss.Style {
													lightDark := lipgloss.LightDark(v.dark)
													switch line.Kind {
													case patch.Addition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patch.Deletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return contextStyle
													}
												}()
												colors := preserveForeground
												candidate := diffCursor{row: y, pane: cursor.pane}
												if func() bool {
													if selection == nil {
														return false
													}
													first, last := selection.First, selection.Last
													if first.row == last.row && first.pane != last.pane {
														return candidate.row == first.row && (candidate.pane == first.pane || candidate.pane == last.pane)
													}
													if first.pane != candidate.pane {
														return false
													}
													return candidate.row >= min(first.row, last.row) && candidate.row <= max(first.row, last.row)
												}() {
													style, colors = func() lipgloss.Style {
														var dark bool = v.dark
														return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f")))
													}(), discardForeground
												}
												if cursor.row == y {
													style, colors = cursorStyle, discardForeground
												}
												{
													var (
														style  lipgloss.Style   = style
														value  string           = value
														colors foregroundPolicy = colors
													)
													value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
														parameters := sequence[2 : len(sequence)-1]
														if parameters == "" {
															return sequence
														}
														parts := strings.Split(parameters, ";")
														filtered := func() []string {
															filtered := make([]string, 0, len(parts))
															for index := 0; index < len(parts); index++ {
																code, err := strconv.Atoi(parts[index])
																if err != nil {
																	filtered = append(filtered, parts[index])
																	continue
																}
																if code == 48 || colors == discardForeground && code == 38 {
																	index = func() int {
																		if index+1 >= len(parts) {
																			return index
																		}
																		switch parts[index+1] {
																		case "2":
																			return min(index+4, len(parts)-1)
																		case "5":
																			return min(index+2, len(parts)-1)
																		default:
																			return index
																		}
																	}()
																	continue
																}
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
																	continue
																}
																filtered = append(filtered, parts[index])
															}
															return filtered
														}()
														if len(filtered) == 0 {
															return ""
														}
														return "\x1b[" + strings.Join(filtered, ";") + "m"
													})
													fitted := func() string {
														var value string = value
														if width <= 0 {
															return ""
														}
														value = strings.ReplaceAll(value, "\t", "    ")
														if 0 > 0 {
															value = ansi.TruncateLeft(value, 0, "")
														}
														value = ansi.Truncate(value, width, "")
														if padding := width - lipgloss.Width(value); padding > 0 {
															value += strings.Repeat(" ", padding)
														}
														return value
													}()
													prefix := func() string {
														const marker = "\x00"
														rendered := style.Render(marker)
														index := strings.Index(rendered, marker)
														if index < 0 {
															return ""
														}
														return rendered[:index]
													}()
													if prefix != "" {
														fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
														fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
													}
													return style.Render(fitted)
												}
											}
											return ""
										}(),
									)
								}
							}
							for len(lines) < viewport.Height {
								lines = append(lines, "")
							}
							return strings.Join(lines, "\n")
						}
					}(v.view))(v.viewport, v.cursor, v.selection),
					"\n",
				)
			}
			return body
		}
	}(v))()
	status := func() string {
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
	}()
	footer := mutedStyle.Render(status)
	if v.search.repeatMiss {
		message := fmt.Sprintf("no matches for %q", v.search.term)
		footer = errorStyle.Render(message)
	}
	if err != nil {
		footer = errorStyle.Render(strings.ReplaceAll(err.Error(), "\n", " "))
	}
	return frame.Render(header, body, func() string {
		var value string = footer
		label := "local changes"
		if v.view.patch.Kind == patch.Branch {
			label = "branch changes from " + v.view.patch.Branch
		}
		if v.viewport.top > 0 {
			label += fmt.Sprintf(" (%d%%)", func(v *diffView, viewport diffViewport) int {
				if len(v.rows) == 0 {
					return 0
				}
				inlineLocalBottom1 := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}())
				return inlineLocalBottom1 * 100 / len(v.rows)
			}(v.view, v.viewport))
		}
		inlineLocalRight1 := mutedStyle.Render(label)
		width := max(20, v.width)
		value = ansi.Truncate(value, max(0, width-lipgloss.Width(inlineLocalRight1)-1), "")
		return value + strings.Repeat(" ", max(1, width-lipgloss.Width(value)-lipgloss.Width(inlineLocalRight1))) + inlineLocalRight1
	}(), v.height)
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	mutedStyle = lipgloss.NewStyle().Faint(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
