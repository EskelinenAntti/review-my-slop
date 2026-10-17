package main

import (
	bytes "bytes"
	context "context"
	io "io"
	strings "strings"
	testing "testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestAppMouseDragCreatesCommentSelection(t *testing.T) {
	t.Setenv("EDITOR", "true")
	for _, release := range []tea.MouseButton{tea.MouseLeft, tea.MouseNone} {
		m := func() appModel {
			var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
			var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
			inlineLocalSize3 := layout.size
			if inlineLocalSize3.Width <= 0 || inlineLocalSize3.Height <= 0 {
				inlineLocalSize3 = appDefaultSize
			}
			m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
				v := &commentscreenView{width: 80, height: 30}
				v.Update(items)
				return v
			})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize3.Width, height: inlineLocalSize3.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
				m := keymapMatcher{sequences: make(map[string]keymapSequence, len(sequences))}
				for _, sequence := range sequences {
					sequence.Keys = append([]string(nil), sequence.Keys...)
					m.sequences[sequence.Prefix] = sequence
				}
				return m
			})(keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymapSequence{Prefix: "[", Keys: []string{"f"}}, keymapSequence{Prefix: "]", Keys: []string{"f"}}, keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
			m.diffView = (func(p patchPatch, options diffscreenOptions) *diffscreenView {
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
			})(p, m.diffOptions)
			m.diffView.Resize(m.width, m.height)
			m.commentView.Resize(m.width, m.height)
			return m
		}()
		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseClickMsg{X: 20, Y: 4, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(appModel)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "model=%T", next)
			return result
		}()
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "%s",
			"click did not focus deletion",
		)

		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseMotionMsg{X: 20, Y: 5, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(appModel)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "model=%T", next)
			return result
		}()
		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseReleaseMsg{X: 20, Y: 6, Button: release})
			t.Helper()
			result, ok := next.(appModel)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "model=%T", next)
			return result
		}()
		_, lines, ok := m.diffView.Selected()
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "released selection = %#v", lines)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, len(lines), 3, "released selection = %#v",
			lines,
		)
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()",
			"released selection = %#v", lines,
		)

		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseMotionMsg{X: 20, Y: 3, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(appModel)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "model=%T", next)
			return result
		}()
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()", "%s",
			"motion after release changed focus",
		)

		m = func() appModel {
			var (
				m   appModel = m
				msg tea.Msg  = func() tea.KeyPressMsg {
					var text string = "c"
					runes := []rune(text)
					return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
				}()
			)
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(appModel)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "model=%T", next)
			return result
		}()
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, len(m.edit.anchor.QuotedLines), 3,
			"comment anchor = %#v", m.edit.anchor,
		)
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, m.edit.anchor.QuotedLines[0], "-old()",
			"comment anchor = %#v", m.edit.anchor,
		)
	}
}

func TestAppMouseClickCommentsAndIgnoreInformationalUI(t *testing.T) {
	m := func() appModel {
		var (
			p     patchPatch        = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
			items []commentsComment = []commentsComment{{ID: "one"}, {ID: "two"}}
		)
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize2 := layout.size
		if inlineLocalSize2.Width <= 0 || inlineLocalSize2.Height <= 0 {
			inlineLocalSize2 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: items}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(items), edit: appCommentEdit{index: -1}, width: inlineLocalSize2.Width, height: inlineLocalSize2.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
			m := keymapMatcher{sequences: make(map[string]keymapSequence, len(sequences))}
			for _, sequence := range sequences {
				sequence.Keys = append([]string(nil), sequence.Keys...)
				m.sequences[sequence.Prefix] = sequence
			}
			return m
		})(keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymapSequence{Prefix: "[", Keys: []string{"f"}}, keymapSequence{Prefix: "]", Keys: []string{"f"}}, keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = (func(p patchPatch, options diffscreenOptions) *diffscreenView {
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
		})(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "C"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(appModel)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.MouseClickMsg{X: 10, Y: 2, Button: tea.MouseLeft})
		t.Helper()
		result, ok := next.(appModel)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "model=%T", next)
		return result
	}()
	selected, _ := m.commentView.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.ID, "two", "%s",
		"click did not focus comment",
	)

	for _, inlineLocalMode1 := range []appMode{
		appModeBrowse,
		appModeComments,
		appModeHelp,
		appModeSearch,
	} {
		m.mode = inlineLocalMode1
		before := func() string {
			switch m.mode {
			case appModeHelp:
				bindings := []helpscreenBinding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
				return (func(bindings []helpscreenBinding, width, height int) string {
					column := 0
					for _, binding := range bindings {
						column = max(column, lipgloss.Width(binding.Keys))
					}
					body := []string{""}
					for _, binding := range bindings {
						line := binding.Keys + strings.Repeat(
							" ",
							column-lipgloss.Width(binding.Keys),
						) + "  " + binding.Description
						body = append(body, ansi.Truncate(line, max(20, width), ""))
					}
					return (func(header string, body []string, footer string, height int) string {
						bodyHeight := (func(height int) int { return max(1, height-3) })(height)
						lines := make([]string,

							// Add validates and stores a comment, assigning an ID and timestamp when
							// absent.
							0, bodyHeight+3)
						lines = append(lines, header)
						lines = append(lines, body[:min(len(body), bodyHeight)]...)
						for len(lines) < bodyHeight+1 {
							lines = append(lines, "")
						}
						lines = append(lines, footer, "")
						return strings.Join(lines, "\n")
					})(
						lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan).Render(
							"review-my-slop help",
						),
						body,
						lipgloss.NewStyle().Faint(true).Render("? or Esc closes help"),
						height,
					)
				})(bindings, m.width, m.height)
			case appModeComments:
				return m.commentView.Render(m.err)
			default:
				return m.diffView.Render(m.err)
			}
		}()
		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(appModel)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "model=%T", next)
			return result
		}()
		m = func(m appModel, msg tea.Msg) appModel {
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(appModel)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "model=%T", next)
			return result
		}(m,
			tea.MouseClickMsg{X: 0, Y: m.height - 2, Button: tea.MouseLeft})
		(func(t *testing.T, got, want appMode, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, m.mode, inlineLocalMode1, "%s",
			"header/footer click triggered an action",
		)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, !m.quitting, true, "%s",
			"header/footer click triggered an action",
		)
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, func() string {
			switch m.mode {
			case appModeHelp:
				bindings := []helpscreenBinding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
				return (func(bindings []helpscreenBinding, width, height int) string {
					column := 0
					for _, binding := range bindings {
						column = max(column, lipgloss.Width(binding.Keys))
					}
					body := []string{""}
					for _, binding := range bindings {
						line := binding.Keys + strings.Repeat(
							" ",
							column-lipgloss.Width(binding.Keys),
						) + "  " + binding.Description
						body = append(body, ansi.Truncate(line, max(20, width), ""))
					}
					return (func(header string, body []string, footer string, height int) string {
						bodyHeight := (func(height int) int { return max(1, height-3) })(height)
						lines := make([]string,

							// Add validates and stores a comment, assigning an ID and timestamp when
							// absent.
							0, bodyHeight+3)
						lines = append(lines, header)
						lines = append(lines, body[:min(len(body), bodyHeight)]...)
						for len(lines) < bodyHeight+1 {
							lines = append(lines, "")
						}
						lines = append(lines, footer, "")
						return strings.Join(lines, "\n")
					})(
						lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan).Render(
							"review-my-slop help",
						),
						body,
						lipgloss.NewStyle().Faint(true).Render("? or Esc closes help"),
						height,
					)
				})(bindings, m.width, m.height)
			case appModeComments:
				return m.commentView.Render(m.err)
			default:
				return m.diffView.Render(m.err)
			}
		}(), before, "%s",
			"header/footer click triggered an action",
		)
	}
}

func TestAppMouseIgnoresOtherButtonsAndKeyboardCancelsDrag(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize1 := layout.size
		if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
			inlineLocalSize1 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
			m := keymapMatcher{sequences: make(map[string]keymapSequence, len(sequences))}
			for _, sequence := range sequences {
				sequence.Keys = append([]string(nil), sequence.Keys...)
				m.sequences[sequence.Prefix] = sequence
			}
			return m
		})(keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymapSequence{Prefix: "[", Keys: []string{"f"}}, keymapSequence{Prefix: "]", Keys: []string{"f"}}, keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = (func(p patchPatch, options diffscreenOptions) *diffscreenView {
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
		})(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.MouseClickMsg{X: 10, Y: 5, Button: tea.MouseRight})
		t.Helper()
		result, ok := next.(appModel)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "model=%T", next)
		return result
	}()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "%s",
		"right click moved focus",
	)

	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft})
		t.Helper()
		result, ok := next.(appModel)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "model=%T", next)
		return result
	}()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "%s",
		"motion without a press moved focus",
	)

	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.MouseClickMsg{X: 10, Y: 3, Button: tea.MouseLeft})
		t.Helper()
		result, ok := next.(appModel)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "j"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(appModel)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft})
		t.Helper()
		result, ok := next.(appModel)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, ok, true, "model=%T", next)
		return result
	}()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "%s",
		"keyboard action did not cancel drag",
	)
}
