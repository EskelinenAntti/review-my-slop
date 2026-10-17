package main

import (
	bytes "bytes"
	fmt "fmt"
	io "io"
	strings "strings"
	testing "testing"

	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestDiffscreenPreserveTranslatesCursorSelectionAndViewport(t *testing.T) {
	old := (func(p patchPatch, dark bool) *diffscreenDiffView {
		var options diffscreenOptions = diffscreenOptions{Dark: dark}
		v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
		for fileIndex, file := range p.Files {
			var file patchFile = file
			v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
			for _, metadata := range file.Metadata {
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
			}
			highlighted := func() diffscreenPair {
				var file *patchFile = &file
				theme := diffscreenLightTheme
				if v.dark {
					theme = diffscreenDarkTheme
				}
				return diffscreenPair{Old: func() []string {
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
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
					if strings.HasPrefix(hunk.Header, "@@") {
						return hunk.Header
					}
					return "@@ " + hunk.Header
				}()})
				for _, indices := range func() [][2]int {
					rows := make([][2]int, 0, len(hunk.Lines))
					for index := 0; index < len(hunk.Lines); {
						if !v.split || hunk.Lines[index].Kind == patchContext {
							rows = append(rows, [2]int{index, index})
							index++
							continue
						}
						if hunk.Lines[index].Kind == patchAddition {
							rows = append(rows, [2]int{-1, index})
							index++
							continue
						}
						addedStart := func() int {
							var start int = index
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
								start++
							}
							return start
						}()
						end := func() int {
							var start int = addedStart
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
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
					row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
					func() {
						var row *diffscreenEntry = &row
						if !v.split {
							row.text = func() string {
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.leftLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, true)
	first := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current diffscreenEntry = old.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !old.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(diffscreenPrivateForward)
			for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
					return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
						var current diffscreenEntry = old.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !old.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffscreenDiffCursor{}, false
		}()
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	cursor, ok := func() (diffscreenDiffCursor, bool) {
		var query string = "added one"
		if query == "" || !(first.row >= 0 && first.row < len(old.rows) && func() int {
			var current diffscreenEntry = old.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !old.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(old.rows)-1; count++ {
			y = (y + int(diffscreenPrivateForward) + len(old.rows)) % len(old.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if old.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(old.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(old.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(old.rows) || old.rows[candidateY].file != old.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: first.pane}
										return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
											var current diffscreenEntry = old.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !old.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
									if old.split {
										cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
											if first.pane == diffscreenLeft {
												return diffscreenRight
											}
											return diffscreenLeft
										}()}
										return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
											var current diffscreenEntry = old.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !old.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}
									return diffscreenDiffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffscreenDiffCursor{}, false
					}
					return diffscreenDiffCursor{}, false
				}
				panes := []diffscreenDiffPane{first.pane}
				if old.split {
					panes = append(panes, func() diffscreenDiffPane {
						if first.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current diffscreenEntry = old.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !old.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patchLine, bool) {
						if !(candidate.row >= 0 && candidate.row < len(old.rows) && func() int {
							var current diffscreenEntry = old.rows[candidate.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !old.split || candidate.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patchLine{}, false
						}
						current := old.rows[candidate.row]
						return old.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !old.split || candidate.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffscreenDiffCursor{}, false
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "%s", "cursor not found")

	selection := diffscreenDiffSelection{First: first, Last: first}
	selection, ok = func() (diffscreenDiffSelection, bool) {
		var selection diffscreenDiffSelection = selection
		if !(selection.First.row >= 0 && selection.First.row < len(old.rows) && func() int {
			var (
				current diffscreenEntry    = old.rows[selection.First.row]
				pane    diffscreenDiffPane = selection.First.pane
			)
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !old.split || pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
			var current diffscreenEntry = old.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !old.split || cursor.pane == diffscreenRight {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "%s", "selection not created")

	viewport := func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, 100), max(1, 4)
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(old.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != diffscreenFileRow {
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
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	viewport = func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = viewport
		headerHeight := 0
		if viewport.Height > 1 {
			headerHeight = 1
		}
		offset := max(0, func() int {
			var height int = viewport.Height
			if diffscreenMiddle == diffscreenMiddle {
				return height / 2
			}
			if diffscreenMiddle == diffscreenPrivateBottom {
				return height - 1
			}
			return 0
		}()-headerHeight)
		viewport.top = cursor.row - offset
		if !(viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(old.rows) && old.rows[viewport.top].kind != diffscreenFileRow) {
			viewport.top = cursor.row - func() int {
				var height int = viewport.Height
				if diffscreenMiddle == diffscreenMiddle {
					return height / 2
				}
				if diffscreenMiddle == diffscreenPrivateBottom {
					return height - 1
				}
				return 0
			}()
		}
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(old.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != diffscreenFileRow {
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
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	screen := &diffscreenView{
		view:      old,
		cursor:    cursor,
		selection: &selection,
		viewport:  viewport,
	}

	next := (func(p patchPatch, dark bool) *diffscreenDiffView {
		var options diffscreenOptions = diffscreenOptions{SideBySide: true, Dark: dark}
		v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
		for fileIndex, file := range p.Files {
			var file patchFile = file
			v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
			for _, metadata := range file.Metadata {
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
			}
			highlighted := func() diffscreenPair {
				var file *patchFile = &file
				theme := diffscreenLightTheme
				if v.dark {
					theme = diffscreenDarkTheme
				}
				return diffscreenPair{Old: func() []string {
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
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
					if strings.HasPrefix(hunk.Header, "@@") {
						return hunk.Header
					}
					return "@@ " + hunk.Header
				}()})
				for _, indices := range func() [][2]int {
					rows := make([][2]int, 0, len(hunk.Lines))
					for index := 0; index < len(hunk.Lines); {
						if !v.split || hunk.Lines[index].Kind == patchContext {
							rows = append(rows, [2]int{index, index})
							index++
							continue
						}
						if hunk.Lines[index].Kind == patchAddition {
							rows = append(rows, [2]int{-1, index})
							index++
							continue
						}
						addedStart := func() int {
							var start int = index
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
								start++
							}
							return start
						}()
						end := func() int {
							var start int = addedStart
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
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
					row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
					func() {
						var row *diffscreenEntry = &row
						if !v.split {
							row.text = func() string {
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.leftLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, true)
	{
		old := screen.view
		leftColumn := screen.viewport.LeftColumn
		rowsAbove := screen.cursor.row - screen.viewport.top
		cursor := func() diffscreenCursorIdentity {
			var cursor diffscreenDiffCursor = screen.cursor
			if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[cursor.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		origin := func() diffscreenCursorIdentity {
			if !(screen.search.from.row >= 0 && screen.search.from.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[screen.search.from.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[screen.search.from.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: screen.search.from, valid: true}
		}()
		screen.viewport = func() diffscreenDiffViewport {
			var width, height int = screen.viewport.Width, screen.viewport.Height
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, height)
			{
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
		}()
		screen.viewport = func() diffscreenDiffViewport {
			var (
				viewport diffscreenDiffViewport = screen.viewport
				columns  int                    = leftColumn
			)
			columns = max(-viewport.LeftColumn, min(columns, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if next.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range next.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()-viewport.LeftColumn))
			viewport.LeftColumn += columns
			{
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
		}()
		translated, ok := func() (diffscreenDiffCursor, bool) {
			if !cursor.valid {
				return diffscreenDiffCursor{}, false
			}
			best := diffscreenCursorMatch{}
			for y := range next.rows {
				candidate := func() diffscreenCursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patchFile = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == cursor.file.OldPath || candidate.NewPath != "" && candidate.NewPath == cursor.file.NewPath
					}()) {
						return diffscreenCursorMatch{}
					}
					best := diffscreenCursorMatch{}
					for _, pane := range [2]diffscreenDiffPane{cursor.cursor.pane, func() diffscreenDiffPane {
						if cursor.cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()} {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
						best = func() diffscreenCursorMatch {
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
			translated, _ = func() (diffscreenDiffCursor, bool) {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					step := int(diffscreenPrivateForward)
					for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
						if cursor, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return cursor, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(); ok {
					return cursor, true
				}
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
						return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
							var current diffscreenEntry = next.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !next.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}()
		}
		screen.cursor = translated
		screen.selection = func() *diffscreenDiffSelection {
			var selection *diffscreenDiffSelection = screen.selection
			if selection == nil {
				return nil
			}
			first := func() diffscreenCursorIdentity {
				var cursor diffscreenDiffCursor = selection.First
				if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
					var current diffscreenEntry = old.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenCursorIdentity{}
				}
				row := old.rows[cursor.row]
				file := old.patch.Files[row.file]
				hunk := file.Hunks[row.hunk]
				line := hunk.Lines[func() int {
					if row.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return row.rightLine
					}
					return row.leftLine
				}()]
				return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
			}()
			last := func() diffscreenCursorIdentity {
				var cursor diffscreenDiffCursor = selection.Last
				if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
					var current diffscreenEntry = old.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenCursorIdentity{}
				}
				row := old.rows[cursor.row]
				file := old.patch.Files[row.file]
				hunk := file.Hunks[row.hunk]
				line := hunk.Lines[func() int {
					if row.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return row.rightLine
					}
					return row.leftLine
				}()]
				return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
			}()
			if !first.valid || !last.valid {
				return nil
			}
			translatedFirst, firstOK := func() (diffscreenDiffCursor, bool) {
				if !first.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == first.file.OldPath || candidate.NewPath != "" && candidate.NewPath == first.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{first.cursor.pane, func() diffscreenDiffPane {
							if first.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
			translatedLast, lastOK := func() (diffscreenDiffCursor, bool) {
				if !last.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == last.file.OldPath || candidate.NewPath != "" && candidate.NewPath == last.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{last.cursor.pane, func() diffscreenDiffPane {
							if last.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
			translated := diffscreenDiffSelection{First: translatedFirst, Last: translatedFirst}
			translated, ok := func() (diffscreenDiffSelection, bool) {
				var selection diffscreenDiffSelection = translated
				if !(selection.First.row >= 0 && selection.First.row < len(next.rows) && func() int {
					var (
						current diffscreenEntry    = next.rows[selection.First.row]
						pane    diffscreenDiffPane = selection.First.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(translatedLast.row >= 0 && translatedLast.row < len(next.rows) && func() int {
					var current diffscreenEntry = next.rows[translatedLast.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || translatedLast.pane == diffscreenRight {
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
			var cursor diffscreenDiffCursor = screen.cursor
			return cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
				var current diffscreenEntry = next.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !next.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0
		}() {
			screen.viewport.top = max(0, screen.cursor.row-rowsAbove)
			screen.viewport = func() diffscreenDiffViewport {
				var (
					viewport diffscreenDiffViewport = screen.viewport
					cursor   diffscreenDiffCursor   = screen.cursor
				)
				if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
					var current diffscreenEntry = next.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
							if current.kind != diffscreenLineRow {
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
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(next.rows) && next.rows[viewport.top].kind != diffscreenFileRow {
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
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(next.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
								if current.kind != diffscreenLineRow {
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
			if translated, ok := func() (diffscreenDiffCursor, bool) {
				if !origin.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == origin.file.OldPath || candidate.NewPath != "" && candidate.NewPath == origin.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{origin.cursor.pane, func() diffscreenDiffPane {
							if origin.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
					var cursor diffscreenDiffCursor = screen.search.from
					screen.cursor = cursor
					screen.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
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
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
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
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
										if current.kind != diffscreenLineRow {
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
				cursor, ok := func(v *diffscreenDiffView, query string, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
					if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return diffscreenDiffCursor{}, false
					}
					query = strings.ToLower(query)
					y := cursor.row
					for count := 0; count < len(v.rows)-1; count++ {
						y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							var query string = query
							if v.rows[y].kind != diffscreenLineRow {
								if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
									var (
										y    int                = y
										pane diffscreenDiffPane = cursor.pane
									)
									for distance := 1; distance <= len(v.rows); distance++ {
										for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
											if candidate, ok := func() (diffscreenDiffCursor, bool) {
												if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
													return diffscreenDiffCursor{}, false
												}
												if candidate, ok := func() (diffscreenDiffCursor, bool) {
													cursor := diffscreenDiffCursor{row: candidateY, pane: pane}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}(); ok {
													return candidate, true
												}
												if v.split {
													cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
														if pane == diffscreenLeft {
															return diffscreenRight
														}
														return diffscreenLeft
													}()}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}
												return diffscreenDiffCursor{}, false
											}(); ok {
												return candidate, true
											}
										}
									}
									return diffscreenDiffCursor{}, false
								}
								return diffscreenDiffCursor{}, false
							}
							panes := []diffscreenDiffPane{cursor.pane}
							if v.split {
								panes = append(panes, func() diffscreenDiffPane {
									if cursor.pane == diffscreenLeft {
										return diffscreenRight
									}
									return diffscreenLeft
								}())
							}
							for _, candidatePane := range panes {
								candidate, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}()
								if !ok {
									continue
								}
								line, _ := func() (patchLine, bool) {
									if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[candidate.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || candidate.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0) {
										return patchLine{}, false
									}
									current := v.rows[candidate.row]
									return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || candidate.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}()], true
								}()
								if strings.Contains(strings.ToLower(line.Text), query) {
									return candidate, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(); ok {
							return candidate, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(screen.view, query, screen.search.from)
				screen.search.miss = !ok
				if ok {
					screen.cursor = cursor
					screen.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
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
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
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
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
										if current.kind != diffscreenLineRow {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() bool {
		var cursor diffscreenDiffCursor = preserved.cursor
		return cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
			var current diffscreenEntry = next.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0
	}(), true, "%s",
		"cursor was not preserved",
	)

	line, ok := func() (patchLine, bool) {
		var cursor diffscreenDiffCursor = preserved.cursor
		if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
			var current diffscreenEntry = next.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchLine{}, false
		}
		current := next.rows[cursor.row]
		return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "cursor line = %#v, ok=%v", line, ok)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line.Text, "added one", "cursor line = %#v, ok=%v",
		line, ok,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, preserved.selection != nil, true,
		"selection = %#v", preserved.selection,
	)

	firstLine, firstOK := func() (patchLine, bool) {
		var cursor diffscreenDiffCursor = preserved.selection.First
		if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
			var current diffscreenEntry = next.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchLine{}, false
		}
		current := next.rows[cursor.row]
		return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	lastLine, lastOK := func() (patchLine, bool) {
		var cursor diffscreenDiffCursor = preserved.selection.Last
		if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
			var current diffscreenEntry = next.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchLine{}, false
		}
		current := next.rows[cursor.row]
		return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !next.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, firstOK, true, "selection endpoints = %#v, %#v",
		firstLine, lastLine,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lastOK, true, "selection endpoints = %#v, %#v",
		firstLine, lastLine,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, firstLine.Text, "before",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lastLine.Text, "added one",
		"selection endpoints = %#v, %#v", firstLine, lastLine,
	)

	got := preserved.cursor.row - preserved.viewport.top
	want := cursor.row - viewport.top
	if got != want {
		t.Fatalf("screen row = %d, want %d", got, want)
	}
}

func TestDiffscreenPreserveReturnsEmptyStateForEmptyView(t *testing.T) {
	old := (func(p patchPatch, dark bool) *diffscreenDiffView {
		var options diffscreenOptions = diffscreenOptions{Dark: dark}
		v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
		for fileIndex, file := range p.Files {
			var file patchFile = file
			v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
			for _, metadata := range file.Metadata {
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
			}
			highlighted := func() diffscreenPair {
				var file *patchFile = &file
				theme := diffscreenLightTheme
				if v.dark {
					theme = diffscreenDarkTheme
				}
				return diffscreenPair{Old: func() []string {
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
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
					if strings.HasPrefix(hunk.Header, "@@") {
						return hunk.Header
					}
					return "@@ " + hunk.Header
				}()})
				for _, indices := range func() [][2]int {
					rows := make([][2]int, 0, len(hunk.Lines))
					for index := 0; index < len(hunk.Lines); {
						if !v.split || hunk.Lines[index].Kind == patchContext {
							rows = append(rows, [2]int{index, index})
							index++
							continue
						}
						if hunk.Lines[index].Kind == patchAddition {
							rows = append(rows, [2]int{-1, index})
							index++
							continue
						}
						addedStart := func() int {
							var start int = index
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
								start++
							}
							return start
						}()
						end := func() int {
							var start int = addedStart
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
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
					row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
					func() {
						var row *diffscreenEntry = &row
						if !v.split {
							row.text = func() string {
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.leftLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, true)
	cursor := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current diffscreenEntry = old.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !old.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(diffscreenPrivateForward)
			for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
					return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
						var current diffscreenEntry = old.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !old.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffscreenDiffCursor{}, false
		}()
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	screen := &diffscreenView{
		view:      old,
		cursor:    cursor,
		selection: (func(value diffscreenDiffSelection) *diffscreenDiffSelection { return &value })(diffscreenDiffSelection{First: cursor, Last: cursor}),
		viewport: func() diffscreenDiffViewport {
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, 80), max(1, 10)
			{
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(old.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != diffscreenFileRow {
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
						if current.kind != diffscreenLineRow {
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
		var next *diffscreenDiffView = (func(p patchPatch, dark bool) *diffscreenDiffView {
			var options diffscreenOptions = diffscreenOptions{Dark: dark}
			v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
			for fileIndex, file := range p.Files {
				var file patchFile = file
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
				for _, metadata := range file.Metadata {
					v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
				}
				highlighted := func() diffscreenPair {
					var file *patchFile = &file
					theme := diffscreenLightTheme
					if v.dark {
						theme = diffscreenDarkTheme
					}
					return diffscreenPair{Old: func() []string {
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
					v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
						if strings.HasPrefix(hunk.Header, "@@") {
							return hunk.Header
						}
						return "@@ " + hunk.Header
					}()})
					for _, indices := range func() [][2]int {
						rows := make([][2]int, 0, len(hunk.Lines))
						for index := 0; index < len(hunk.Lines); {
							if !v.split || hunk.Lines[index].Kind == patchContext {
								rows = append(rows, [2]int{index, index})
								index++
								continue
							}
							if hunk.Lines[index].Kind == patchAddition {
								rows = append(rows, [2]int{-1, index})
								index++
								continue
							}
							addedStart := func() int {
								var start int = index
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
									start++
								}
								return start
							}()
							end := func() int {
								var start int = addedStart
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
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
						row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
						func() {
							var row *diffscreenEntry = &row
							if !v.split {
								row.text = func() string {
									var line patchLine = hunk.Lines[row.rightLine]
									if line.Kind == patchDeletion {
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
									var line patchLine = hunk.Lines[row.leftLine]
									if line.Kind == patchDeletion {
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
									var line patchLine = hunk.Lines[row.rightLine]
									if line.Kind == patchDeletion {
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
		})(patchPatch{}, true)
		old := screen.view
		leftColumn := screen.viewport.LeftColumn
		rowsAbove := screen.cursor.row - screen.viewport.top
		cursor := func() diffscreenCursorIdentity {
			var cursor diffscreenDiffCursor = screen.cursor
			if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[cursor.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		origin := func() diffscreenCursorIdentity {
			if !(screen.search.from.row >= 0 && screen.search.from.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[screen.search.from.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[screen.search.from.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || screen.search.from.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: screen.search.from, valid: true}
		}()
		screen.viewport = func() diffscreenDiffViewport {
			var width, height int = screen.viewport.Width, screen.viewport.Height
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, height)
			{
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
		}()
		screen.viewport = func() diffscreenDiffViewport {
			var (
				viewport diffscreenDiffViewport = screen.viewport
				columns  int                    = leftColumn
			)
			columns = max(-viewport.LeftColumn, min(columns, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if next.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range next.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()-viewport.LeftColumn))
			viewport.LeftColumn += columns
			{
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
		}()
		translated, ok := func() (diffscreenDiffCursor, bool) {
			if !cursor.valid {
				return diffscreenDiffCursor{}, false
			}
			best := diffscreenCursorMatch{}
			for y := range next.rows {
				candidate := func() diffscreenCursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patchFile = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == cursor.file.OldPath || candidate.NewPath != "" && candidate.NewPath == cursor.file.NewPath
					}()) {
						return diffscreenCursorMatch{}
					}
					best := diffscreenCursorMatch{}
					for _, pane := range [2]diffscreenDiffPane{cursor.cursor.pane, func() diffscreenDiffPane {
						if cursor.cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()} {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
						best = func() diffscreenCursorMatch {
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
			translated, _ = func() (diffscreenDiffCursor, bool) {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					step := int(diffscreenPrivateForward)
					for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
						if cursor, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return cursor, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(); ok {
					return cursor, true
				}
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
						return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
							var current diffscreenEntry = next.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !next.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}()
		}
		screen.cursor = translated
		screen.selection = func() *diffscreenDiffSelection {
			var selection *diffscreenDiffSelection = screen.selection
			if selection == nil {
				return nil
			}
			first := func() diffscreenCursorIdentity {
				var cursor diffscreenDiffCursor = selection.First
				if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
					var current diffscreenEntry = old.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenCursorIdentity{}
				}
				row := old.rows[cursor.row]
				file := old.patch.Files[row.file]
				hunk := file.Hunks[row.hunk]
				line := hunk.Lines[func() int {
					if row.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return row.rightLine
					}
					return row.leftLine
				}()]
				return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
			}()
			last := func() diffscreenCursorIdentity {
				var cursor diffscreenDiffCursor = selection.Last
				if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
					var current diffscreenEntry = old.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenCursorIdentity{}
				}
				row := old.rows[cursor.row]
				file := old.patch.Files[row.file]
				hunk := file.Hunks[row.hunk]
				line := hunk.Lines[func() int {
					if row.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return row.rightLine
					}
					return row.leftLine
				}()]
				return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
			}()
			if !first.valid || !last.valid {
				return nil
			}
			translatedFirst, firstOK := func() (diffscreenDiffCursor, bool) {
				if !first.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == first.file.OldPath || candidate.NewPath != "" && candidate.NewPath == first.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{first.cursor.pane, func() diffscreenDiffPane {
							if first.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
			translatedLast, lastOK := func() (diffscreenDiffCursor, bool) {
				if !last.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == last.file.OldPath || candidate.NewPath != "" && candidate.NewPath == last.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{last.cursor.pane, func() diffscreenDiffPane {
							if last.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
			translated := diffscreenDiffSelection{First: translatedFirst, Last: translatedFirst}
			translated, ok := func() (diffscreenDiffSelection, bool) {
				var selection diffscreenDiffSelection = translated
				if !(selection.First.row >= 0 && selection.First.row < len(next.rows) && func() int {
					var (
						current diffscreenEntry    = next.rows[selection.First.row]
						pane    diffscreenDiffPane = selection.First.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(translatedLast.row >= 0 && translatedLast.row < len(next.rows) && func() int {
					var current diffscreenEntry = next.rows[translatedLast.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || translatedLast.pane == diffscreenRight {
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
			var cursor diffscreenDiffCursor = screen.cursor
			return cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
				var current diffscreenEntry = next.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !next.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0
		}() {
			screen.viewport.top = max(0, screen.cursor.row-rowsAbove)
			screen.viewport = func() diffscreenDiffViewport {
				var (
					viewport diffscreenDiffViewport = screen.viewport
					cursor   diffscreenDiffCursor   = screen.cursor
				)
				if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
					var current diffscreenEntry = next.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
							if current.kind != diffscreenLineRow {
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
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(next.rows) && next.rows[viewport.top].kind != diffscreenFileRow {
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
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(next.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
								if current.kind != diffscreenLineRow {
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
			if translated, ok := func() (diffscreenDiffCursor, bool) {
				if !origin.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == origin.file.OldPath || candidate.NewPath != "" && candidate.NewPath == origin.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{origin.cursor.pane, func() diffscreenDiffPane {
							if origin.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
					var cursor diffscreenDiffCursor = screen.search.from
					screen.cursor = cursor
					screen.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
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
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
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
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
										if current.kind != diffscreenLineRow {
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
				cursor, ok := func(v *diffscreenDiffView, query string, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
					if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return diffscreenDiffCursor{}, false
					}
					query = strings.ToLower(query)
					y := cursor.row
					for count := 0; count < len(v.rows)-1; count++ {
						y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							var query string = query
							if v.rows[y].kind != diffscreenLineRow {
								if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
									var (
										y    int                = y
										pane diffscreenDiffPane = cursor.pane
									)
									for distance := 1; distance <= len(v.rows); distance++ {
										for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
											if candidate, ok := func() (diffscreenDiffCursor, bool) {
												if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
													return diffscreenDiffCursor{}, false
												}
												if candidate, ok := func() (diffscreenDiffCursor, bool) {
													cursor := diffscreenDiffCursor{row: candidateY, pane: pane}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}(); ok {
													return candidate, true
												}
												if v.split {
													cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
														if pane == diffscreenLeft {
															return diffscreenRight
														}
														return diffscreenLeft
													}()}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}
												return diffscreenDiffCursor{}, false
											}(); ok {
												return candidate, true
											}
										}
									}
									return diffscreenDiffCursor{}, false
								}
								return diffscreenDiffCursor{}, false
							}
							panes := []diffscreenDiffPane{cursor.pane}
							if v.split {
								panes = append(panes, func() diffscreenDiffPane {
									if cursor.pane == diffscreenLeft {
										return diffscreenRight
									}
									return diffscreenLeft
								}())
							}
							for _, candidatePane := range panes {
								candidate, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}()
								if !ok {
									continue
								}
								line, _ := func() (patchLine, bool) {
									if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[candidate.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || candidate.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0) {
										return patchLine{}, false
									}
									current := v.rows[candidate.row]
									return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || candidate.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}()], true
								}()
								if strings.Contains(strings.ToLower(line.Text), query) {
									return candidate, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(); ok {
							return candidate, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(screen.view, query, screen.search.from)
				screen.search.miss = !ok
				if ok {
					screen.cursor = cursor
					screen.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
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
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
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
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
										if current.kind != diffscreenLineRow {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !func() bool {
		var (
			v      *diffscreenDiffView  = preserved.view
			cursor diffscreenDiffCursor = preserved.cursor
		)
		return cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[cursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || cursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0
	}(), true,
		"state = %#v", preserved,
	)
	(func(t *testing.T, got, want *diffscreenDiffSelection, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, preserved.selection, nil, "state = %#v", preserved)
}

func TestDiffscreenPreserveClampsHorizontalOffsetForShorterLines(t *testing.T) {
	p := (func() patchPatch {
		lines := make([]patchLine, 20)
		for index := range lines {
			lines[index] = patchLine{
				Kind:      patchContext,
				Text:      strings.Repeat("long", 20),
				OldNumber: patchLineNumber(index + 1),
				NewNumber: patchLineNumber(index + 1),
			}
		}
		return patchPatch{
			Files: []patchFile{
				{
					DisplayPath: "long.go",
					Hunks:       []patchHunk{{Header: "@@", Lines: lines}},
				},
			},
		}
	})()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	old := (func(p patchPatch, dark bool) *diffscreenDiffView {
		var options diffscreenOptions = diffscreenOptions{Dark: dark}
		v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
		for fileIndex, file := range p.Files {
			var file patchFile = file
			v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
			for _, metadata := range file.Metadata {
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
			}
			highlighted := func() diffscreenPair {
				var file *patchFile = &file
				theme := diffscreenLightTheme
				if v.dark {
					theme = diffscreenDarkTheme
				}
				return diffscreenPair{Old: func() []string {
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
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
					if strings.HasPrefix(hunk.Header, "@@") {
						return hunk.Header
					}
					return "@@ " + hunk.Header
				}()})
				for _, indices := range func() [][2]int {
					rows := make([][2]int, 0, len(hunk.Lines))
					for index := 0; index < len(hunk.Lines); {
						if !v.split || hunk.Lines[index].Kind == patchContext {
							rows = append(rows, [2]int{index, index})
							index++
							continue
						}
						if hunk.Lines[index].Kind == patchAddition {
							rows = append(rows, [2]int{-1, index})
							index++
							continue
						}
						addedStart := func() int {
							var start int = index
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
								start++
							}
							return start
						}()
						end := func() int {
							var start int = addedStart
							for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
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
					row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
					func() {
						var row *diffscreenEntry = &row
						if !v.split {
							row.text = func() string {
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.leftLine]
								if line.Kind == patchDeletion {
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
								var line patchLine = hunk.Lines[row.rightLine]
								if line.Kind == patchDeletion {
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
	cursor := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
						return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
							var current diffscreenEntry = old.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !old.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(diffscreenPrivateForward)
			for y := -1 + step; y >= 0 && y < len(old.rows); y += step {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
					return cursor, cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
						var current diffscreenEntry = old.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !old.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0
				}(); ok {
					return cursor, true
				}
			}
			return diffscreenDiffCursor{}, false
		}()
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffscreenDiffViewport {
		var (
			viewport diffscreenDiffViewport = func() diffscreenDiffViewport {
				var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
				viewport.Width, viewport.Height = max(1, 40), max(1, 10)
				{
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(old.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != diffscreenFileRow {
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
							if current.kind != diffscreenLineRow {
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
				if current.kind != diffscreenLineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffscreenDiffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(old.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(old.rows) && old.rows[end].kind != diffscreenFileRow {
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
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, viewport.LeftColumn != 0, true, "%s",
		"fixture does not scroll horizontally",
	)

	short := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}
	preserved := &diffscreenView{view: old, cursor: cursor, viewport: viewport}
	{
		var next *diffscreenDiffView = (func(p patchPatch, dark bool) *diffscreenDiffView {
			var options diffscreenOptions = diffscreenOptions{Dark: dark}
			v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
			for fileIndex, file := range p.Files {
				var file patchFile = file
				v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
				for _, metadata := range file.Metadata {
					v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
				}
				highlighted := func() diffscreenPair {
					var file *patchFile = &file
					theme := diffscreenLightTheme
					if v.dark {
						theme = diffscreenDarkTheme
					}
					return diffscreenPair{Old: func() []string {
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
					v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
						if strings.HasPrefix(hunk.Header, "@@") {
							return hunk.Header
						}
						return "@@ " + hunk.Header
					}()})
					for _, indices := range func() [][2]int {
						rows := make([][2]int, 0, len(hunk.Lines))
						for index := 0; index < len(hunk.Lines); {
							if !v.split || hunk.Lines[index].Kind == patchContext {
								rows = append(rows, [2]int{index, index})
								index++
								continue
							}
							if hunk.Lines[index].Kind == patchAddition {
								rows = append(rows, [2]int{-1, index})
								index++
								continue
							}
							addedStart := func() int {
								var start int = index
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
									start++
								}
								return start
							}()
							end := func() int {
								var start int = addedStart
								for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
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
						row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
						func() {
							var row *diffscreenEntry = &row
							if !v.split {
								row.text = func() string {
									var line patchLine = hunk.Lines[row.rightLine]
									if line.Kind == patchDeletion {
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
									var line patchLine = hunk.Lines[row.leftLine]
									if line.Kind == patchDeletion {
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
									var line patchLine = hunk.Lines[row.rightLine]
									if line.Kind == patchDeletion {
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
		cursor := func() diffscreenCursorIdentity {
			var cursor diffscreenDiffCursor = preserved.cursor
			if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[cursor.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || cursor.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		origin := func() diffscreenCursorIdentity {
			if !(preserved.search.from.row >= 0 && preserved.search.from.row < len(old.rows) && func() int {
				var current diffscreenEntry = old.rows[preserved.search.from.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || preserved.search.from.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return diffscreenCursorIdentity{}
			}
			row := old.rows[preserved.search.from.row]
			file := old.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != diffscreenLineRow {
					return -1
				}
				if !old.split || preserved.search.from.pane == diffscreenRight {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: preserved.search.from, valid: true}
		}()
		preserved.viewport = func() diffscreenDiffViewport {
			var width, height int = preserved.viewport.Width, preserved.viewport.Height
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, height)
			{
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
		}()
		preserved.viewport = func() diffscreenDiffViewport {
			var (
				viewport diffscreenDiffViewport = preserved.viewport
				columns  int                    = leftColumn
			)
			columns = max(-viewport.LeftColumn, min(columns, func() int {
				contentWidth := max(1, viewport.Width-14)
				extra := 0
				if next.split {
					contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
				}
				longest := 0
				for _, current := range next.rows {
					if current.kind != diffscreenLineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()-viewport.LeftColumn))
			viewport.LeftColumn += columns
			{
				var viewport diffscreenDiffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(next.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
						if current.kind != diffscreenLineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}
		}()
		translated, ok := func() (diffscreenDiffCursor, bool) {
			if !cursor.valid {
				return diffscreenDiffCursor{}, false
			}
			best := diffscreenCursorMatch{}
			for y := range next.rows {
				candidate := func() diffscreenCursorMatch {
					current := next.rows[y]
					if !(current.hunk >= 0 && func() bool {
						var candidate patchFile = next.patch.Files[current.file]
						return candidate.OldPath != "" && candidate.OldPath == cursor.file.OldPath || candidate.NewPath != "" && candidate.NewPath == cursor.file.NewPath
					}()) {
						return diffscreenCursorMatch{}
					}
					best := diffscreenCursorMatch{}
					for _, pane := range [2]diffscreenDiffPane{cursor.cursor.pane, func() diffscreenDiffPane {
						if cursor.cursor.pane == diffscreenLeft {
							return diffscreenRight
						}
						return diffscreenLeft
					}()} {
						candidate, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: pane}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}()
						if !ok {
							continue
						}
						line, _ := func() (patchLine, bool) {
							if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[candidate.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := next.rows[candidate.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || candidate.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
						best = func() diffscreenCursorMatch {
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
			translated, _ = func() (diffscreenDiffCursor, bool) {
				if cursor, ok := func() (diffscreenDiffCursor, bool) {
					step := int(diffscreenPrivateForward)
					for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
						if cursor, ok := func() (diffscreenDiffCursor, bool) {
							cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
							return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[cursor.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0
						}(); ok {
							return cursor, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(); ok {
					return cursor, true
				}
				step := int(diffscreenPrivateForward)
				for y := -1 + step; y >= 0 && y < len(next.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
						return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
							var current diffscreenEntry = next.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !next.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}(); ok {
						return cursor, true
					}
				}
				return diffscreenDiffCursor{}, false
			}()
		}
		preserved.cursor = translated
		preserved.selection = func() *diffscreenDiffSelection {
			var selection *diffscreenDiffSelection = preserved.selection
			if selection == nil {
				return nil
			}
			first := func() diffscreenCursorIdentity {
				var cursor diffscreenDiffCursor = selection.First
				if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
					var current diffscreenEntry = old.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenCursorIdentity{}
				}
				row := old.rows[cursor.row]
				file := old.patch.Files[row.file]
				hunk := file.Hunks[row.hunk]
				line := hunk.Lines[func() int {
					if row.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return row.rightLine
					}
					return row.leftLine
				}()]
				return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
			}()
			last := func() diffscreenCursorIdentity {
				var cursor diffscreenDiffCursor = selection.Last
				if !(cursor.row >= 0 && cursor.row < len(old.rows) && func() int {
					var current diffscreenEntry = old.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return diffscreenCursorIdentity{}
				}
				row := old.rows[cursor.row]
				file := old.patch.Files[row.file]
				hunk := file.Hunks[row.hunk]
				line := hunk.Lines[func() int {
					if row.kind != diffscreenLineRow {
						return -1
					}
					if !old.split || cursor.pane == diffscreenRight {
						return row.rightLine
					}
					return row.leftLine
				}()]
				return diffscreenCursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
			}()
			if !first.valid || !last.valid {
				return nil
			}
			translatedFirst, firstOK := func() (diffscreenDiffCursor, bool) {
				if !first.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == first.file.OldPath || candidate.NewPath != "" && candidate.NewPath == first.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{first.cursor.pane, func() diffscreenDiffPane {
							if first.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
			translatedLast, lastOK := func() (diffscreenDiffCursor, bool) {
				if !last.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == last.file.OldPath || candidate.NewPath != "" && candidate.NewPath == last.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{last.cursor.pane, func() diffscreenDiffPane {
							if last.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
			translated := diffscreenDiffSelection{First: translatedFirst, Last: translatedFirst}
			translated, ok := func() (diffscreenDiffSelection, bool) {
				var selection diffscreenDiffSelection = translated
				if !(selection.First.row >= 0 && selection.First.row < len(next.rows) && func() int {
					var (
						current diffscreenEntry    = next.rows[selection.First.row]
						pane    diffscreenDiffPane = selection.First.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(translatedLast.row >= 0 && translatedLast.row < len(next.rows) && func() int {
					var current diffscreenEntry = next.rows[translatedLast.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || translatedLast.pane == diffscreenRight {
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
			var cursor diffscreenDiffCursor = preserved.cursor
			return cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
				var current diffscreenEntry = next.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !next.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0
		}() {
			preserved.viewport.top = max(0, preserved.cursor.row-rowsAbove)
			preserved.viewport = func() diffscreenDiffViewport {
				var (
					viewport diffscreenDiffViewport = preserved.viewport
					cursor   diffscreenDiffCursor   = preserved.cursor
				)
				if !(cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
					var current diffscreenEntry = next.rows[cursor.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !next.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
							if current.kind != diffscreenLineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}
				viewport = func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(next.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
							if current.kind != diffscreenLineRow {
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
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(next.rows) && next.rows[viewport.top].kind != diffscreenFileRow {
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
					viewport = func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(next.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(next.rows) && next.rows[end].kind != diffscreenFileRow {
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
								if current.kind != diffscreenLineRow {
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
			if translated, ok := func() (diffscreenDiffCursor, bool) {
				if !origin.valid {
					return diffscreenDiffCursor{}, false
				}
				best := diffscreenCursorMatch{}
				for y := range next.rows {
					candidate := func() diffscreenCursorMatch {
						current := next.rows[y]
						if !(current.hunk >= 0 && func() bool {
							var candidate patchFile = next.patch.Files[current.file]
							return candidate.OldPath != "" && candidate.OldPath == origin.file.OldPath || candidate.NewPath != "" && candidate.NewPath == origin.file.NewPath
						}()) {
							return diffscreenCursorMatch{}
						}
						best := diffscreenCursorMatch{}
						for _, pane := range [2]diffscreenDiffPane{origin.cursor.pane, func() diffscreenDiffPane {
							if origin.cursor.pane == diffscreenLeft {
								return diffscreenRight
							}
							return diffscreenLeft
						}()} {
							candidate, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: y, pane: pane}
								return cursor, cursor.row >= 0 && cursor.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[cursor.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || cursor.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0
							}()
							if !ok {
								continue
							}
							line, _ := func() (patchLine, bool) {
								if !(candidate.row >= 0 && candidate.row < len(next.rows) && func() int {
									var current diffscreenEntry = next.rows[candidate.row]
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}() >= 0) {
									return patchLine{}, false
								}
								current := next.rows[candidate.row]
								return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
									if current.kind != diffscreenLineRow {
										return -1
									}
									if !next.split || candidate.pane == diffscreenRight {
										return current.rightLine
									}
									return current.leftLine
								}()], true
							}()
							match := diffscreenCursorMatch{cursor: candidate, rank: func() int {
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
							best = func() diffscreenCursorMatch {
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
					var cursor diffscreenDiffCursor = preserved.search.from
					preserved.cursor = cursor
					preserved.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
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
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
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
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
										if current.kind != diffscreenLineRow {
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
				cursor, ok := func(v *diffscreenDiffView, query string, cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
					if query == "" || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[cursor.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return diffscreenDiffCursor{}, false
					}
					query = strings.ToLower(query)
					y := cursor.row
					for count := 0; count < len(v.rows)-1; count++ {
						y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
						if candidate, ok := func() (diffscreenDiffCursor, bool) {
							var query string = query
							if v.rows[y].kind != diffscreenLineRow {
								if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
									var (
										y    int                = y
										pane diffscreenDiffPane = cursor.pane
									)
									for distance := 1; distance <= len(v.rows); distance++ {
										for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
											if candidate, ok := func() (diffscreenDiffCursor, bool) {
												if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
													return diffscreenDiffCursor{}, false
												}
												if candidate, ok := func() (diffscreenDiffCursor, bool) {
													cursor := diffscreenDiffCursor{row: candidateY, pane: pane}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}(); ok {
													return candidate, true
												}
												if v.split {
													cursor := diffscreenDiffCursor{row: candidateY, pane: func() diffscreenDiffPane {
														if pane == diffscreenLeft {
															return diffscreenRight
														}
														return diffscreenLeft
													}()}
													return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
														var current diffscreenEntry = v.rows[cursor.row]
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || cursor.pane == diffscreenRight {
															return current.rightLine
														}
														return current.leftLine
													}() >= 0
												}
												return diffscreenDiffCursor{}, false
											}(); ok {
												return candidate, true
											}
										}
									}
									return diffscreenDiffCursor{}, false
								}
								return diffscreenDiffCursor{}, false
							}
							panes := []diffscreenDiffPane{cursor.pane}
							if v.split {
								panes = append(panes, func() diffscreenDiffPane {
									if cursor.pane == diffscreenLeft {
										return diffscreenRight
									}
									return diffscreenLeft
								}())
							}
							for _, candidatePane := range panes {
								candidate, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: candidatePane}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}()
								if !ok {
									continue
								}
								line, _ := func() (patchLine, bool) {
									if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[candidate.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || candidate.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0) {
										return patchLine{}, false
									}
									current := v.rows[candidate.row]
									return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || candidate.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}()], true
								}()
								if strings.Contains(strings.ToLower(line.Text), query) {
									return candidate, true
								}
							}
							return diffscreenDiffCursor{}, false
						}(); ok {
							return candidate, true
						}
					}
					return diffscreenDiffCursor{}, false
				}(preserved.view, query, preserved.search.from)
				preserved.search.miss = !ok
				if ok {
					preserved.cursor = cursor
					preserved.viewport = func(v *diffscreenDiffView, viewport diffscreenDiffViewport) diffscreenDiffViewport {
						if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[cursor.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}
						viewport = func() diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
									if current.kind != diffscreenLineRow {
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
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
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
							viewport = func() diffscreenDiffViewport {
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
										if current.kind != diffscreenLineRow {
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
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, preserved.viewport.LeftColumn, 0,
		"horizontal offset = %d", preserved.viewport.LeftColumn,
	)
}

func TestDiffscreenUpdatePreservesCursorAfterLineDeletion(t *testing.T) {
	original, target, cases := func() (original patchPatch, target patchLine, cases []diffscreenCursorDeletionCase) {
		early := patchHunk{Header: "@@ -1,0 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "early change", NewNumber: 1}}}
		before := patchLine{Kind: patchContext, Text: "before", OldNumber: 50, NewNumber: 50}
		added := patchLine{Kind: patchAddition, Text: "removed addition", NewNumber: 51}
		after := patchLine{Kind: patchContext, Text: "after", OldNumber: 51, NewNumber: 52}
		target = patchLine{Kind: patchAddition, Text: "target", NewNumber: 53}
		late := patchHunk{Header: "@@ -100,0 +100 @@", Lines: []patchLine{{Kind: patchAddition, Text: "late change", NewNumber: 100}}}
		original = patchPatch{Files: []patchFile{{OldPath: "file.go", NewPath: "file.go", Hunks: []patchHunk{early, {Header: "@@ -50,2 +50,4 @@", Lines: []patchLine{before, added, after, target}}, late}}}}
		shiftedAfter, shiftedTarget := after, target
		shiftedAfter.NewNumber--
		shiftedTarget.NewNumber--
		cases = []diffscreenCursorDeletionCase{{"line before cursor deleted", []patchHunk{early, {Header: "@@ -50,2 +50,3 @@", Lines: []patchLine{before, shiftedAfter, shiftedTarget}}, late}, shiftedTarget}, {"cursor line deleted", []patchHunk{early, {Header: "@@ -50,2 +50,3 @@", Lines: []patchLine{before, added, after}}, late}, after}, {"cursor hunk removed", []patchHunk{early, late}, late.Lines[0]}}
		return original, target, cases
	}()
	for _, split := range []bool{false, true} {
		for _, tc := range cases {
			t.Run(
				fmt.Sprintf("%s/split=%v", tc.name, split),
				func(t *testing.T) {
					v := (func(p patchPatch, options diffscreenOptions) *diffscreenView {
						v := &diffscreenView{options: options, width: 80, height: 30}
						v.view = func() *diffscreenDiffView {
							{
								var options diffscreenOptions = diffscreenOptions{SideBySide: v.options.SideBySide && v.width >= 100, Dark: v.options.Dark}
								v := &diffscreenDiffView{patch: p, split: options.SideBySide, dark: options.Dark}
								for fileIndex, file := range p.Files {
									var file patchFile = file
									v.rows = append(v.rows, diffscreenEntry{kind: diffscreenFileRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: file.DisplayPath})
									for _, metadata := range file.Metadata {
										v.rows = append(v.rows, diffscreenEntry{kind: diffscreenMetadataRow, file: fileIndex, hunk: -1, leftLine: -1, rightLine: -1, text: metadata})
									}
									highlighted := func() diffscreenPair {
										var file *patchFile = &file
										theme := diffscreenLightTheme
										if v.dark {
											theme = diffscreenDarkTheme
										}
										return diffscreenPair{Old: func() []string {
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
										v.rows = append(v.rows, diffscreenEntry{kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex, leftLine: -1, rightLine: -1, text: func() string {
											if strings.HasPrefix(hunk.Header, "@@") {
												return hunk.Header
											}
											return "@@ " + hunk.Header
										}()})
										for _, indices := range func() [][2]int {
											rows := make([][2]int, 0, len(hunk.Lines))
											for index := 0; index < len(hunk.Lines); {
												if !v.split || hunk.Lines[index].Kind == patchContext {
													rows = append(rows, [2]int{index, index})
													index++
													continue
												}
												if hunk.Lines[index].Kind == patchAddition {
													rows = append(rows, [2]int{-1, index})
													index++
													continue
												}
												addedStart := func() int {
													var start int = index
													for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchDeletion {
														start++
													}
													return start
												}()
												end := func() int {
													var start int = addedStart
													for start < len(hunk.Lines) && hunk.Lines[start].Kind == patchAddition {
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
											row := diffscreenEntry{kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex, leftLine: indices[0], rightLine: indices[1]}
											func() {
												var row *diffscreenEntry = &row
												if !v.split {
													row.text = func() string {
														var line patchLine = hunk.Lines[row.rightLine]
														if line.Kind == patchDeletion {
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
														var line patchLine = hunk.Lines[row.leftLine]
														if line.Kind == patchDeletion {
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
														var line patchLine = hunk.Lines[row.rightLine]
														if line.Kind == patchDeletion {
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
						v.viewport = func(v *diffscreenDiffView, width, height int) diffscreenDiffViewport {
							var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
							viewport.Width, viewport.Height = max(1, width), max(1, height)
							{
								var viewport diffscreenDiffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != diffscreenFileRow {
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
										if current.kind != diffscreenLineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}
						}(v.view, v.width, (func(height int) int { return max(1, height-3) })(v.height))
						v.cursor, _ = func() (diffscreenDiffCursor, bool) {
							var v *diffscreenDiffView = v.view
							if cursor, ok := func() (diffscreenDiffCursor, bool) {
								step := int(diffscreenPrivateForward)
								for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
									if cursor, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: y, pane: diffscreenRight}
										return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
											var current diffscreenEntry = v.rows[cursor.row]
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || cursor.pane == diffscreenRight {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return cursor, true
									}
								}
								return diffscreenDiffCursor{}, false
							}(); ok {
								return cursor, true
							}
							step := int(diffscreenPrivateForward)
							for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
								if cursor, ok := func() (diffscreenDiffCursor, bool) {
									cursor := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
									return cursor, cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
										var current diffscreenEntry = v.rows[cursor.row]
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || cursor.pane == diffscreenRight {
											return current.rightLine
										}
										return current.leftLine
									}() >= 0
								}(); ok {
									return cursor, true
								}
							}
							return diffscreenDiffCursor{}, false
						}()
						return v
					})(original, diffscreenOptions{SideBySide: split})
					v.Resize(120, 4)
					v.BeginSearch()
					v.InsertSearch("target")
					v.AcceptSearch()
					_, line, ok := v.Current()
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, ok, true,
						"initial cursor = %#v, ok=%v", line, ok,
					)
					(func(t *testing.T, got, want patchLine, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, line, target,
						"initial cursor = %#v, ok=%v", line, ok,
					)

					v.Update(patchPatch{Files: []patchFile{{OldPath: "file.go", NewPath: "file.go", Hunks: tc.hunks}}})
					_, line, ok = v.Current()
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, ok, true,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					(func(t *testing.T, got, want patchLine, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, line, tc.want,
						"restored cursor = %#v, ok=%v, want %#v", line, ok,
						tc.want,
					)
					t.Helper()
					inlineLocalTop1 := v.viewport.top
					inlineLocalBottom1 := inlineLocalTop1 + func(v *diffscreenDiffView, viewport diffscreenDiffViewport) int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}(v.view, v.viewport)
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, v.cursor.row >= inlineLocalTop1, true, "cursor row %d is above viewport %d:%d", v.cursor.row, inlineLocalTop1, inlineLocalBottom1)
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, v.cursor.row < inlineLocalBottom1, true, "cursor row %d is below viewport %d:%d", v.cursor.row, inlineLocalTop1, inlineLocalBottom1)
				},
			)
		}
	}
}

type diffscreenCursorDeletionCase struct {
	name  string
	hunks []patchHunk
	want  patchLine
}
