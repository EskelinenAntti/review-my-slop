package diffscreen

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := (func(p patch.Patch, dark bool) *diffView {
		var options Options = Options{Dark: dark}
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
	})(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, true)
	first := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current entry = old.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !old.split || cursor.pane == right {
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
			for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
					return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
						var current entry = old.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !old.split || cursor.pane == right {
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
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	cursor, ok := func() (diffCursor, bool) {
		var query string = "added one"
		if query == "" || !(first.row >= 0 && first.row < len(old.rows) && func() int {
			var current entry = old.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !old.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(old.rows)-1; count++ {
			y = (y + int(forward) + len(old.rows)) % len(old.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if old.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(old.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(old.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(old.rows) || old.rows[candidateY].file != old.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: first.pane}
										return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
											var current entry = old.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !old.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
									if old.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if first.pane == left {
												return right
											}
											return left
										}()}
										return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
											var current entry = old.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !old.split || cursor.pane == right {
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
				panes := []diffPane{first.pane}
				if old.split {
					panes = append(panes, func() diffPane {
						if first.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current entry = old.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !old.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(old.rows) && func() int {
							var current entry = old.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !old.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := old.rows[candidate.row]
						return old.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !old.split || candidate.pane == right {
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
	}()
	testutil.RequireEqual(t, ok, true, "%s", "cursor not found")

	selection := diffSelection{First: first, Last: first}
	selection, ok = func() (diffSelection, bool) {
		var selection diffSelection = selection
		if !(selection.First.row >= 0 && selection.First.row < len(old.rows) && func() int {
			var (
				current entry    = old.rows[selection.First.row]
				pane    diffPane = selection.First.pane
			)
			if current.kind != lineRow {
				return -1
			}
			if !old.split || pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
			var current entry = old.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !old.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return selection, false
		}
		first := old.rows[selection.First.row]
		last := old.rows[cursor.row]
		if first.file != last.file || first.hunk != last.hunk {
			return selection, false
		}
		selection.Last = cursor
		return selection, true
	}()
	testutil.RequireEqual(t, ok, true, "%s", "selection not created")

	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 100), max(1, 4)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(old.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if old.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range old.rows {
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
	viewport = func() diffViewport {
		var viewport diffViewport = viewport
		headerHeight := 0
		if viewport.Height > 1 {
			headerHeight = 1
		}
		offset := max(0, func() int {
			var height int = viewport.Height
			if middle == middle {
				return height / 2
			}
			if middle == bottom {
				return height - 1
			}
			return 0
		}()-headerHeight)
		viewport.top = cursor.row - offset
		if !(viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(old.rows) && old.rows[viewport.top].kind != fileRow) {
			viewport.top = cursor.row - func() int {
				var height int = viewport.Height
				if middle == middle {
					return height / 2
				}
				if middle == bottom {
					return height - 1
				}
				return 0
			}()
		}
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(old.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if old.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range old.rows {
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
	screen := &View{
		view:      old,
		cursor:    cursor,
		selection: &selection,
		viewport:  viewport,
	}

	next := (func(p patch.Patch, dark bool) *diffView {
		var options Options = Options{SideBySide: true, Dark: dark}
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
	})(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, true)
	{
		old := screen.view
		leftColumn := screen.viewport.LeftColumn
		rowsAbove := screen.cursor.row - screen.viewport.top
		cursor := func() cursorIdentity {
			var cursor diffCursor = screen.cursor
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
			if !(screen.search.from.row >= 0 && screen.search.from.row < len(old.rows) && func() int {
				var current entry = old.rows[screen.search.from.row]
				if current.kind != lineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return cursorIdentity{}
			}
			row := old.rows[screen.search.from.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != lineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == right {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return cursorIdentity{file: file, hunk: hunk, line: line, cursor: screen.search.from, valid: true}
		}()
		screen.viewport = func() diffViewport {
			var width, height int = screen.viewport.Width, screen.viewport.Height
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
		screen.viewport = func() diffViewport {
			var (
				viewport diffViewport = screen.viewport
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
		screen.cursor = translated
		screen.selection = func() *diffSelection {
			var selection *diffSelection = screen.selection
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
		screen.view = next
		if func() bool {
			var cursor diffCursor = screen.cursor
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
			screen.viewport.top = max(0, screen.cursor.row-rowsAbove)
			screen.viewport = func() diffViewport {
				var (
					viewport diffViewport = screen.viewport
					cursor   diffCursor   = screen.cursor
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
		if screen.search.active {
			screen.search.from = screen.cursor
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
				screen.search.from = translated
			}
			func() {
				var query string = screen.search.query
				screen.search.query = query
				if query == "" {
					var cursor diffCursor = screen.search.from
					screen.cursor = cursor
					screen.viewport = func(v *diffView, viewport diffViewport) diffViewport {
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
					}(screen.view, screen.viewport)
					screen.search.miss = false
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
				}(screen.view, query, screen.search.from)
				screen.search.miss = !ok
				if ok {
					screen.cursor = cursor
					screen.viewport = func(v *diffView, viewport diffViewport) diffViewport {
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
					}(screen.view, screen.viewport)
				}
			}()
		}
	}
	preserved := screen
	testutil.RequireEqual(t, func() bool {
		var cursor diffCursor = preserved.cursor
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
	}(), true, "%s",
		"cursor was not preserved",
	)

	line, ok := func() (patch.Line, bool) {
		var cursor diffCursor = preserved.cursor
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
			return patch.Line{}, false
		}
		current := next.rows[cursor.row]
		return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !next.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, ok, true, "cursor line = %#v, ok=%v", line, ok)
	testutil.RequireEqual(t, line.Text, "added one", "cursor line = %#v, ok=%v",
		line, ok,
	)
	testutil.RequireEqual(t, preserved.selection != nil, true,
		"selection = %#v", preserved.selection,
	)

	firstLine, firstOK := func() (patch.Line, bool) {
		var cursor diffCursor = preserved.selection.First
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
			return patch.Line{}, false
		}
		current := next.rows[cursor.row]
		return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !next.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	lastLine, lastOK := func() (patch.Line, bool) {
		var cursor diffCursor = preserved.selection.Last
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
			return patch.Line{}, false
		}
		current := next.rows[cursor.row]
		return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !next.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, firstOK, true, "selection endpoints = %#v, %#v",
		firstLine, lastLine,
	)
	testutil.RequireEqual(t, lastOK, true, "selection endpoints = %#v, %#v",
		firstLine, lastLine,
	)
	testutil.RequireEqual(t, firstLine.Text, "before",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)
	testutil.RequireEqual(t, lastLine.Text, "added one",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)

	got := preserved.cursor.row - preserved.viewport.top
	want := cursor.row - viewport.top
	if got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := (func(p patch.Patch, dark bool) *diffView {
		var options Options = Options{Dark: dark}
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
	})(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, true)
	cursor := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current entry = old.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !old.split || cursor.pane == right {
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
			for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
					return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
						var current entry = old.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !old.split || cursor.pane == right {
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
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	screen := &View{
		view:      old,
		cursor:    cursor,
		selection: (func(value diffSelection) *diffSelection { return &value })(diffSelection{First: cursor, Last: cursor}),
		viewport: func() diffViewport {
			var viewport diffViewport = diffViewport{}
			viewport.Width, viewport.Height = max(1, 80), max(1, 10)
			{
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(old.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != fileRow {
						end++
					}
					return end
				}()))
				viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
					contentWidth := max(1, viewport.Width-14)
					extra := 0
					if old.split {
						contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
					}
					longest := 0
					for _, current := range old.rows {
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
		}(),
	}

	{
		var next *diffView = (func(p patch.Patch, dark bool) *diffView {
			var options Options = Options{Dark: dark}
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
		})(patch.Patch{}, true)
		old := screen.view
		leftColumn := screen.viewport.LeftColumn
		rowsAbove := screen.cursor.row - screen.viewport.top
		cursor := func() cursorIdentity {
			var cursor diffCursor = screen.cursor
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
			if !(screen.search.from.row >= 0 && screen.search.from.row < len(old.rows) && func() int {
				var current entry = old.rows[screen.search.from.row]
				if current.kind != lineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return cursorIdentity{}
			}
			row := old.rows[screen.search.from.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != lineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == right {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return cursorIdentity{file: file, hunk: hunk, line: line, cursor: screen.search.from, valid: true}
		}()
		screen.viewport = func() diffViewport {
			var width, height int = screen.viewport.Width, screen.viewport.Height
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
		screen.viewport = func() diffViewport {
			var (
				viewport diffViewport = screen.viewport
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
		screen.cursor = translated
		screen.selection = func() *diffSelection {
			var selection *diffSelection = screen.selection
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
		screen.view = next
		if func() bool {
			var cursor diffCursor = screen.cursor
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
			screen.viewport.top = max(0, screen.cursor.row-rowsAbove)
			screen.viewport = func() diffViewport {
				var (
					viewport diffViewport = screen.viewport
					cursor   diffCursor   = screen.cursor
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
		if screen.search.active {
			screen.search.from = screen.cursor
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
				screen.search.from = translated
			}
			func() {
				var query string = screen.search.query
				screen.search.query = query
				if query == "" {
					var cursor diffCursor = screen.search.from
					screen.cursor = cursor
					screen.viewport = func(v *diffView, viewport diffViewport) diffViewport {
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
					}(screen.view, screen.viewport)
					screen.search.miss = false
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
				}(screen.view, query, screen.search.from)
				screen.search.miss = !ok
				if ok {
					screen.cursor = cursor
					screen.viewport = func(v *diffView, viewport diffViewport) diffViewport {
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
					}(screen.view, screen.viewport)
				}
			}()
		}
	}
	preserved := screen
	testutil.RequireEqual(t, !func() bool {
		var (
			v      *diffView  = preserved.view
			cursor diffCursor = preserved.cursor
		)
		return cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0
	}(), true,
		"state = %#v", preserved,
	)
	testutil.RequireEqual(t, preserved.selection, nil, "state = %#v", preserved)
}

func TestPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := (func() patch.Patch {
		lines := make([]patch.Line, 20)
		for index := range lines {
			lines[index] = patch.Line{
				Kind:      patch.Context,
				Text:      strings.Repeat("long", 20),
				OldNumber: patch.LineNumber(index + 1),
				NewNumber: patch.LineNumber(index + 1),
			}
		}
		return patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "long.go",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		}
	})()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := (func(p patch.Patch, dark bool) *diffView {
		var options Options = Options{Dark: dark}
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
	})(p, true)
	cursor := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current entry = old.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !old.split || cursor.pane == right {
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
			for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
					return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
						var current entry = old.rows[cursor.row]
						if current.kind != lineRow {
							return -1
						}
						if !old.split || cursor.pane == right {
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
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffViewport {
		var (
			viewport diffViewport = func() diffViewport {
				var viewport diffViewport = diffViewport{}
				viewport.Width, viewport.Height = max(1, 40), max(1, 10)
				{
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(old.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != fileRow {
							end++
						}
						return end
					}()))
					viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
						contentWidth := max(1, viewport.Width-14)
						extra := 0
						if old.split {
							contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
						}
						longest := 0
						for _, current := range old.rows {
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
			columns int = 30
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if old.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range old.rows {
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
				end := max(0, len(old.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != fileRow {
					end++
				}
				return end
			}()))
			viewport.LeftColumn = max(0, min(viewport.LeftColumn, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if old.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range old.rows {
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
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true, "%s",
		"fixture does not scroll horizontally",
	)

	short := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}
	preserved := &View{view: old, cursor: cursor, viewport: viewport}
	{
		var next *diffView = (func(p patch.Patch, dark bool) *diffView {
			var options Options = Options{Dark: dark}
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
		})(short, true)
		old := preserved.view
		leftColumn := preserved.viewport.LeftColumn
		rowsAbove := preserved.cursor.row - preserved.viewport.top
		cursor := func() cursorIdentity {
			var cursor diffCursor = preserved.cursor
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
			if !(preserved.search.from.row >= 0 && preserved.search.from.row < len(old.rows) && func() int {
				var current entry = old.rows[preserved.search.from.row]
				if current.kind != lineRow {
					return -1
				}
				if !old.split || preserved.search.from.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return cursorIdentity{}
			}
			row := old.rows[preserved.search.from.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != lineRow {
					return -1
				}
				if !old.split || preserved.search.from.pane == right {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return cursorIdentity{file: file, hunk: hunk, line: line, cursor: preserved.search.from, valid: true}
		}()
		preserved.viewport = func() diffViewport {
			var width, height int = preserved.viewport.Width, preserved.viewport.Height
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
		preserved.viewport = func() diffViewport {
			var (
				viewport diffViewport = preserved.viewport
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
		preserved.cursor = translated
		preserved.selection = func() *diffSelection {
			var selection *diffSelection = preserved.selection
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
		preserved.view = next
		if func() bool {
			var cursor diffCursor = preserved.cursor
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
			preserved.viewport.top = max(0, preserved.cursor.row-rowsAbove)
			preserved.viewport = func() diffViewport {
				var (
					viewport diffViewport = preserved.viewport
					cursor   diffCursor   = preserved.cursor
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
		if preserved.search.active {
			preserved.search.from = preserved.cursor
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
				preserved.search.from = translated
			}
			func() {
				var query string = preserved.search.query
				preserved.search.query = query
				if query == "" {
					var cursor diffCursor = preserved.search.from
					preserved.cursor = cursor
					preserved.viewport = func(v *diffView, viewport diffViewport) diffViewport {
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
					}(preserved.view, preserved.viewport)
					preserved.search.miss = false
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
				}(preserved.view, query, preserved.search.from)
				preserved.search.miss = !ok
				if ok {
					preserved.cursor = cursor
					preserved.viewport = func(v *diffView, viewport diffViewport) diffViewport {
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
					}(preserved.view, preserved.viewport)
				}
			}()
		}
	}
	testutil.RequireEqual(t, preserved.viewport.LeftColumn, 0,
		"horizontal offset = %d", preserved.viewport.LeftColumn,
	)
}

func TestUpdatePreservesCursorAfterLineDeletion(t *testing.T) {
	original, target, cases := func() (original patch.Patch, target patch.Line, cases []cursorDeletionCase) {
		early := patch.Hunk{Header: "@@ -1,0 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "early change", NewNumber: 1}}}
		before := patch.Line{Kind: patch.Context, Text: "before", OldNumber: 50, NewNumber: 50}
		added := patch.Line{Kind: patch.Addition, Text: "removed addition", NewNumber: 51}
		after := patch.Line{Kind: patch.Context, Text: "after", OldNumber: 51, NewNumber: 52}
		target = patch.Line{Kind: patch.Addition, Text: "target", NewNumber: 53}
		late := patch.Hunk{Header: "@@ -100,0 +100 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "late change", NewNumber: 100}}}
		original = patch.Patch{Files: []patch.File{{OldPath: "file.go", NewPath: "file.go", Hunks: []patch.Hunk{early, {Header: "@@ -50,2 +50,4 @@", Lines: []patch.Line{before, added, after, target}}, late}}}}
		shiftedAfter, shiftedTarget := after, target
		shiftedAfter.NewNumber--
		shiftedTarget.NewNumber--
		cases = []cursorDeletionCase{{"line before cursor deleted", []patch.Hunk{early, {Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{before, shiftedAfter, shiftedTarget}}, late}, shiftedTarget}, {"cursor line deleted", []patch.Hunk{early, {Header: "@@ -50,2 +50,3 @@", Lines: []patch.Line{before, added, after}}, late}, after}, {"cursor hunk removed", []patch.Hunk{early, late}, late.Lines[0]}}
		return original, target, cases
	}()
	for _, split := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(
				fmt.Sprintf("%s/split=%v", tc.name, split),
				func(t *testing.T) {
					v := New(original, Options{SideBySide: split})
					v.Resize(120, 4)
					v.BeginSearch()
					v.InsertSearch("target")
					v.AcceptSearch()
					_, line, ok := v.Current()
					testutil.RequireEqual(t, ok, true,
						"initial cursor = %#v, ok=%v", line, ok,
					)
					testutil.RequireEqual(t, line, target,
						"initial cursor = %#v, ok=%v", line, ok,
					)

					v.Update(patch.Patch{Files: []patch.File{{OldPath: "file.go", NewPath: "file.go", Hunks: tc.hunks}}})
					_, line, ok = v.Current()
					testutil.RequireEqual(t, ok, true,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					testutil.RequireEqual(t, line, tc.want,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					t.Helper()
					inlineLocalTop1 := v.viewport.top
					inlineLocalBottom1 := inlineLocalTop1 + func(v *diffView, viewport diffViewport) int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}(v.view, v.viewport)
					testutil.RequireEqual(t, v.cursor.row >= inlineLocalTop1, true, "cursor row %d is above viewport %d:%d", v.cursor.row, inlineLocalTop1, inlineLocalBottom1)
					testutil.RequireEqual(t, v.cursor.row < inlineLocalBottom1, true, "cursor row %d is below viewport %d:%d", v.cursor.row, inlineLocalTop1, inlineLocalBottom1)
				},
			)
		}
	}
}

type cursorDeletionCase struct {
	name  string
	hunks []patch.Hunk
	want  patch.Line
}
