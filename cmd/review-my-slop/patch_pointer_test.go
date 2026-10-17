package main

import (
	bytes "bytes"
	io "io"
	strings "strings"
	testing "testing"

	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestDiffscreenClickAndDragSelectCode(t *testing.T) {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, diffscreenOptions{})
	v.Resize(80, 20)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.BeginDrag(20, 4), true, "%s",
		"code click was ignored",
	)

	_, line, _ := v.Current()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line.Text, "removed one", "%s",
		"click did not focus code without selecting a range",
	)
	(func(t *testing.T, got, want *diffscreenDiffSelection, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.selection, nil, "%s",
		"click did not focus code without selecting a range",
	)

	v.DragTo(30, 6)
	v.EndDrag()
	_, lines, ok := v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "drag selection = %#v", lines)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 3, "drag selection = %#v", lines)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lines[0].Text, "removed one",
		"drag selection = %#v", lines,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lines[2].Text, "added one", "drag selection = %#v",
		lines,
	)

	end := v.cursor
	v.DragTo(20, 3)
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, end, "%s",
		"motion after release changed selection",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.BeginDrag(20, 6), true, "%s",
		"new click did not clear old selection",
	)
	(func(t *testing.T, got, want *diffscreenDiffSelection, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.selection, nil, "%s",
		"new click did not clear old selection",
	)

	v.DragTo(20, 3)
	_, lines, ok = v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "backward drag = %#v", lines)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 4, "backward drag = %#v", lines)

	v.DragTo(20, 6)
	_, lines, _ = v.Selected()
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 1, "%s",
		"drag back to origin did not shrink selection",
	)
}

func TestDiffscreenMouseHitTestingMatchesScrolledProjection(t *testing.T) {
	for _, split := range []bool{false, true} {
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
		})(), diffscreenOptions{SideBySide: split})
		v.Resize(121, 10)
		v.ScrollVertical(5)
		v.ScrollHorizontal(20)
		for _, point := range [][2]int{
			{0, 0},
			{0, 1},
			{0, 8},
			{0, 9},
			{-1, 3},
			{121, 3},
		} {
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, !v.BeginDrag(point[0], point[1]), true,
				"non-code point %v accepted", point,
			)
		}
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.BeginDrag(100, 2), true, "%s",
			"click did not account for viewport and sticky header",
		)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.cursor.row, 5, "%s",
			"click did not account for viewport and sticky header",
		)
		(func(t *testing.T, got, want diffscreenDiffPane, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.cursor.pane, diffscreenRight, "%s",
			"click did not account for viewport and sticky header",
		)

		v.DragTo(100, 4)
		_, lines, ok := v.Selected()
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "scrolled drag selection = %#v",
			lines,
		)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, len(lines), 3, "scrolled drag selection = %#v",
			lines,
		)
	}
}

func TestDiffscreenSplitMouseRejectsDividerEmptyPaneAndCrossPaneDrag(t *testing.T) {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, diffscreenOptions{SideBySide: true})
	v.Resize(120, 20)
	for _, x := range []int{58, 59, 60} {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, !v.BeginDrag(x, 3), true, "%s",
			"divider accepted",
		)
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !v.BeginDrag(80, 5), true, "%s",
		"empty right pane accepted",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.BeginDrag(10, 4), true, "%s",
		"deletion click did not focus old pane",
	)
	(func(t *testing.T, got, want diffscreenDiffPane, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor.pane, diffscreenLeft, "%s",
		"deletion click did not focus old pane",
	)

	origin := v.cursor
	v.DragTo(80, 4)
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, origin, "%s", "drag crossed panes")
	(func(t *testing.T, got, want *diffscreenDiffSelection, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.selection, nil, "%s", "drag crossed panes")

	v.DragTo(10, 5)
	_, lines, ok := v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "old-pane drag = %#v", lines)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 2, "old-pane drag = %#v", lines)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lines[1].Text, "removed two",
		"old-pane drag = %#v", lines,
	)
}

func TestDiffscreenDragRejectsOtherHunksAndCancelsOnStateChanges(t *testing.T) {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, diffscreenOptions{})
	v.Resize(80, 20)
	v.BeginDrag(10, 3)
	v.DragTo(10, 4)
	cursor, selection := v.cursor, *v.selection
	v.DragTo(10, 10)
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, cursor, "%s",
		"drag crossed file/hunk boundary",
	)
	(func(t *testing.T, got, want diffscreenDiffSelection, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, *v.selection, selection, "%s",
		"drag crossed file/hunk boundary",
	)

	for _, change := range []func(){
		func() {
			v.Update(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}})
		},
		func() { v.Resize(80, 20) },
		func() { v.Move(diffscreenNextLine) },
		func() { v.ClearSelection() },
	} {
		v.BeginDrag(10, 3)
		change()
		(func(t *testing.T, got, want *diffscreenDiffCursor, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.drag, nil, "%s",
			"stale drag survived state change",
		)
	}
}
