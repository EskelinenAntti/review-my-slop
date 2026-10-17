package main

import (
	bytes "bytes"
	context "context"
	errors "errors"
	fmt "fmt"
	"image/color"
	io "io"
	os "os"
	exec "os/exec"
	filepath "path/filepath"
	slices "slices"
	strconv "strconv"
	strings "strings"
	testing "testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestAppVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved []commentsComment
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize30 := layout.size
		if inlineLocalSize30.Width <= 0 || inlineLocalSize30.Height <= 0 {
			inlineLocalSize30 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize30.Width, height: inlineLocalSize30.Height, save: func(stored commentsComment, _ patchPatch) (commentsComment, error) {
			saved = append(saved, stored)
			stored.ID = "new"
			return stored, nil
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "v"
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
	_ = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appCommentEditorFinishedMsg{body: "fix both lines"})
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
	})(t, len(saved), 1, "saved comments = %d", len(saved))

	anchor := saved[0].Anchor
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, anchor.FilePath, "main.go", "anchor = %#v", anchor)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, anchor.OldStart, 2, "anchor = %#v", anchor)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, anchor.NewStart, 2, "anchor = %#v", anchor)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}), true,
		"anchor = %#v", anchor,
	)
}

func TestAppCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize12 := layout.size
		if inlineLocalSize12.Width <= 0 || inlineLocalSize12.Height <= 0 {
			inlineLocalSize12 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize12.Width, height: inlineLocalSize12.Height, save: func(commentsComment, patchPatch) (commentsComment, error) {
			return commentsComment{}, errors.New("storage unavailable")
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appCommentEditorFinishedMsg{body: "keep this"})
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
	})(t, m.edit.body, "", "body=%q err=%v", m.edit.body,
		m.err,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err != nil, true, "body=%q err=%v", m.edit.body,
		m.err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err.Error(), "storage unavailable",
		"body=%q err=%v", m.edit.body, m.err,
	)
}

func TestAppCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := func() appModel {
		var (
			m appModel = func() appModel {
				var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
				var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
				inlineLocalSize29 := layout.size
				if inlineLocalSize29.Width <= 0 || inlineLocalSize29.Height <= 0 {
					inlineLocalSize29 = appDefaultSize
				}
				m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
					v := &commentscreenView{width: 80, height: 30}
					v.Update(items)
					return v
				})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize29.Width, height: inlineLocalSize29.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
			msg tea.Msg = func() tea.KeyPressMsg {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err != nil, true, "error = %v", m.err)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err.Error(), "$EDITOR is not set", "error = %v",
		m.err,
	)
}

func TestAppEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv("EDITOR", "true")
	called := false
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize11 := layout.size
		if inlineLocalSize11.Width <= 0 || inlineLocalSize11.Height <= 0 {
			inlineLocalSize11 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize11.Width, height: inlineLocalSize11.Height, save: func(stored commentsComment, _ patchPatch) (commentsComment, error) {
			called = true
			return stored, nil
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appCommentEditorFinishedMsg{body: " \n"})
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !called, true, "called=%v comments=%d", called,
		len(m.comments.items),
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(m.comments.items), 0, "called=%v comments=%d",
		called, len(m.comments.items),
	)
}

func TestAppOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize5 := layout.size
		if inlineLocalSize5.Width <= 0 || inlineLocalSize5.Height <= 0 {
			inlineLocalSize5 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize5.Width, height: inlineLocalSize5.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.currentPatch.Root = "/tmp/repo with spaces"
	func() {
		var (
			m    *appModel = &m
			text string    = "new()"
		)
		t.Helper()
		m.diffView.Move(diffscreenFirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreenNextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	cmd, err := func() (tea.Cmd, error) {
		path, number, err := func() (string, int, error) {
			file, line, ok := m.diffView.Current()
			if !ok {
				return "", 0, errors.New("select a code line to open in $EDITOR")
			}
			path, number := file.NewPath, line.NewNumber
			if path == "" || path == "/dev/null" {
				path = file.OldPath
			}
			if number == 0 {
				number = line.OldNumber
			}
			if path == "" || path == "/dev/null" || number < 1 {
				return "", 0, errors.New("current line has no editable working-tree location")
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
			}
			return path, int(number), nil
		}()
		if err != nil {
			return nil, err
		}
		return (func(
			ctx context.Context,
			path string,
			line int,
			onComplete func(error) tea.Msg,
		) (tea.Cmd, error) {
			command, err := (func() (string, error) {
				command := strings.TrimSpace(os.Getenv("EDITOR"))
				if command == "" {
					return "", errors.New("$EDITOR is not set")
				}
				return command, nil
			})()
			if err != nil {
				return nil, err
			}
			adapter := &editorCommandAdapter{}
			adapter.run = func() error {
				var cmd *exec.Cmd = exec.CommandContext(ctx, "sh", "-c", command+" +"+strconv.Itoa(line)+" "+(func(value string) string {
					return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
				})(path))
				cmd.Stdin, cmd.Stdout, cmd.Stderr = adapter.stdin, adapter.stdout, adapter.stderr
				return cmd.Run()
			}
			return tea.Exec(adapter, onComplete), nil
		})(m.ctx, path, number, func(err error) tea.Msg { return appSourceEditorFinishedMsg{err: err} })
	}()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "command=%v err=%v", cmd, err)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd != nil, true, "command=%v err=%v", cmd, err)

	path, line, err := func() (string, int, error) {
		file, line, ok := m.diffView.Current()
		if !ok {
			return "", 0, errors.New("select a code line to open in $EDITOR")
		}
		path, number := file.NewPath, line.NewNumber
		if path == "" || path == "/dev/null" {
			path = file.OldPath
		}
		if number == 0 {
			number = line.OldNumber
		}
		if path == "" || path == "/dev/null" || number < 1 {
			return "", 0, errors.New("current line has no editable working-tree location")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
		}
		return path, int(number), nil
	}()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "location=%q:%d err=%v", path, line, err)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, path, "/tmp/repo with spaces/main.go",
		"location=%q:%d err=%v", path, line, err,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line, 2, "location=%q:%d err=%v", path, line, err)
}

func TestAppOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := func() appModel {
		var (
			m appModel = func() appModel {
				var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
				var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
				inlineLocalSize18 := layout.size
				if inlineLocalSize18.Width <= 0 || inlineLocalSize18.Height <= 0 {
					inlineLocalSize18 = appDefaultSize
				}
				m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
					v := &commentscreenView{width: 80, height: 30}
					v.Update(items)
					return v
				})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize18.Width, height: inlineLocalSize18.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "e"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err != nil, true, "error = %v", m.err)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err.Error(), "$EDITOR is not set", "error = %v",
		m.err,
	)
}

func TestAppCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []commentsComment{
		{ID: "one", Body: "old body"},
		{ID: "two", Body: "second"},
	}
	var persisted, deleted commentsComment
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize33 := layout.size
		if inlineLocalSize33.Width <= 0 || inlineLocalSize33.Height <= 0 {
			inlineLocalSize33 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: items}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(items), edit: appCommentEdit{index: -1}, width: inlineLocalSize33.Width, height: inlineLocalSize33.Height, save: func(stored commentsComment, _ patchPatch) (commentsComment, error) {
			persisted = stored
			return stored, nil
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.delete = func(stored commentsComment, _ patchPatch) error { deleted = stored; return nil }
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
	(func(t *testing.T, got, want appMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.mode, appModeComments, "%s",
		"comments did not open",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(func() string {
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
	}(), "old body"), true,
		"%s", "comments did not open",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
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
		next, _ := m.Update(appCommentEditorFinishedMsg{body: "edited body"})
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
	})(t, persisted.ID, "one", "persisted = %#v", persisted)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, persisted.Body, "edited body", "persisted = %#v",
		persisted,
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "D"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, deleted.ID, "one", "deleted=%#v comments=%#v",
		deleted, m.comments.items,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(m.comments.items), 1,
		"deleted=%#v comments=%#v", deleted, m.comments.items,
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "q"
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
	(func(t *testing.T, got, want appMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.mode, appModeBrowse, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !m.quitting, true, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
}

func TestAppOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := func() appModel {
		var (
			p     patchPatch        = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
			items []commentsComment = []commentsComment{{ID: "read", Body: "already read"}}
		)
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize7 := layout.size
		if inlineLocalSize7.Width <= 0 || inlineLocalSize7.Height <= 0 {
			inlineLocalSize7 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: items}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(items), edit: appCommentEdit{index: -1}, width: inlineLocalSize7.Width, height: inlineLocalSize7.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.load = func() ([]commentsComment, error) { return nil, nil }

	next, cmd := m.Update(func() tea.KeyPressMsg {
		var text string = "C"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() appModel {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd != nil, true, "%s",
		"opening comments did not request a refresh",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = cmd()
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
	(func(t *testing.T, got, want appMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.mode, appModeComments, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(m.comments.items), 0, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
}

func TestAppCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := func() appModel {
		var (
			p     patchPatch        = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
			items []commentsComment = []commentsComment{{ID: "keep", Body: "keep"}}
		)
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize14 := layout.size
		if inlineLocalSize14.Width <= 0 || inlineLocalSize14.Height <= 0 {
			inlineLocalSize14 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: items}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(items), edit: appCommentEdit{index: -1}, width: inlineLocalSize14.Width, height: inlineLocalSize14.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.load = func() ([]commentsComment, error) { return nil, errors.New("storage unavailable") }

	next, cmd := m.Update(func() tea.KeyPressMsg {
		var text string = "C"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() appModel {
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
			msg tea.Msg  = cmd()
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
	})(t, len(m.comments.items), 1, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err != nil, true, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err.Error(),
		"refresh comments: storage unavailable", "comments=%#v error=%v",
		m.comments.items, m.err,
	)
}

func TestAppEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := func() appModel {
		var (
			p     patchPatch        = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
			items []commentsComment = []commentsComment{{ID: "one", Body: "old"}}
		)
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize16 := layout.size
		if inlineLocalSize16.Width <= 0 || inlineLocalSize16.Height <= 0 {
			inlineLocalSize16 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: items}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(items), edit: appCommentEdit{index: -1}, width: inlineLocalSize16.Width, height: inlineLocalSize16.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	deleted := false
	m.delete = func(commentsComment, patchPatch) error { deleted = true; return nil }
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
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
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
		next, _ := m.Update(appCommentEditorFinishedMsg{body: "\n"})
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, deleted, true, "deleted=%v comments=%d", deleted,
		len(m.comments.items),
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(m.comments.items), 0, "deleted=%v comments=%d",
		deleted, len(m.comments.items),
	)
}

func TestAppCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := func() appModel {
		var (
			p     patchPatch        = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
			items []commentsComment = []commentsComment{{ID: "one", Body: "keep"}}
		)
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize21 := layout.size
		if inlineLocalSize21.Width <= 0 || inlineLocalSize21.Height <= 0 {
			inlineLocalSize21 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: items}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(items), edit: appCommentEdit{index: -1}, width: inlineLocalSize21.Width, height: inlineLocalSize21.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.delete = func(commentsComment, patchPatch) error { return errors.New("delete failed") }
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "D"
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
	})(t, len(m.comments.items), 1, "comments=%#v render=%q",
		m.comments.items, func() string {
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
		}(),
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(ansi.Strip(func() string {
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
		}()), "delete failed"), true,
		"comments=%#v render=%q", m.comments.items, func() string {
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
		}(),
	)
}

func TestAppSelectionCannotCrossHunk(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize31 := layout.size
		if inlineLocalSize31.Width <= 0 || inlineLocalSize31.Height <= 0 {
			inlineLocalSize31 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize31.Width, height: inlineLocalSize31.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
				var text string = "v"
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
	for range 10 {
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
	}
	_, line, _ := m.diffView.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, line.Text != "more()", true, "%s",
		"diffSelection crossed hunk",
	)
}

func TestAppVimSequencesAndLayoutToggle(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize26 := layout.size
		if inlineLocalSize26.Width <= 0 || inlineLocalSize26.Height <= 0 {
			inlineLocalSize26 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize26.Width, height: inlineLocalSize26.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	var saved []bool
	m.saveLayout = func(enabled bool) error { saved = append(saved, enabled); return nil }
	m.diffOptions.SideBySide = false
	m.diffView.Configure(m.diffOptions)
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
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
				var text string = "G"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "more()", "G line=%q", func() string { _, line, _ := m.diffView.Current(); return line.Text }())

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "g"
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "g"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "gg line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "t"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.diffOptions.SideBySide, true, "%s",
		"split view not enabled",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(func() string {
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
	}(), "│"), true, "%s",
		"split view not enabled",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "t"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !m.diffOptions.SideBySide, true, "saved=%v", saved)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestAppSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize2 := layout.size
		if inlineLocalSize2.Width <= 0 || inlineLocalSize2.Height <= 0 {
			inlineLocalSize2 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize2.Width, height: inlineLocalSize2.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	var saved []bool
	m.saveLayout = func(enabled bool) error { saved = append(saved, enabled); return nil }
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
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
				var text string = "t"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !m.diffOptions.SideBySide, true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, slices.Equal(saved, []bool{false}), true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
}

func TestAppResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize24 := layout.size
		if inlineLocalSize24.Width <= 0 || inlineLocalSize24.Height <= 0 {
			inlineLocalSize24 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize24.Width, height: inlineLocalSize24.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.saveLayout = nil
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	func() {
		var (
			m    *appModel = &m
			text string    = "keep()"
		)
		t.Helper()
		m.diffView.Move(diffscreenFirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreenNextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	_, before, _ := m.diffView.Current()
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
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
	_, got, ok := m.diffView.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "focused line=%#v, want %#v", got,
		before,
	)
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, before, "focused line=%#v, want %#v", got,
		before,
	)
}

func TestAppZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreenAlignment
	}{
		{
			"z", diffscreenCenter,
		}, {"t", diffscreenTop}, {"b", diffscreenBottom},
	} {
		m := func() appModel {
			var p patchPatch = func() patchPatch {
				lines := make([]patchLine, 30)
				for index := range lines {
					lines[index] = patchLine{Kind: patchContext, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patchLineNumber(index + 1), NewNumber: patchLineNumber(index + 1)}
				}
				return patchPatch{Files: []patchFile{{DisplayPath: "long.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
			}()
			var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
			inlineLocalSize4 := layout.size
			if inlineLocalSize4.Width <= 0 || inlineLocalSize4.Height <= 0 {
				inlineLocalSize4 = appDefaultSize
			}
			m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
				v := &commentscreenView{width: 80, height: 30}
				v.Update(items)
				return v
			})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize4.Width, height: inlineLocalSize4.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
			next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 9})
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
		want := (func(p patchPatch, options diffscreenOptions) *diffscreenView {
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
		})(func() patchPatch {
			lines := make([]patchLine, 30)
			for index := range lines {
				lines[index] = patchLine{Kind: patchContext, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patchLineNumber(index + 1), NewNumber: patchLineNumber(index + 1)}
			}
			return patchPatch{Files: []patchFile{{DisplayPath: "long.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
		}(), diffscreenOptions{Dark: true})
		want.Resize(100, 9)
		for range 10 {
			m.diffView.Move(diffscreenNextLine)
			want.Move(diffscreenNextLine)
		}
		want.Align(test.alignment)
		m = func() appModel {
			var (
				m   appModel = m
				msg tea.Msg  = func() tea.KeyPressMsg {
					var text string = "z"
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
			var (
				m   appModel = m
				msg tea.Msg  = func() tea.KeyPressMsg {
					runes := []rune(test.key)
					return tea.KeyPressMsg(tea.Key{Text: test.key, Code: runes[0]})
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
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, m.diffView.Render(nil), want.Render(nil),
			"z%s did not align view", test.key,
		)
	}
}

func TestAppPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize34 := layout.size
		if inlineLocalSize34.Width <= 0 || inlineLocalSize34.Height <= 0 {
			inlineLocalSize34 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize34.Width, height: inlineLocalSize34.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
				var text string = "G"
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
	last := func() string { _, line, _ := m.diffView.Current(); return line.Text }()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "g"
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "h"
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "g"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), last, "%s",
		"pending prefix moved focus",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "g"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "gg line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)
}

func TestAppStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize32 := layout.size
		if inlineLocalSize32.Width <= 0 || inlineLocalSize32.Height <= 0 {
			inlineLocalSize32 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize32.Width, height: inlineLocalSize32.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	status := ansi.Strip(strings.Split(func() string {
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
	}(), "\n")[m.height-2])
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.HasPrefix(status, "j/k/h/l move"), true,
		"status=%q", status,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.HasSuffix(status, "local changes"), true,
		"status=%q", status,
	)

	help := ansi.Strip(func() string {
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
	}())
	for _, binding := range []string{
		"Ctrl-w h/l/w",
		"zz/zt/zb",
		"n/N",
		"]f/[f",
		"Tab",
		"t",
	} {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, strings.Contains(help, binding), true,
			"help missing %q", binding,
		)
	}
}

func TestAppStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := func() appModel {
		var p patchPatch = func() patchPatch {
			lines := make([]patchLine, 30)
			for index := range lines {
				lines[index] = patchLine{Kind: patchContext, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patchLineNumber(index + 1), NewNumber: patchLineNumber(index + 1)}
			}
			return patchPatch{Files: []patchFile{{DisplayPath: "long.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
		}()
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize19 := layout.size
		if inlineLocalSize19.Width <= 0 || inlineLocalSize19.Height <= 0 {
			inlineLocalSize19 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize19.Width, height: inlineLocalSize19.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 9})
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(func() string {
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
	}(), "%"), true, "%s",
		"initial view shows progress",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "l"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(func() string {
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
	}(), "%"), true, "%s",
		"horizontal scroll shows progress",
	)

	for range 10 {
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
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(func() string {
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
	}(), "%)"), true, "%s",
		"scrolled view hides progress",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "G"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(func() string {
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
		}(), "local changes (100%)"), true, "%s",
		"final view hides progress",
	)
}

func TestAppStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize15 := layout.size
		if inlineLocalSize15.Width <= 0 || inlineLocalSize15.Height <= 0 {
			inlineLocalSize15 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize15.Width, height: inlineLocalSize15.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 100})
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(func() string {
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
	}(), "%)"), true, "%s",
		"fitting diff shows progress",
	)
}

func TestAppSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
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
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	func() {
		var (
			m    *appModel = &m
			text string    = "new()"
		)
		t.Helper()
		m.diffView.Move(diffscreenFirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreenNextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl})
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "h"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "old pane line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl})
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
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl})
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "new()", "new pane line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)
}

func TestAppHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := func() appModel {
		var p patchPatch = func() patchPatch {
			lines := make([]patchLine, 30)
			for index := range lines {
				lines[index] = patchLine{Kind: patchContext, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patchLineNumber(index + 1), NewNumber: patchLineNumber(index + 1)}
			}
			return patchPatch{Files: []patchFile{{DisplayPath: "long.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
		}()
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize13 := layout.size
		if inlineLocalSize13.Width <= 0 || inlineLocalSize13.Height <= 0 {
			inlineLocalSize13 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize13.Width, height: inlineLocalSize13.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 37, Height: 30})
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
	initial := m.diffView.Render(nil)
	want := (func(p patchPatch, options diffscreenOptions) *diffscreenView {
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
	})(func() patchPatch {
		lines := make([]patchLine, 30)
		for index := range lines {
			lines[index] = patchLine{Kind: patchContext, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patchLineNumber(index + 1), NewNumber: patchLineNumber(index + 1)}
		}
		return patchPatch{Files: []patchFile{{DisplayPath: "long.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
	}(), diffscreenOptions{Dark: true})
	want.Resize(37, 30)
	want.ScrollHorizontal(2 * appHorizontalScrollStep)
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "l"
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
		next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
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
	})(t, m.diffView.Render(nil), want.Render(nil), "%s",
		"right keys did not scroll by two steps",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "h"
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
		next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
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
	})(t, m.diffView.Render(nil), initial, "%s",
		"left keys did not restore initial offset",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "$"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.diffView.Render(nil) != initial, true, "%s",
		"$ did not scroll",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "0"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.diffView.Render(nil), initial, "%s",
		"0 did not restore initial offset",
	)
}

func TestAppFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize9 := layout.size
		if inlineLocalSize9.Width <= 0 || inlineLocalSize9.Height <= 0 {
			inlineLocalSize9 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize9.Width, height: inlineLocalSize9.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.currentPatch.Branch = "main"
	m.kind = patchBranch
	var requested []patchKind
	m.refresh = func(kind patchKind) (patchPatch, error) {
		requested = append(requested, kind)
		p := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{fmt.Sprintf("refresh-%d", len(requested))}
		return p, nil
	}
	next, cmd := m.Update(tea.FocusMsg{})
	m = func() appModel {
		var next tea.Model = next
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd != nil, true, "%s", "focus did not refresh")

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = cmd()
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
	next, cmd = m.Update(func() tea.KeyPressMsg {
		var text string = "R"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() appModel {
		var next tea.Model = next
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd != nil, true, "%s", "R did not refresh")

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = cmd()
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		slices.Equal(requested, []patchKind{patchBranch, patchBranch}), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"refresh-2"},
		), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
}

func TestAppSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize10 := layout.size
		if inlineLocalSize10.Width <= 0 || inlineLocalSize10.Height <= 0 {
			inlineLocalSize10 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize10.Width, height: inlineLocalSize10.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	refreshed := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.refresh = func(patchKind) (patchPatch, error) { return refreshed, nil }

	next, cmd := m.Update(appSourceEditorFinishedMsg{})
	m = func() appModel {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd != nil, true, "%s",
		"editor completion did not refresh",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = cmd()
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"after-editor"},
		),
		true, "metadata=%q", m.currentPatch.Files[0].Metadata,
	)
}

func TestAppHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(
		ansi.Strip(func() string {
			var m appModel = func() appModel {
				var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
				var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
				inlineLocalSize8 := layout.size
				if inlineLocalSize8.Width <= 0 || inlineLocalSize8.Height <= 0 {
					inlineLocalSize8 = appDefaultSize
				}
				m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
					v := &commentscreenView{width: 80, height: 30}
					v.Update(items)
					return v
				})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize8.Width, height: inlineLocalSize8.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		}()),
		"\n",
		2,
	)[0]
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, header, "review-my-slop  +2-1", "header = %q",
		header,
	)
}

func TestAppSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize20 := layout.size
		if inlineLocalSize20.Width <= 0 || inlineLocalSize20.Height <= 0 {
			inlineLocalSize20 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize20.Width, height: inlineLocalSize20.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	origin := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "/"
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "keep"
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
	first := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	(func(t *testing.T, got, want appMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.mode, appModeSearch, "mode=%v line=%q", m.mode,
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()", "mode=%v line=%q", m.mode,
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "n"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}() != first, true,
		"next=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()", "next=%#v",
		func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "N"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), first,
		"previous=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "/"
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "missing"
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(func() string {
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
	}(), "no matches"), true,
		"%s", "missing search did not miss",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), first,
		"cancel=%#v origin=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(), origin,
	)
}

func TestAppSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
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
	origin := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "/"
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "main.go"
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
	file, _, _ := m.diffView.Current()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.DisplayPath, "main.go", "file=%q",
		file.DisplayPath,
	)

	for range len("main.go") {
		m = func() appModel {
			var (
				m   appModel = m
				msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace})
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
	}
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), origin,
		"diffCursor=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)
}

func TestAppSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize28 := layout.size
		if inlineLocalSize28.Width <= 0 || inlineLocalSize28.Height <= 0 {
			inlineLocalSize28 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize28.Width, height: inlineLocalSize28.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
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
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	origin := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "/"
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
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "old()"
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "diffCursor=%#v line=%q",
		func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(), func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), origin,
		"cancel=%#v want=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(), origin,
	)
}

func TestAppTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize27 := layout.size
		if inlineLocalSize27.Width <= 0 || inlineLocalSize27.Height <= 0 {
			inlineLocalSize27 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize27.Width, height: inlineLocalSize27.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.currentPatch.Branch = "main"
	m.refresh = func(kind patchKind) (patchPatch, error) {
		p := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		p.Kind, p.Branch = kind, "main"
		return p, nil
	}
	next, _ := m.Update(func() tea.KeyPressMsg {
		var text string = "tab"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() appModel {
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
	(func(t *testing.T, got, want patchKind, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.kind, patchBranch, "kind=%v", m.kind)

	stale := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
	stale.Files[0].Metadata = []string{"stale"}
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appRefreshDiffMsg{patch: stale})
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		!slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"stale"},
		), true,
		"%s", "stale refresh applied",
	)

	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = func() tea.KeyPressMsg {
				var text string = "tab"
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
	(func(t *testing.T, got, want patchKind, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.kind, patchUnstaged, "kind=%v after toggling back to local", m.kind)
}

func TestAppTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize17 := layout.size
		if inlineLocalSize17.Width <= 0 || inlineLocalSize17.Height <= 0 {
			inlineLocalSize17 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize17.Width, height: inlineLocalSize17.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	refreshed := false
	m.refresh = func(patchKind) (patchPatch, error) {
		refreshed = true
		return patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}, nil
	}
	next, cmd := m.Update(func() tea.KeyPressMsg {
		var text string = "tab"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() appModel {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, cmd == nil, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !refreshed, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	(func(t *testing.T, got, want patchKind, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.kind, patchUnstaged, "tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
}

func TestAppDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := func() appModel {
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize23 := layout.size
		if inlineLocalSize23.Width <= 0 || inlineLocalSize23.Height <= 0 {
			inlineLocalSize23 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: patchPatch{}, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize23.Width, height: inlineLocalSize23.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: patchPatch{}.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		})(patchPatch{}, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = appRefreshDiffMsg{patch: patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}}
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
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "initial focus=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	func() {
		var (
			m    *appModel = &m
			text string    = "new()"
		)
		t.Helper()
		m.diffView.Move(diffscreenFirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreenNextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	changed := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appRefreshDiffMsg{patch: changed})
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
	if _, _, ok := m.diffView.Current(); !ok {
		t.Fatal("refresh lost focus")
	}
}

func TestAppCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patchPatch
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize25 := layout.size
		if inlineLocalSize25.Width <= 0 || inlineLocalSize25.Height <= 0 {
			inlineLocalSize25 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize25.Width, height: inlineLocalSize25.Height, save: func(stored commentsComment, p patchPatch) (commentsComment, error) { saved = p; return stored, nil }, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	refreshed := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appRefreshDiffMsg{patch: refreshed})
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
	_ = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appCommentEditorFinishedMsg{body: "comment"})
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}), true,
		"metadata=%q", saved.Files[0].Metadata,
	)
}

func TestAppViewPreservesTerminalColors(t *testing.T) {
	result := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}, {Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patchAddition, Text: "more()", NewNumber: 4}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize6 := layout.size
		if inlineLocalSize6.Width <= 0 || inlineLocalSize6.Height <= 0 {
			inlineLocalSize6 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize6.Width, height: inlineLocalSize6.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	}().View()
	(func(t *testing.T, got, want color.Color, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, result.BackgroundColor, nil, "view=%#v", result)
	(func(t *testing.T, got, want color.Color, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, result.ForegroundColor, nil, "view=%#v", result)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, result.AltScreen, true, "view=%#v", result)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, result.ReportFocus, true, "view=%#v", result)
	(func(t *testing.T, got, want tea.MouseMode, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, result.MouseMode, tea.MouseModeCellMotion, "%s",
		"view does not capture terminal wheel events",
	)
}

func TestAppMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := func() appModel {
		var p patchPatch = func() patchPatch {
			lines := make([]patchLine, 30)
			for index := range lines {
				lines[index] = patchLine{Kind: patchContext, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patchLineNumber(index + 1), NewNumber: patchLineNumber(index + 1)}
			}
			return patchPatch{Files: []patchFile{{DisplayPath: "long.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
		}()
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize22 := layout.size
		if inlineLocalSize22.Width <= 0 || inlineLocalSize22.Height <= 0 {
			inlineLocalSize22 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize22.Width, height: inlineLocalSize22.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 37, Height: 9})
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
				var text string = "v"
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
	_, focus, _ := m.diffView.Current()
	_, selected, _ := m.diffView.Selected()
	initial := func() string {
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
	for _, button := range []tea.MouseButton{
		tea.MouseWheelDown,
		tea.MouseWheelRight,
	} {
		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseWheelMsg{Button: button})
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
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
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
	}() != initial, true, "%s",
		"wheel did not scroll content",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(func() string {
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
	}(), "%)"), true, "%s",
		"wheel did not scroll content",
	)

	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, focus, "%s",
		"wheel changed focus or selection",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, slices.Equal(selected, gotSelected), true, "%s",
		"wheel changed focus or selection",
	)

	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseWheelMsg{Button: button})
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
	}
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
	}(), initial, "%s",
		"reverse wheel did not restore content",
	)

	for _, inlineLocalMode1 := range []appMode{appModeComments, appModeHelp} {
		m.mode = inlineLocalMode1
		before := m.diffView.Render(nil)
		m = func() appModel {
			var m appModel = m
			t.Helper()
			next, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
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
		})(t, m.diffView.Render(nil), before, "%s",
			"wheel scrolled diff behind another screen",
		)
	}
}
