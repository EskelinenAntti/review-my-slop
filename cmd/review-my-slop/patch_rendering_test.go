package main

import (
	bytes "bytes"
	fmt "fmt"
	io "io"
	regexp "regexp"
	strconv "strconv"
	strings "strings"
	testing "testing"

	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestDiffscreenSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(t *testing.T) {
	p := patchPatch{
		Files: []patchFile{{DisplayPath: "file", Hunks: []patchHunk{
			{
				Header: "one",
				Lines: []patchLine{
					{Kind: patchDeletion, Text: "d1", OldNumber: 1},
					{Kind: patchDeletion, Text: "d2", OldNumber: 2},
					{Kind: patchAddition, Text: "a1", NewNumber: 1},
					{Kind: patchAddition, Text: "a2", NewNumber: 2},
					{Kind: patchAddition, Text: "a3", NewNumber: 3},
					{
						Kind:      patchContext,
						Text:      "c",
						OldNumber: 3,
						NewNumber: 4,
					},
				},
			},
			{
				Header: "two",
				Lines: []patchLine{
					{Kind: patchAddition, Text: "separate", NewNumber: 5},
				},
			},
		}}},
	}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(p, true)
	var code []diffscreenEntry
	for _, current := range v.rows {
		if current.kind == diffscreenLineRow {
			code = append(code, current)
		}
	}
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(code), 5, "visual code entries = %d",
		len(code),
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[0].leftLine, 0, "pairing = %#v", code[:3])
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[0].rightLine, 2, "pairing = %#v", code[:3])
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[1].leftLine, 1, "pairing = %#v", code[:3])
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[1].rightLine, 3, "pairing = %#v", code[:3])
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[2].leftLine, -1, "pairing = %#v", code[:3])
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[2].rightLine, 4, "pairing = %#v", code[:3])
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[4].hunk != code[3].hunk, true,
		"hunks were paired: %#v", code[4],
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, code[4].leftLine, -1, "hunks were paired: %#v",
		code[4],
	)
}

func TestDiffscreenSplitSelectionOnlyIncludesActivePane(t *testing.T) {
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	first := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	removed, _ := func() (diffscreenDiffCursor, bool) {
		var query string = "removed one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if v.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: first.pane}
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
											if first.pane == diffscreenLeft {
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
				panes := []diffscreenDiffPane{first.pane}
				if v.split {
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
	}()
	added, _ := func() (diffscreenDiffCursor, bool) {
		var query string = "added one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if v.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: first.pane}
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
											if first.pane == diffscreenLeft {
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
				panes := []diffscreenDiffPane{first.pane}
				if v.split {
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
	}()
	inlineLocalLeft2 := diffscreenDiffSelection{First: removed, Last: removed}
	inlineLocalLeft2, ok := func() (diffscreenDiffSelection, bool) {
		var selection diffscreenDiffSelection = inlineLocalLeft2
		if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
			var (
				current diffscreenEntry    = v.rows[selection.First.row]
				pane    diffscreenDiffPane = selection.First.pane
			)
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || !(removed.row >= 0 && removed.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[removed.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || removed.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return selection, false
		}
		first := v.rows[selection.First.row]
		last := v.rows[removed.row]
		if first.file != last.file || first.hunk != last.hunk {
			return selection, false
		}
		selection.Last = removed
		return selection, true
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "left lines = %#v", func() []patchLine {
		var selection diffscreenDiffSelection = inlineLocalLeft2
		if _, ok := func() (diffscreenDiffSelection, bool) {
			var selection diffscreenDiffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current diffscreenEntry    = v.rows[selection.First.row]
					pane    diffscreenDiffPane = selection.First.pane
				)
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[selection.Last.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || selection.Last.pane == diffscreenRight {
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
			first, _ := func() (patchLine, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.First.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patchLine, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patchLine{first}
			}
			return []patchLine{first, last}
		}
		lines := make([]patchLine, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffscreenDiffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patchLine, bool) {
				var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
					return patchLine{}, false
				}
				current := v.rows[cursor.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}())
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(func() []patchLine {
		var selection diffscreenDiffSelection = inlineLocalLeft2
		if _, ok := func() (diffscreenDiffSelection, bool) {
			var selection diffscreenDiffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current diffscreenEntry    = v.rows[selection.First.row]
					pane    diffscreenDiffPane = selection.First.pane
				)
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[selection.Last.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || selection.Last.pane == diffscreenRight {
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
			first, _ := func() (patchLine, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.First.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patchLine, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patchLine{first}
			}
			return []patchLine{first, last}
		}
		lines := make([]patchLine, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffscreenDiffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patchLine, bool) {
				var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
					return patchLine{}, false
				}
				current := v.rows[cursor.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}()), 1, "left lines = %#v",
		func() []patchLine {
			var selection diffscreenDiffSelection = inlineLocalLeft2
			if _, ok := func() (diffscreenDiffSelection, bool) {
				var selection diffscreenDiffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current diffscreenEntry    = v.rows[selection.First.row]
						pane    diffscreenDiffPane = selection.First.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
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
				first, _ := func() (patchLine, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.First.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patchLine, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.Last.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patchLine{first}
				}
				return []patchLine{first, last}
			}
			lines := make([]patchLine, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffscreenDiffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patchLine, bool) {
					var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
						return patchLine{}, false
					}
					current := v.rows[cursor.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}(),
	)
	(func(t *testing.T, got, want patchLineKind, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() []patchLine {
		var selection diffscreenDiffSelection = inlineLocalLeft2
		if _, ok := func() (diffscreenDiffSelection, bool) {
			var selection diffscreenDiffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current diffscreenEntry    = v.rows[selection.First.row]
					pane    diffscreenDiffPane = selection.First.pane
				)
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[selection.Last.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || selection.Last.pane == diffscreenRight {
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
			first, _ := func() (patchLine, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.First.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patchLine, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patchLine{first}
			}
			return []patchLine{first, last}
		}
		lines := make([]patchLine, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffscreenDiffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patchLine, bool) {
				var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
					return patchLine{}, false
				}
				current := v.rows[cursor.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}()[0].Kind, patchDeletion, "left lines = %#v", func() []patchLine {
		var selection diffscreenDiffSelection = inlineLocalLeft2
		if _, ok := func() (diffscreenDiffSelection, bool) {
			var selection diffscreenDiffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current diffscreenEntry    = v.rows[selection.First.row]
					pane    diffscreenDiffPane = selection.First.pane
				)
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[selection.Last.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || selection.Last.pane == diffscreenRight {
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
			first, _ := func() (patchLine, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.First.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.First.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patchLine, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patchLine{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patchLine{first}
			}
			return []patchLine{first, last}
		}
		lines := make([]patchLine, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffscreenDiffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patchLine, bool) {
				var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
					return patchLine{}, false
				}
				current := v.rows[cursor.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || cursor.pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}(),
	)

	inlineLocalRight2 := diffscreenDiffSelection{First: added, Last: added}
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len((func(v *diffscreenDiffView) func(selection diffscreenDiffSelection) []patchLine {
		return func(selection diffscreenDiffSelection) []patchLine {
			if _, ok := func() (diffscreenDiffSelection, bool) {
				var selection diffscreenDiffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current diffscreenEntry    = v.rows[selection.First.row]
						pane    diffscreenDiffPane = selection.First.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
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
			first, last := min(
				selection.First.row,
				selection.Last.row,
			), max(selection.First.row, selection.Last.row)
			if first == last && selection.First.pane != selection.Last.pane {
				first, _ := func() (patchLine, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.First.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patchLine, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.Last.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patchLine{first}
				}
				return []patchLine{first, last}
			}
			lines := make([]patchLine, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffscreenDiffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patchLine, bool) {
					var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
						return patchLine{}, false
					}
					current := v.rows[cursor.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}
	}(v))(inlineLocalRight2)), 1, "right lines = %#v",
		(func(v *diffscreenDiffView) func(selection diffscreenDiffSelection) []patchLine {
			return func(selection diffscreenDiffSelection) []patchLine {
				if _, ok := func() (diffscreenDiffSelection, bool) {
					var selection diffscreenDiffSelection = selection
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current diffscreenEntry    = v.rows[selection.First.row]
							pane    diffscreenDiffPane = selection.First.pane
						)
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.Last.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
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
				first, last := min(
					selection.First.row,
					selection.Last.row,
				), max(selection.First.row, selection.Last.row)
				if first == last && selection.First.pane != selection.Last.pane {
					first, _ := func() (patchLine, bool) {
						if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[selection.First.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || selection.First.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patchLine{}, false
						}
						current := v.rows[selection.First.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || selection.First.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					last, _ := func() (patchLine, bool) {
						if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
							var current diffscreenEntry = v.rows[selection.Last.row]
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || selection.Last.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patchLine{}, false
						}
						current := v.rows[selection.Last.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || selection.Last.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if first == last {
						return []patchLine{first}
					}
					return []patchLine{first, last}
				}
				lines := make([]patchLine, 0, last-first+1)
				for y := first; y <= last; y++ {
					pane := func() diffscreenDiffPane {
						if y == selection.Last.row {
							return selection.Last.pane
						}
						return selection.First.pane
					}()
					if line, ok := func() (patchLine, bool) {
						var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
							return patchLine{}, false
						}
						current := v.rows[cursor.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != diffscreenLineRow {
								return -1
							}
							if !v.split || cursor.pane == diffscreenRight {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}(); ok {
						lines = append(lines, line)
					}
				}
				return lines
			}
		}(v))(inlineLocalRight2),
	)
	(func(t *testing.T, got, want patchLineKind, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, (func(v *diffscreenDiffView) func(selection diffscreenDiffSelection) []patchLine {
		return func(selection diffscreenDiffSelection) []patchLine {
			if _, ok := func() (diffscreenDiffSelection, bool) {
				var selection diffscreenDiffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current diffscreenEntry    = v.rows[selection.First.row]
						pane    diffscreenDiffPane = selection.First.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
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
			first, last := min(
				selection.First.row,
				selection.Last.row,
			), max(selection.First.row, selection.Last.row)
			if first == last && selection.First.pane != selection.Last.pane {
				first, _ := func() (patchLine, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.First.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patchLine, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.Last.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patchLine{first}
				}
				return []patchLine{first, last}
			}
			lines := make([]patchLine, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffscreenDiffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patchLine, bool) {
					var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
						return patchLine{}, false
					}
					current := v.rows[cursor.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}
	}(v))(inlineLocalRight2)[0].Kind, patchAddition, "right lines = %#v", (func(v *diffscreenDiffView) func(selection diffscreenDiffSelection) []patchLine {
		return func(selection diffscreenDiffSelection) []patchLine {
			if _, ok := func() (diffscreenDiffSelection, bool) {
				var selection diffscreenDiffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current diffscreenEntry    = v.rows[selection.First.row]
						pane    diffscreenDiffPane = selection.First.pane
					)
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || pane == diffscreenRight {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current diffscreenEntry = v.rows[selection.Last.row]
					if current.kind != diffscreenLineRow {
						return -1
					}
					if !v.split || selection.Last.pane == diffscreenRight {
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
			first, last := min(
				selection.First.row,
				selection.Last.row,
			), max(selection.First.row, selection.Last.row)
			if first == last && selection.First.pane != selection.Last.pane {
				first, _ := func() (patchLine, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.First.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.First.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patchLine, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current diffscreenEntry = v.rows[selection.Last.row]
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patchLine{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || selection.Last.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patchLine{first}
				}
				return []patchLine{first, last}
			}
			lines := make([]patchLine, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffscreenDiffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patchLine, bool) {
					var cursor diffscreenDiffCursor = diffscreenDiffCursor{row: y, pane: pane}
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
						return patchLine{}, false
					}
					current := v.rows[cursor.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != diffscreenLineRow {
							return -1
						}
						if !v.split || cursor.pane == diffscreenRight {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}
	}(v))(inlineLocalRight2),
	)
}

func TestDiffscreenSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "file",
				Hunks: []patchHunk{{Header: "@@", Lines: []patchLine{
					{Kind: patchAddition, Text: "right", NewNumber: 1},
					{
						Kind:      patchContext,
						Text:      "both",
						OldNumber: 1,
						NewNumber: 2,
					},
					{Kind: patchDeletion, Text: "left", OldNumber: 2},
				}}},
			},
		},
	}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(p, true)
	first := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	leftCursor, ok := func() (diffscreenDiffCursor, bool) {
		if !v.split || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			var y int = first.row
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
			return candidate, true
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			for y := first.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[first.row].file {
					break
				}
				if candidate, ok := func() (diffscreenDiffCursor, bool) {
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
					return candidate, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(diffscreenPrivateForward)
		for y := first.row + step; y >= 0 && y < len(v.rows); y += step {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "%s", "did not find later left pane")

	line, _ := func() (patchLine, bool) {
		if !(leftCursor.row >= 0 && leftCursor.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[leftCursor.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || leftCursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patchLine{}, false
		}
		current := v.rows[leftCursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || leftCursor.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line.Text, "both", "later left line = %q",
		line.Text,
	)

	last, _ := func() (diffscreenDiffCursor, bool) {
		if cursor, ok := func() (diffscreenDiffCursor, bool) {
			var start int = len(v.rows)
			step := int(diffscreenPrivateBackward)
			for y := start + step; y >= 0 && y < len(v.rows); y += step {
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
		var start int = len(v.rows)
		step := int(diffscreenPrivateBackward)
		for y := start + step; y >= 0 && y < len(v.rows); y += step {
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
	inlineLocalRight1, ok := func() (diffscreenDiffCursor, bool) {
		if !v.split || !(last.row >= 0 && last.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[last.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || last.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			var y int = last.row
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
			return candidate, true
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			for y := last.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[last.row].file {
					break
				}
				if candidate, ok := func() (diffscreenDiffCursor, bool) {
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
					return candidate, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(diffscreenPrivateForward)
		for y := last.row + step; y >= 0 && y < len(v.rows); y += step {
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
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "%s", "did not find earlier right pane")

	line, _ = (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
		return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
				return patchLine{}, false
			}
			current := v.rows[cursor.row]
			return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}()], true
		}
	}(v))(inlineLocalRight1)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line.Text, "both", "earlier right line = %q",
		line.Text,
	)
}

func TestDiffscreenSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "file",
				Hunks: []patchHunk{
					{
						Header: "@@",
						Lines: []patchLine{
							{Kind: patchAddition, Text: "one", NewNumber: 1},
							{Kind: patchAddition, Text: "two", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(p, true)
	cursor := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	if _, ok := func() (diffscreenDiffCursor, bool) {
		if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
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
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			var y int = cursor.row
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
			return candidate, true
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			for y := cursor.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[cursor.row].file {
					break
				}
				if candidate, ok := func() (diffscreenDiffCursor, bool) {
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
					return candidate, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(diffscreenPrivateForward)
		for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
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
	}(); ok {
		t.Fatal("switched to empty pane")
	}
}

func TestDiffscreenSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "file",
				Hunks: []patchHunk{
					{
						Header: "@@",
						Lines: []patchLine{
							{
								Kind:      patchContext,
								Text:      "one",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patchAddition, Text: "right", NewNumber: 2},
							{
								Kind:      patchContext,
								Text:      "two",
								OldNumber: 2,
								NewNumber: 3,
							},
							{Kind: patchDeletion, Text: "left", OldNumber: 3},
							{
								Kind:      patchContext,
								Text:      "three",
								OldNumber: 4,
								NewNumber: 4,
							},
						},
					},
				},
			},
		},
	}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(p, true)
	first := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	inlineLocalLeft1, _ := func() (diffscreenDiffCursor, bool) {
		if !v.split || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			var y int = first.row
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
			return candidate, true
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			for y := first.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[first.row].file {
					break
				}
				if candidate, ok := func() (diffscreenDiffCursor, bool) {
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
					return candidate, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(diffscreenPrivateForward)
		for y := first.row + step; y >= 0 && y < len(v.rows); y += step {
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
	nextLeft, _ := func() (diffscreenDiffCursor, bool) {
		if !(inlineLocalLeft1.row >= 0 && inlineLocalLeft1.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[inlineLocalLeft1.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || inlineLocalLeft1.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		step := int(diffscreenPrivateForward)
		for y := inlineLocalLeft1.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				cursor := diffscreenDiffCursor{row: y, pane: inlineLocalLeft1.pane}
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
	line, _ := (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
		return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
				return patchLine{}, false
			}
			current := v.rows[cursor.row]
			return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}()], true
		}
	}(v))(nextLeft)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line.Text, "two", "left movement = %q", line.Text)

	rightAtContext, _ := func() (diffscreenDiffCursor, bool) {
		if !v.split || !(nextLeft.row >= 0 && nextLeft.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[nextLeft.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || nextLeft.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			var y int = nextLeft.row
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
			return candidate, true
		}
		if candidate, ok := func() (diffscreenDiffCursor, bool) {
			for y := nextLeft.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[nextLeft.row].file {
					break
				}
				if candidate, ok := func() (diffscreenDiffCursor, bool) {
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
					return candidate, true
				}
			}
			return diffscreenDiffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(diffscreenPrivateForward)
		for y := nextLeft.row + step; y >= 0 && y < len(v.rows); y += step {
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
	}()
	nextRight, _ := func() (diffscreenDiffCursor, bool) {
		if !(rightAtContext.row >= 0 && rightAtContext.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[rightAtContext.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || rightAtContext.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		step := int(diffscreenPrivateForward)
		for y := rightAtContext.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffscreenDiffCursor, bool) {
				cursor := diffscreenDiffCursor{row: y, pane: rightAtContext.pane}
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
	line, _ = (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
		return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
				return patchLine{}, false
			}
			current := v.rows[cursor.row]
			return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}()], true
		}
	}(v))(nextRight)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line.Text, "three", "right movement = %q",
		line.Text,
	)
}

func TestDiffscreenSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patchLine{
		{
			Kind:      patchDeletion,
			Text:      "d1",
			OldNumber: 1,
		}, {
			Kind:      patchAddition,
			Text:      "a1",
			NewNumber: 1,
		}, {Kind: patchContext, Text: "c1", OldNumber: 2, NewNumber: 2},
		{
			Kind:      patchDeletion,
			Text:      "d2",
			OldNumber: 3,
		}, {
			Kind:      patchAddition,
			Text:      "a2",
			NewNumber: 3,
		}, {Kind: patchContext, Text: "c2", OldNumber: 4, NewNumber: 4},
		{
			Kind:      patchDeletion,
			Text:      "d3",
			OldNumber: 5,
		}, {
			Kind:      patchAddition,
			Text:      "a3",
			NewNumber: 5,
		}, {Kind: patchContext, Text: "c3", OldNumber: 6, NewNumber: 6},
	}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "file",
				Hunks:       []patchHunk{{Header: "@@", Lines: lines}},
			},
		},
	}, true,
	)
	cursor := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, 120), max(1, 4)
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
	}()
	viewport = func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = viewport
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
	}()
	originalTop := viewport.top
	viewport, moved := func() (diffscreenDiffViewport, diffscreenDiffCursor) {
		var (
			viewport diffscreenDiffViewport = viewport
			cursor   diffscreenDiffCursor   = cursor
		)
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var (
				current diffscreenEntry    = v.rows[cursor.row]
				pane    diffscreenDiffPane = cursor.pane
			)
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return viewport, cursor
		}
		distance := int(diffscreenPrivateForward) * max(1, viewport.Height/2)
		viewport.top += distance
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
		target := min(len(v.rows)-1, max(0, cursor.row+distance))
		candidate, ok := func() (diffscreenDiffCursor, bool) {
			var (
				pane     diffscreenDiffPane     = cursor.pane
				viewport diffscreenDiffViewport = viewport
			)
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}()
			for distance := 0; distance < height; distance++ {
				for _, y := range []int{target + int(diffscreenPrivateForward)*distance, target - int(diffscreenPrivateForward)*distance} {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						if y < viewport.top || y >= viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								height--
							}
							return max(1, height)
						}() {
							return diffscreenDiffCursor{}, false
						}
						cursor := diffscreenDiffCursor{row: y, pane: pane}
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
			}
			return diffscreenDiffCursor{}, false
		}()
		if ok {
			cursor = candidate
		}
		return viewport, cursor
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, viewport.top > originalTop, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, moved.row > cursor.row, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)

	viewport, moved = func() (diffscreenDiffViewport, diffscreenDiffCursor) {
		var (
			viewport diffscreenDiffViewport = viewport
			cursor   diffscreenDiffCursor   = moved
		)
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var (
				current diffscreenEntry    = v.rows[cursor.row]
				pane    diffscreenDiffPane = cursor.pane
			)
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return viewport, cursor
		}
		distance := int(diffscreenPrivateBackward) * max(1, viewport.Height/2)
		viewport.top += distance
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
		target := min(len(v.rows)-1, max(0, cursor.row+distance))
		candidate, ok := func() (diffscreenDiffCursor, bool) {
			var (
				pane     diffscreenDiffPane     = cursor.pane
				viewport diffscreenDiffViewport = viewport
			)
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}()
			for distance := 0; distance < height; distance++ {
				for _, y := range []int{target + int(diffscreenPrivateBackward)*distance, target - int(diffscreenPrivateBackward)*distance} {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						if y < viewport.top || y >= viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								height--
							}
							return max(1, height)
						}() {
							return diffscreenDiffCursor{}, false
						}
						cursor := diffscreenDiffCursor{row: y, pane: pane}
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
			}
			return diffscreenDiffCursor{}, false
		}()
		if ok {
			cursor = candidate
		}
		return viewport, cursor
	}()
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, viewport.top, originalTop,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, moved.row, cursor.row,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
}

func TestDiffscreenFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := func() patchPatch {
		p := patchPatch{Files: []patchFile{{DisplayPath: "first.go", Hunks: []patchHunk{{Header: "@@", Lines: []patchLine{{Kind: patchContext, Text: "first one", OldNumber: 1, NewNumber: 1}, {Kind: patchContext, Text: "first two", OldNumber: 2, NewNumber: 2}, {Kind: patchContext, Text: "first three", OldNumber: 3, NewNumber: 3}}}}}, {DisplayPath: "second.go", Hunks: []patchHunk{{Header: "@@", Lines: []patchLine{{Kind: patchContext, Text: "second one", OldNumber: 1, NewNumber: 1}}}}}}}
		return p
	}()
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	viewport := func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, 60), max(1, 3)
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
	}()
	viewport.top = 3

	rendered := strings.Split(
		ansi.Strip((func(v *diffscreenDiffView) func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
			return func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == diffscreenLineRow {
						lines = append(
							lines,
							(func(v *diffscreenDiffView) func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								return func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || diffscreenLeft == diffscreenRight {
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
										if diffscreenLeft == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffscreenDiffView) func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										return func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || pane == diffscreenRight {
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
											if pane == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: pane}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										diffscreenRight,
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
								case diffscreenFileRow:
									return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
								case diffscreenMetadataRow:
									return diffscreenMetadataStyle.Render("  " + current.text)
								case diffscreenHunkRow:
									return diffscreenHunkStyle.Render(current.text)
								case diffscreenLineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+")
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-")
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
										}(), diffscreenDiscardForeground
									}
									if cursor.row == y {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, diffscreenDiffCursor{}, nil)),
		"\n",
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(rendered), viewport.Height,
		"sticky render = %#v", rendered,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(rendered[0], "first.go"), true,
		"sticky render = %#v", rendered,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(rendered[1], "first two"), true,
		"sticky header covered content rows: %#v", rendered,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(rendered[2], "first three"), true,
		"sticky header covered content rows: %#v", rendered,
	)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip((func(v *diffscreenDiffView) func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
			return func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == diffscreenLineRow {
						lines = append(
							lines,
							(func(v *diffscreenDiffView) func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								return func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || diffscreenLeft == diffscreenRight {
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
										if diffscreenLeft == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffscreenDiffView) func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										return func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || pane == diffscreenRight {
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
											if pane == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: pane}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										diffscreenRight,
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
								case diffscreenFileRow:
									return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
								case diffscreenMetadataRow:
									return diffscreenMetadataStyle.Render("  " + current.text)
								case diffscreenHunkRow:
									return diffscreenHunkStyle.Render(current.text)
								case diffscreenLineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+")
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-")
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
										}(), diffscreenDiscardForeground
									}
									if cursor.row == y {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, diffscreenDiffCursor{}, nil)),
		"\n",
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Count(
		strings.Join(rendered, "\n"),
		"second.go",
	), 1,
		"file header was duplicated at its natural position: %#v", rendered)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(
		ansi.Strip((func(v *diffscreenDiffView) func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
			return func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == diffscreenLineRow {
						lines = append(
							lines,
							(func(v *diffscreenDiffView) func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								return func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || diffscreenLeft == diffscreenRight {
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
										if diffscreenLeft == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffscreenDiffView) func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										return func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || pane == diffscreenRight {
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
											if pane == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: pane}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										diffscreenRight,
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
								case diffscreenFileRow:
									return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
								case diffscreenMetadataRow:
									return diffscreenMetadataStyle.Render("  " + current.text)
								case diffscreenHunkRow:
									return diffscreenHunkStyle.Render(current.text)
								case diffscreenLineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+")
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-")
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
										}(), diffscreenDiscardForeground
									}
									if cursor.row == y {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, diffscreenDiffCursor{}, nil)),
		"\n",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(rendered[0], "second.go"), true,
		"sticky header did not change with the file: %#v", rendered,
	)
}

func TestDiffscreenKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})((func() patchPatch {
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
	})(), true)
	cursor, _ := func() (diffscreenDiffCursor, bool) {
		if cursor, ok := func() (diffscreenDiffCursor, bool) {
			var start int = len(v.rows)
			step := int(diffscreenPrivateBackward)
			for y := start + step; y >= 0 && y < len(v.rows); y += step {
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
		var start int = len(v.rows)
		step := int(diffscreenPrivateBackward)
		for y := start + step; y >= 0 && y < len(v.rows); y += step {
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
	viewport := func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = func() diffscreenDiffViewport {
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, 50), max(1, 4)
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
		}()
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
	}()
	rendered := ansi.Strip((func(v *diffscreenDiffView) func(
		viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
		return func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == diffscreenLineRow {
					lines = append(
						lines,
						(func(v *diffscreenDiffView) func(
							current diffscreenEntry, y int,
							viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
							return func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || diffscreenLeft == diffscreenRight {
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
									if diffscreenLeft == diffscreenLeft {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+") + " "
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-") + " "
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
										}(), diffscreenDiscardForeground
									}
									if cursor == candidate {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffscreenDiffView) func(
									current diffscreenEntry, y int,
									pane diffscreenDiffPane, width, offset int,
									cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									return func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || pane == diffscreenRight {
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
										if pane == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: pane}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									diffscreenRight,
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
							case diffscreenFileRow:
								return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
							case diffscreenMetadataRow:
								return diffscreenMetadataStyle.Render("  " + current.text)
							case diffscreenHunkRow:
								return diffscreenHunkStyle.Render(current.text)
							case diffscreenLineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patchAddition {
									prefix = diffscreenAddedStyle.Render("+")
								}
								if line.Kind == patchDeletion {
									prefix = diffscreenRemovedStyle.Render("-")
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
									case patchAddition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patchDeletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return diffscreenContextStyle
									}
								}()
								colors := diffscreenPreserveForeground
								candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
									}(), diffscreenDiscardForeground
								}
								if cursor.row == y {
									style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
								}
								{
									var (
										style  lipgloss.Style             = style
										value  string                     = value
										colors diffscreenForegroundPolicy = colors
									)
									value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	}(v))(viewport, cursor, nil))
	line, _ := (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
		return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
				return patchLine{}, false
			}
			current := v.rows[cursor.row]
			return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}()], true
		}
	}(v))(cursor)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		cursor.row < viewport.top+func() int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				height--
			}
			return max(1, height)
		}(), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
}

func TestDiffscreenSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "file",
				Hunks: []patchHunk{
					{
						Header: "@@",
						Lines: []patchLine{
							{
								Kind:      patchContext,
								Text:      "\t\tlong line",
								OldNumber: 1,
								NewNumber: 1,
							},
						},
					},
				},
			},
		},
	}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(p, true)
	cursor := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	rendered := ansi.Strip((func(
		v *diffscreenDiffView, cursor diffscreenDiffCursor, width int,
		selection *diffscreenDiffSelection) string {
		return (func(
			v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
			selection *diffscreenDiffSelection) string {
			viewport := func() diffscreenDiffViewport {
				var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
				viewport.Width, viewport.Height = max(1, width), max(1, 1)
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
			}()
			viewport.top = target.row
			return (func(v *diffscreenDiffView) func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
				return func(
					viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
					lines := make([]string, 0, viewport.Height)
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						current := v.rows[viewport.top]
						lines = append(
							lines,
							func() string {
								var path string = v.patch.Files[current.file].DisplayPath
								return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
							}(),
						)
					}
					end := min(len(v.rows), viewport.top+func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}())
					for y := viewport.top; y < end; y++ {
						current := v.rows[y]
						if v.split && current.kind == diffscreenLineRow {
							lines = append(
								lines,
								(func(v *diffscreenDiffView) func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									return func(
										current diffscreenEntry, y int,
										viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
											inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
											return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
										}()
										inlineLocalLeft1 := func() string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || diffscreenLeft == diffscreenRight {
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
											if diffscreenLeft == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										inlineLocalRight1 := (func(v *diffscreenDiffView) func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											return func(
												current diffscreenEntry, y int,
												pane diffscreenDiffPane, width, offset int,
												cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
												index := func() int {
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || pane == diffscreenRight {
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
												if pane == diffscreenLeft {
													text, numberValue = current.left, line.OldNumber
												}
												prefix := "  "
												if line.Kind == patchAddition {
													prefix = diffscreenAddedStyle.Render("+") + " "
												}
												if line.Kind == patchDeletion {
													prefix = diffscreenRemovedStyle.Render("-") + " "
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
													case patchAddition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patchDeletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return diffscreenContextStyle
													}
												}()
												colors := diffscreenPreserveForeground
												candidate := diffscreenDiffCursor{row: y, pane: pane}
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
													}(), diffscreenDiscardForeground
												}
												if cursor == candidate {
													style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
												}
												{
													var (
														style  lipgloss.Style             = style
														value  string                     = value
														colors diffscreenForegroundPolicy = colors
													)
													value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
											diffscreenRight,
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
									case diffscreenFileRow:
										return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
									case diffscreenMetadataRow:
										return diffscreenMetadataStyle.Render("  " + current.text)
									case diffscreenHunkRow:
										return diffscreenHunkStyle.Render(current.text)
									case diffscreenLineRow:
										hunk := v.patch.Files[current.file].Hunks[current.hunk]
										line := hunk.Lines[current.rightLine]
										prefix := " "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+")
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-")
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
											}(), diffscreenDiscardForeground
										}
										if cursor.row == y {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
			}(v))(viewport, active, selection)
		})(v, cursor, cursor, width, selection)
	})(v, cursor, 120, nil))
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.ContainsRune(rendered, '\t'), true,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Index(rendered, "│"), 59,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lipgloss.Width(rendered), 120,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
}

func TestDiffscreenHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := patchPatch{Files: []patchFile{{DisplayPath: "long", Hunks: []patchHunk{{Header: "@@", Lines: []patchLine{{Kind: patchContext, Text: "abcdefghijklmnopqrstuvwxyz" + "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}}}}}}}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, 37), max(1, 1)
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
	}()
	viewport = func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = viewport
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
	}()
	before := ansi.Strip((func(v *diffscreenDiffView) func(
		viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
		return func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == diffscreenLineRow {
					lines = append(
						lines,
						(func(v *diffscreenDiffView) func(
							current diffscreenEntry, y int,
							viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
							return func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || diffscreenLeft == diffscreenRight {
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
									if diffscreenLeft == diffscreenLeft {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+") + " "
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-") + " "
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
										}(), diffscreenDiscardForeground
									}
									if cursor == candidate {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffscreenDiffView) func(
									current diffscreenEntry, y int,
									pane diffscreenDiffPane, width, offset int,
									cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									return func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || pane == diffscreenRight {
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
										if pane == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: pane}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									diffscreenRight,
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
							case diffscreenFileRow:
								return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
							case diffscreenMetadataRow:
								return diffscreenMetadataStyle.Render("  " + current.text)
							case diffscreenHunkRow:
								return diffscreenHunkStyle.Render(current.text)
							case diffscreenLineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patchAddition {
									prefix = diffscreenAddedStyle.Render("+")
								}
								if line.Kind == patchDeletion {
									prefix = diffscreenRemovedStyle.Render("-")
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
									case patchAddition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patchDeletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return diffscreenContextStyle
									}
								}()
								colors := diffscreenPreserveForeground
								candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
									}(), diffscreenDiscardForeground
								}
								if cursor.row == y {
									style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
								}
								{
									var (
										style  lipgloss.Style             = style
										value  string                     = value
										colors diffscreenForegroundPolicy = colors
									)
									value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	}(v))(viewport, cursor, nil))
	viewport = func() diffscreenDiffViewport {
		var (
			viewport diffscreenDiffViewport = viewport
			columns  int                    = 4
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
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
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
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
	}()
	after := ansi.Strip((func(v *diffscreenDiffView) func(
		viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
		return func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == diffscreenLineRow {
					lines = append(
						lines,
						(func(v *diffscreenDiffView) func(
							current diffscreenEntry, y int,
							viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
							return func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || diffscreenLeft == diffscreenRight {
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
									if diffscreenLeft == diffscreenLeft {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+") + " "
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-") + " "
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
										}(), diffscreenDiscardForeground
									}
									if cursor == candidate {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffscreenDiffView) func(
									current diffscreenEntry, y int,
									pane diffscreenDiffPane, width, offset int,
									cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									return func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || pane == diffscreenRight {
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
										if pane == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: pane}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									diffscreenRight,
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
							case diffscreenFileRow:
								return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
							case diffscreenMetadataRow:
								return diffscreenMetadataStyle.Render("  " + current.text)
							case diffscreenHunkRow:
								return diffscreenHunkStyle.Render(current.text)
							case diffscreenLineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patchAddition {
									prefix = diffscreenAddedStyle.Render("+")
								}
								if line.Kind == patchDeletion {
									prefix = diffscreenRemovedStyle.Render("-")
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
									case patchAddition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patchDeletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return diffscreenContextStyle
									}
								}()
								colors := diffscreenPreserveForeground
								candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
									}(), diffscreenDiscardForeground
								}
								if cursor.row == y {
									style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
								}
								{
									var (
										style  lipgloss.Style             = style
										value  string                     = value
										colors diffscreenForegroundPolicy = colors
									)
									value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	}(v))(viewport, cursor, nil))
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, before[:14], after[:14],
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(after[14:], "efghij"), true,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, viewport.LeftColumn, 4,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
}

func TestDiffscreenHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(patchPatch{Files: []patchFile{{DisplayPath: "long", Hunks: []patchHunk{{Header: "@@", Lines: []patchLine{{Kind: patchContext, Text: "abcdefghijklmnopqrstuvwxyz" + "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}}}}}}}, true)
	cursor := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, 120), max(1, 1)
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
	}()
	viewport = func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = viewport
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
	}()
	viewport = func() diffscreenDiffViewport {
		var (
			viewport diffscreenDiffViewport = viewport
			columns  int                    = 8
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
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
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
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
	}()
	rendered := ansi.Strip((func(v *diffscreenDiffView) func(
		viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
		return func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == diffscreenLineRow {
					lines = append(
						lines,
						(func(v *diffscreenDiffView) func(
							current diffscreenEntry, y int,
							viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
							return func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != diffscreenLineRow {
											return -1
										}
										if !v.split || diffscreenLeft == diffscreenRight {
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
									if diffscreenLeft == diffscreenLeft {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+") + " "
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-") + " "
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
										}(), diffscreenDiscardForeground
									}
									if cursor == candidate {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffscreenDiffView) func(
									current diffscreenEntry, y int,
									pane diffscreenDiffPane, width, offset int,
									cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									return func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || pane == diffscreenRight {
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
										if pane == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: pane}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									diffscreenRight,
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
							case diffscreenFileRow:
								return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
							case diffscreenMetadataRow:
								return diffscreenMetadataStyle.Render("  " + current.text)
							case diffscreenHunkRow:
								return diffscreenHunkStyle.Render(current.text)
							case diffscreenLineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patchAddition {
									prefix = diffscreenAddedStyle.Render("+")
								}
								if line.Kind == patchDeletion {
									prefix = diffscreenRemovedStyle.Render("-")
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
									case patchAddition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patchDeletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return diffscreenContextStyle
									}
								}()
								colors := diffscreenPreserveForeground
								candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
									}(), diffscreenDiscardForeground
								}
								if cursor.row == y {
									style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
								}
								{
									var (
										style  lipgloss.Style             = style
										value  string                     = value
										colors diffscreenForegroundPolicy = colors
									)
									value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	}(v))(viewport, cursor, nil))
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Index(rendered, "│"), 59,
		"gutters moved: %q", rendered,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, rendered[:6], "    1 ", "gutters moved: %q",
		rendered,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, rendered[63:69], "    1 ", "gutters moved: %q",
		rendered,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(rendered[6:59], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(rendered[69:], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
}

func TestDiffscreenHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(patchPatch{Files: []patchFile{{DisplayPath: "long", Hunks: []patchHunk{{Header: "@@", Lines: []patchLine{{Kind: patchContext, Text: "abcdefghijklmnopqrstuvwxyz" + "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}}}}}}}, true)
	viewport := func() diffscreenDiffViewport {
		var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
		viewport.Width, viewport.Height = max(1, 37), max(1, 1)
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
	}()
	viewport = func() diffscreenDiffViewport {
		var (
			viewport diffscreenDiffViewport = viewport
			columns  int                    = int(^uint(0) >> 1)
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
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
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
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
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, viewport.LeftColumn != 0, true, "%s",
		"end did not scroll",
	)

	viewport = func() diffscreenDiffViewport {
		var (
			viewport diffscreenDiffViewport = viewport
			columns  int                    = -int(^uint(0) >> 1)
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
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
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
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
	}()
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, viewport.LeftColumn, 0, "start = %d",
		viewport.LeftColumn,
	)
}

func TestDiffscreenDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	added, _ := func() (diffscreenDiffCursor, bool) {
		var query string = "added one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if v.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: first.pane}
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
											if first.pane == diffscreenLeft {
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
				panes := []diffscreenDiffPane{first.pane}
				if v.split {
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
					line, _ := (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
						return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
								return patchLine{}, false
							}
							current := v.rows[cursor.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}
					}(v))(candidate)
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
	removed, _ := func() (diffscreenDiffCursor, bool) {
		var query string = "removed one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if v.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: first.pane}
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
											if first.pane == diffscreenLeft {
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
				panes := []diffscreenDiffPane{first.pane}
				if v.split {
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
					line, _ := (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
						return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
								return patchLine{}, false
							}
							current := v.rows[cursor.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}
					}(v))(candidate)
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
	addedRender := (func(
		v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
		selection *diffscreenDiffSelection) string {
		viewport := func() diffscreenDiffViewport {
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, 1)
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
		}()
		viewport.top = target.row
		return (func(v *diffscreenDiffView) func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
			return func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == diffscreenLineRow {
						lines = append(
							lines,
							(func(v *diffscreenDiffView) func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								return func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || diffscreenLeft == diffscreenRight {
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
										if diffscreenLeft == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffscreenDiffView) func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										return func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || pane == diffscreenRight {
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
											if pane == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: pane}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										diffscreenRight,
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
								case diffscreenFileRow:
									return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
								case diffscreenMetadataRow:
									return diffscreenMetadataStyle.Render("  " + current.text)
								case diffscreenHunkRow:
									return diffscreenHunkStyle.Render(current.text)
								case diffscreenLineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+")
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-")
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
										}(), diffscreenDiscardForeground
									}
									if cursor.row == y {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, active, selection)
	})(v, added, first, 80, nil)
	removedRender := (func(
		v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
		selection *diffscreenDiffSelection) string {
		viewport := func() diffscreenDiffViewport {
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, 1)
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
		}()
		viewport.top = target.row
		return (func(v *diffscreenDiffView) func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
			return func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == diffscreenLineRow {
						lines = append(
							lines,
							(func(v *diffscreenDiffView) func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								return func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || diffscreenLeft == diffscreenRight {
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
										if diffscreenLeft == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffscreenDiffView) func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										return func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || pane == diffscreenRight {
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
											if pane == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: pane}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										diffscreenRight,
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
								case diffscreenFileRow:
									return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
								case diffscreenMetadataRow:
									return diffscreenMetadataStyle.Render("  " + current.text)
								case diffscreenHunkRow:
									return diffscreenHunkStyle.Render(current.text)
								case diffscreenLineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+")
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-")
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
										}(), diffscreenDiscardForeground
									}
									if cursor.row == y {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, active, selection)
	})(v, removed, first, 80, nil)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(addedRender, "\x1b[32m+\x1b[m"),
		true, "added=%q removed=%q", addedRender, removedRender,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(removedRender, "\x1b[31m-\x1b[m"),
		true, "added=%q removed=%q", addedRender, removedRender,
	)

	var (
		rendered string = (func(
			v *diffscreenDiffView, cursor diffscreenDiffCursor, width int,
			selection *diffscreenDiffSelection) string {
			return (func(
				v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
				selection *diffscreenDiffSelection) string {
				viewport := func() diffscreenDiffViewport {
					var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
					viewport.Width, viewport.Height = max(1, width), max(1, 1)
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
				}()
				viewport.top = target.row
				return (func(v *diffscreenDiffView) func(
					viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
					return func(
						viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
						lines := make([]string, 0, viewport.Height)
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							current := v.rows[viewport.top]
							lines = append(
								lines,
								func() string {
									var path string = v.patch.Files[current.file].DisplayPath
									return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
								}(),
							)
						}
						end := min(len(v.rows), viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								height--
							}
							return max(1, height)
						}())
						for y := viewport.top; y < end; y++ {
							current := v.rows[y]
							if v.split && current.kind == diffscreenLineRow {
								lines = append(
									lines,
									(func(v *diffscreenDiffView) func(
										current diffscreenEntry, y int,
										viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										return func(
											current diffscreenEntry, y int,
											viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
												inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
												return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
											}()
											inlineLocalLeft1 := func() string {
												index := func() int {
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || diffscreenLeft == diffscreenRight {
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
												if diffscreenLeft == diffscreenLeft {
													text, numberValue = current.left, line.OldNumber
												}
												prefix := "  "
												if line.Kind == patchAddition {
													prefix = diffscreenAddedStyle.Render("+") + " "
												}
												if line.Kind == patchDeletion {
													prefix = diffscreenRemovedStyle.Render("-") + " "
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
													case patchAddition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patchDeletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return diffscreenContextStyle
													}
												}()
												colors := diffscreenPreserveForeground
												candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
													}(), diffscreenDiscardForeground
												}
												if cursor == candidate {
													style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
												}
												{
													var (
														style  lipgloss.Style             = style
														value  string                     = value
														colors diffscreenForegroundPolicy = colors
													)
													value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
											inlineLocalRight1 := (func(v *diffscreenDiffView) func(
												current diffscreenEntry, y int,
												pane diffscreenDiffPane, width, offset int,
												cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
												return func(
													current diffscreenEntry, y int,
													pane diffscreenDiffPane, width, offset int,
													cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
													index := func() int {
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || pane == diffscreenRight {
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
													if pane == diffscreenLeft {
														text, numberValue = current.left, line.OldNumber
													}
													prefix := "  "
													if line.Kind == patchAddition {
														prefix = diffscreenAddedStyle.Render("+") + " "
													}
													if line.Kind == patchDeletion {
														prefix = diffscreenRemovedStyle.Render("-") + " "
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
														case patchAddition:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
														case patchDeletion:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
														default:
															return diffscreenContextStyle
														}
													}()
													colors := diffscreenPreserveForeground
													candidate := diffscreenDiffCursor{row: y, pane: pane}
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
														}(), diffscreenDiscardForeground
													}
													if cursor == candidate {
														style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
													}
													{
														var (
															style  lipgloss.Style             = style
															value  string                     = value
															colors diffscreenForegroundPolicy = colors
														)
														value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																	if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
																	if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
												diffscreenRight,
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
										case diffscreenFileRow:
											return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
										case diffscreenMetadataRow:
											return diffscreenMetadataStyle.Render("  " + current.text)
										case diffscreenHunkRow:
											return diffscreenHunkStyle.Render(current.text)
										case diffscreenLineRow:
											hunk := v.patch.Files[current.file].Hunks[current.hunk]
											line := hunk.Lines[current.rightLine]
											prefix := " "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+")
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-")
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
												}(), diffscreenDiscardForeground
											}
											if cursor.row == y {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
				}(v))(viewport, active, selection)
			})(v, cursor, cursor, width, selection)
		})(v, added, 80, nil)
		expected diffscreenSgrExpectation = diffscreenSgrExpectation{reverse: true}
	)
	t.Helper()
	state := diffscreenSgrState{}
	column := 0
	for len(rendered) > 0 {
		location := diffscreenSgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			column = func() int {
				var (
					text   string             = rendered
					column int                = column
					state  diffscreenSgrState = state
				)
				t.Helper()
				for range text {
					column++
					var column int = column
					t.Helper()
					(func(t *testing.T, got, want string, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
				}
				return column
			}()
			break
		}
		column = func() int {
			var (
				text   string             = rendered[:location[0]]
				column int                = column
				state  diffscreenSgrState = state
			)
			t.Helper()
			for range text {
				column++
				var column int = column
				t.Helper()
				(func(t *testing.T, got, want string, format string, args ...any) {
					t.Helper()
					if got != want {
						t.Fatalf(format, args...)
					}
				})(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
				(func(t *testing.T, got, want bool, format string, args ...any) {
					t.Helper()
					if got != want {
						t.Fatalf(format, args...)
					}
				})(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
			}
			return column
		}()
		var (
			s          *diffscreenSgrState = &state
			parameters string              = rendered[location[2]:location[3]]
		)
		if parameters == "" {
			parameters = "0"
		}
		values := strings.Split(parameters, ";")
		for index := 0; index < len(values); index++ {
			value, _ := strconv.Atoi(values[index])
			switch value {
			case 0:
				*s = diffscreenSgrState{}
			case 7:
				s.reverse = true
			case 27:
				s.reverse = false
			case 49:
				s.background = ""
			case 48:
				index = func() int {
					if index+4 < len(values) && values[index+1] == "2" {
						s.background = strings.Join(values[index+2:index+5], ";")
						return index + 4
					}
					return index
				}()
			default:
				continue
			}
		}
		rendered = rendered[location[1]:]
	}
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, column, 80, "styled columns = %d, want %d", column, 80)
}

func TestDiffscreenSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, false)
	first := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	removed, _ := func() (diffscreenDiffCursor, bool) {
		var query string = "removed one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if v.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: first.pane}
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
											if first.pane == diffscreenLeft {
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
				panes := []diffscreenDiffPane{first.pane}
				if v.split {
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
					line, _ := (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
						return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
								return patchLine{}, false
							}
							current := v.rows[cursor.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}
					}(v))(candidate)
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
	selection := diffscreenDiffSelection{First: removed, Last: removed}
	rendered := (func(
		v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
		selection *diffscreenDiffSelection) string {
		viewport := func() diffscreenDiffViewport {
			var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, 1)
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
		}()
		viewport.top = target.row
		return (func(v *diffscreenDiffView) func(
			viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
			return func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == diffscreenLineRow {
						lines = append(
							lines,
							(func(v *diffscreenDiffView) func(
								current diffscreenEntry, y int,
								viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
								return func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != diffscreenLineRow {
												return -1
											}
											if !v.split || diffscreenLeft == diffscreenRight {
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
										if diffscreenLeft == diffscreenLeft {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+") + " "
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-") + " "
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
											}(), diffscreenDiscardForeground
										}
										if cursor == candidate {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffscreenDiffView) func(
										current diffscreenEntry, y int,
										pane diffscreenDiffPane, width, offset int,
										cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										return func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || pane == diffscreenRight {
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
											if pane == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: pane}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										diffscreenRight,
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
								case diffscreenFileRow:
									return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
								case diffscreenMetadataRow:
									return diffscreenMetadataStyle.Render("  " + current.text)
								case diffscreenHunkRow:
									return diffscreenHunkStyle.Render(current.text)
								case diffscreenLineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patchAddition {
										prefix = diffscreenAddedStyle.Render("+")
									}
									if line.Kind == patchDeletion {
										prefix = diffscreenRemovedStyle.Render("-")
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
										case patchAddition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patchDeletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return diffscreenContextStyle
										}
									}()
									colors := diffscreenPreserveForeground
									candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
										}(), diffscreenDiscardForeground
									}
									if cursor.row == y {
										style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
									}
									{
										var (
											style  lipgloss.Style             = style
											value  string                     = value
											colors diffscreenForegroundPolicy = colors
										)
										value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, active, selection)
	})(v, removed, first, 72, &selection)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(rendered, "\x1b[1m"), true,
		"selection is bold: %q", rendered,
	)

	{
		var (
			rendered string                   = rendered
			expected diffscreenSgrExpectation = diffscreenSgrExpectation{background: "219;234;254"}
		)
		t.Helper()
		state := diffscreenSgrState{}
		column := 0
		for len(rendered) > 0 {
			location := diffscreenSgrPattern.FindStringSubmatchIndex(rendered)
			if location == nil {
				column = func() int {
					var (
						text   string             = rendered
						column int                = column
						state  diffscreenSgrState = state
					)
					t.Helper()
					for range text {
						column++
						var column int = column
						t.Helper()
						(func(t *testing.T, got, want string, format string, args ...any) {
							t.Helper()
							if got != want {
								t.Fatalf(format, args...)
							}
						})(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
						(func(t *testing.T, got, want bool, format string, args ...any) {
							t.Helper()
							if got != want {
								t.Fatalf(format, args...)
							}
						})(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
					}
					return column
				}()
				break
			}
			column = func() int {
				var (
					text   string             = rendered[:location[0]]
					column int                = column
					state  diffscreenSgrState = state
				)
				t.Helper()
				for range text {
					column++
					var column int = column
					t.Helper()
					(func(t *testing.T, got, want string, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
				}
				return column
			}()
			var (
				s          *diffscreenSgrState = &state
				parameters string              = rendered[location[2]:location[3]]
			)
			if parameters == "" {
				parameters = "0"
			}
			values := strings.Split(parameters, ";")
			for index := 0; index < len(values); index++ {
				value, _ := strconv.Atoi(values[index])
				switch value {
				case 0:
					*s = diffscreenSgrState{}
				case 7:
					s.reverse = true
				case 27:
					s.reverse = false
				case 49:
					s.background = ""
				case 48:
					index = func() int {
						if index+4 < len(values) && values[index+1] == "2" {
							s.background = strings.Join(values[index+2:index+5], ";")
							return index + 4
						}
						return index
					}()
				default:
					continue
				}
			}
			rendered = rendered[location[1]:]
		}
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, column, 72, "styled columns = %d, want %d", column, 72)
	}
}

func TestDiffscreenSyntaxHighlightingSurvivesDiffStyling(t *testing.T) {
	p := patchPatch{
		Files: []patchFile{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				OldSource:   "package main\nold()\n",
				NewSource:   "package main\nnew()\n",
				Hunks: []patchHunk{
					{
						Header: "@@",
						Lines: []patchLine{
							{Kind: patchDeletion, Text: "old()", OldNumber: 2},
							{Kind: patchAddition, Text: "new()", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	v := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
	first := func() diffscreenDiffCursor {
		t.Helper()
		cursor, ok := func() (diffscreenDiffCursor, bool) {
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
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	added, _ := func() (diffscreenDiffCursor, bool) {
		var query string = "new()"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current diffscreenEntry = v.rows[first.row]
			if current.kind != diffscreenLineRow {
				return -1
			}
			if !v.split || first.pane == diffscreenRight {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffscreenDiffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(diffscreenPrivateForward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffscreenDiffCursor, bool) {
				var query string = query
				if v.rows[y].kind != diffscreenLineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(diffscreenPrivateForward)*distance, y - int(diffscreenPrivateForward)*distance} {
								if candidate, ok := func() (diffscreenDiffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffscreenDiffCursor{}, false
									}
									if candidate, ok := func() (diffscreenDiffCursor, bool) {
										cursor := diffscreenDiffCursor{row: candidateY, pane: first.pane}
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
											if first.pane == diffscreenLeft {
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
				panes := []diffscreenDiffPane{first.pane}
				if v.split {
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
					line, _ := (func(v *diffscreenDiffView) func(cursor diffscreenDiffCursor) (patchLine, bool) {
						return func(cursor diffscreenDiffCursor) (patchLine, bool) {
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
								return patchLine{}, false
							}
							current := v.rows[cursor.row]
							return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !v.split || cursor.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}
					}(v))(candidate)
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
	for _, cursor := range []diffscreenDiffCursor{first, added} {
		rendered := (func(
			v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
			selection *diffscreenDiffSelection) string {
			viewport := func() diffscreenDiffViewport {
				var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
				viewport.Width, viewport.Height = max(1, width), max(1, 1)
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
			}()
			viewport.top = target.row
			return (func(v *diffscreenDiffView) func(
				viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
				return func(
					viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
					lines := make([]string, 0, viewport.Height)
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
						current := v.rows[viewport.top]
						lines = append(
							lines,
							func() string {
								var path string = v.patch.Files[current.file].DisplayPath
								return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
							}(),
						)
					}
					end := min(len(v.rows), viewport.top+func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}())
					for y := viewport.top; y < end; y++ {
						current := v.rows[y]
						if v.split && current.kind == diffscreenLineRow {
							lines = append(
								lines,
								(func(v *diffscreenDiffView) func(
									current diffscreenEntry, y int,
									viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
									return func(
										current diffscreenEntry, y int,
										viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
										leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
											inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
											return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
										}()
										inlineLocalLeft1 := func() string {
											index := func() int {
												if current.kind != diffscreenLineRow {
													return -1
												}
												if !v.split || diffscreenLeft == diffscreenRight {
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
											if diffscreenLeft == diffscreenLeft {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patchAddition {
												prefix = diffscreenAddedStyle.Render("+") + " "
											}
											if line.Kind == patchDeletion {
												prefix = diffscreenRemovedStyle.Render("-") + " "
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
												case patchAddition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patchDeletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return diffscreenContextStyle
												}
											}()
											colors := diffscreenPreserveForeground
											candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
												}(), diffscreenDiscardForeground
											}
											if cursor == candidate {
												style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
											}
											{
												var (
													style  lipgloss.Style             = style
													value  string                     = value
													colors diffscreenForegroundPolicy = colors
												)
												value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										inlineLocalRight1 := (func(v *diffscreenDiffView) func(
											current diffscreenEntry, y int,
											pane diffscreenDiffPane, width, offset int,
											cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											return func(
												current diffscreenEntry, y int,
												pane diffscreenDiffPane, width, offset int,
												cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
												index := func() int {
													if current.kind != diffscreenLineRow {
														return -1
													}
													if !v.split || pane == diffscreenRight {
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
												if pane == diffscreenLeft {
													text, numberValue = current.left, line.OldNumber
												}
												prefix := "  "
												if line.Kind == patchAddition {
													prefix = diffscreenAddedStyle.Render("+") + " "
												}
												if line.Kind == patchDeletion {
													prefix = diffscreenRemovedStyle.Render("-") + " "
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
													case patchAddition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patchDeletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return diffscreenContextStyle
													}
												}()
												colors := diffscreenPreserveForeground
												candidate := diffscreenDiffCursor{row: y, pane: pane}
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
													}(), diffscreenDiscardForeground
												}
												if cursor == candidate {
													style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
												}
												{
													var (
														style  lipgloss.Style             = style
														value  string                     = value
														colors diffscreenForegroundPolicy = colors
													)
													value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
											diffscreenRight,
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
									case diffscreenFileRow:
										return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
									case diffscreenMetadataRow:
										return diffscreenMetadataStyle.Render("  " + current.text)
									case diffscreenHunkRow:
										return diffscreenHunkStyle.Render(current.text)
									case diffscreenLineRow:
										hunk := v.patch.Files[current.file].Hunks[current.hunk]
										line := hunk.Lines[current.rightLine]
										prefix := " "
										if line.Kind == patchAddition {
											prefix = diffscreenAddedStyle.Render("+")
										}
										if line.Kind == patchDeletion {
											prefix = diffscreenRemovedStyle.Render("-")
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
											case patchAddition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patchDeletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return diffscreenContextStyle
											}
										}()
										colors := diffscreenPreserveForeground
										candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
											}(), diffscreenDiscardForeground
										}
										if cursor.row == y {
											style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
										}
										{
											var (
												style  lipgloss.Style             = style
												value  string                     = value
												colors diffscreenForegroundPolicy = colors
											)
											value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
			}(v))(viewport, active, selection)
		})(v, cursor, diffscreenDiffCursor{}, 80, nil)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, strings.Contains(rendered, "[38;2;"), true,
			"syntax highlighting missing: %q", rendered,
		)
	}
}

func TestDiffscreenRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patchPatch, bool) *diffscreenDiffView
		width       int
	}{{(func(p patchPatch, dark bool) *diffscreenDiffView {
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
	}), 37}, {(func(p patchPatch, dark bool) *diffscreenDiffView {
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
	}), 120}} {
		v := test.constructor(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, true)
		cursor := func() diffscreenDiffCursor {
			t.Helper()
			cursor, ok := func() (diffscreenDiffCursor, bool) {
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
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "%s", "no first cursor")
			return cursor
		}()
		for {
			width := lipgloss.Width((func(
				v *diffscreenDiffView, cursor diffscreenDiffCursor, width int,
				selection *diffscreenDiffSelection) string {
				return (func(
					v *diffscreenDiffView, target, active diffscreenDiffCursor, width int,
					selection *diffscreenDiffSelection) string {
					viewport := func() diffscreenDiffViewport {
						var viewport diffscreenDiffViewport = diffscreenDiffViewport{}
						viewport.Width, viewport.Height = max(1, width), max(1, 1)
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
					}()
					viewport.top = target.row
					return (func(v *diffscreenDiffView) func(
						viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
						return func(
							viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
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
							lines := make([]string, 0, viewport.Height)
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
								current := v.rows[viewport.top]
								lines = append(
									lines,
									func() string {
										var path string = v.patch.Files[current.file].DisplayPath
										return diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
									}(),
								)
							}
							end := min(len(v.rows), viewport.top+func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
									height--
								}
								return max(1, height)
							}())
							for y := viewport.top; y < end; y++ {
								current := v.rows[y]
								if v.split && current.kind == diffscreenLineRow {
									lines = append(
										lines,
										(func(v *diffscreenDiffView) func(
											current diffscreenEntry, y int,
											viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
											return func(
												current diffscreenEntry, y int,
												viewport diffscreenDiffViewport, cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
												leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
													inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
													return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
												}()
												inlineLocalLeft1 := func() string {
													index := func() int {
														if current.kind != diffscreenLineRow {
															return -1
														}
														if !v.split || diffscreenLeft == diffscreenRight {
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
													if diffscreenLeft == diffscreenLeft {
														text, numberValue = current.left, line.OldNumber
													}
													prefix := "  "
													if line.Kind == patchAddition {
														prefix = diffscreenAddedStyle.Render("+") + " "
													}
													if line.Kind == patchDeletion {
														prefix = diffscreenRemovedStyle.Render("-") + " "
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
														case patchAddition:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
														case patchDeletion:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
														default:
															return diffscreenContextStyle
														}
													}()
													colors := diffscreenPreserveForeground
													candidate := diffscreenDiffCursor{row: y, pane: diffscreenLeft}
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
														}(), diffscreenDiscardForeground
													}
													if cursor == candidate {
														style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
													}
													{
														var (
															style  lipgloss.Style             = style
															value  string                     = value
															colors diffscreenForegroundPolicy = colors
														)
														value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																	if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
																	if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
												inlineLocalRight1 := (func(v *diffscreenDiffView) func(
													current diffscreenEntry, y int,
													pane diffscreenDiffPane, width, offset int,
													cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
													return func(
														current diffscreenEntry, y int,
														pane diffscreenDiffPane, width, offset int,
														cursor diffscreenDiffCursor, selection *diffscreenDiffSelection) string {
														index := func() int {
															if current.kind != diffscreenLineRow {
																return -1
															}
															if !v.split || pane == diffscreenRight {
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
														if pane == diffscreenLeft {
															text, numberValue = current.left, line.OldNumber
														}
														prefix := "  "
														if line.Kind == patchAddition {
															prefix = diffscreenAddedStyle.Render("+") + " "
														}
														if line.Kind == patchDeletion {
															prefix = diffscreenRemovedStyle.Render("-") + " "
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
															case patchAddition:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
															case patchDeletion:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
															default:
																return diffscreenContextStyle
															}
														}()
														colors := diffscreenPreserveForeground
														candidate := diffscreenDiffCursor{row: y, pane: pane}
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
															}(), diffscreenDiscardForeground
														}
														if cursor == candidate {
															style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
														}
														{
															var (
																style  lipgloss.Style             = style
																value  string                     = value
																colors diffscreenForegroundPolicy = colors
															)
															value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																		if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
																		if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
													diffscreenRight,
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
											case diffscreenFileRow:
												return diffscreenFileStyle.Width(max(20, width)).Render(current.text)
											case diffscreenMetadataRow:
												return diffscreenMetadataStyle.Render("  " + current.text)
											case diffscreenHunkRow:
												return diffscreenHunkStyle.Render(current.text)
											case diffscreenLineRow:
												hunk := v.patch.Files[current.file].Hunks[current.hunk]
												line := hunk.Lines[current.rightLine]
												prefix := " "
												if line.Kind == patchAddition {
													prefix = diffscreenAddedStyle.Render("+")
												}
												if line.Kind == patchDeletion {
													prefix = diffscreenRemovedStyle.Render("-")
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
													case patchAddition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patchDeletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return diffscreenContextStyle
													}
												}()
												colors := diffscreenPreserveForeground
												candidate := diffscreenDiffCursor{row: y, pane: cursor.pane}
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
													}(), diffscreenDiscardForeground
												}
												if cursor.row == y {
													style, colors = diffscreenCursorStyle, diffscreenDiscardForeground
												}
												{
													var (
														style  lipgloss.Style             = style
														value  string                     = value
														colors diffscreenForegroundPolicy = colors
													)
													value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == diffscreenDiscardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
					}(v))(viewport, active, selection)
				})(v, cursor, cursor, width, selection)
			})(v, cursor, test.width, nil))
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, width, test.width, "width=%d want=%d",
				width, test.width,
			)
			next, ok := func() (diffscreenDiffCursor, bool) {
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
					return diffscreenDiffCursor{}, false
				}
				var pane diffscreenDiffPane = cursor.pane
				step := int(diffscreenPrivateForward)
				for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffscreenDiffCursor, bool) {
						cursor := diffscreenDiffCursor{row: y, pane: pane}
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
			if !ok {
				break
			}
			cursor = next
		}
	}
}

func TestDiffscreenRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
	value := strings.Join(
		[]string{
			"\x1b[48;2;255;0;0;38;2;1;2;3mtruecolor",
			"\x1b[48;5;123;1mindexed",
			"\x1b[45mstandard",
			"\x1b[105mbright",
		},
		" ",
	)
	rendered := func() string {
		var (
			style lipgloss.Style = func() lipgloss.Style {
				lightDark := lipgloss.LightDark(true)
				switch patchAddition {
				case patchAddition:
					return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
				case patchDeletion:
					return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
				default:
					return diffscreenContextStyle
				}
			}()
			value string = value
		)
		value = diffscreenAnsiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
					if code == 48 || diffscreenPreserveForeground == diffscreenDiscardForeground && code == 38 {
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
					if code >= 40 && code <= 49 || code >= 100 && code <= 107 || diffscreenPreserveForeground == diffscreenDiscardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
			if 80 <= 0 {
				return ""
			}
			value = strings.ReplaceAll(value, "\t", "    ")
			if 0 > 0 {
				value = ansi.TruncateLeft(value, 0, "")
			}
			value = ansi.Truncate(value, 80, "")
			if padding := 80 - lipgloss.Width(value); padding > 0 {
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
	}()
	for _, forbidden := range []string{
		"48;2;255;0;0",
		"48;5;123",
		"[45m",
		"[105m",
	} {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, !strings.Contains(rendered, forbidden), true,
			"retains %q: %q", forbidden, rendered,
		)
	}
}

type diffscreenSgrExpectation struct {
	background string
	reverse    bool
}
type diffscreenSgrState struct {
	background string
	reverse    bool
}

var diffscreenSgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)
