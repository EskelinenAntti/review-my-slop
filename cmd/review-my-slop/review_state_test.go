package main

import (
	bytes "bytes"
	context "context"
	json "encoding/json"
	errors "errors"
	fmt "fmt"
	io "io"
	os "os"
	filepath "path/filepath"
	slices "slices"
	strings "strings"
	testing "testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestAppStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := func() (appModel, error) {
		var (
			p                patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
			inlineLocalSize4 appSize    = appDefaultSize
		)
		preferences, err := (func() (settingsPreferences, error) {
			path, err := (func() (string, error) {
				config, err := os.UserConfigDir()
				if err != nil {
					return "", fmt.Errorf("resolve UI settings directory: %w", err)
				}
				return filepath.Join(config, "review-my-slop", "ui.json"), nil
			})()
			if err != nil {
				return settingsPreferences{}, err
			}
			return (func(path string) (settingsPreferences, error) {
				data, err := os.ReadFile(path)
				if errors.Is(err, os.ErrNotExist) {
					return settingsPreferences{}, nil
				}
				if err != nil {
					return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
				}
				var settings settingsPreferences
				if err := json.Unmarshal(data, &settings); err != nil {
					return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
				}
				return settings, nil
			})(path)
		})()
		if err != nil {
			return appModel{}, err
		}
		m := func() appModel {
			var layout appInitialLayout = appInitialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error {
				return (func(preferences settingsPreferences) error {
					path, err := (func() (string, error) {
						config, err := os.UserConfigDir()
						if err != nil {
							return "", fmt.Errorf("resolve UI settings directory: %w", err)
						}
						return filepath.Join(config, "review-my-slop", "ui.json"), nil
					})()
					if err != nil {
						return err
					}
					return (func(path string, preferences settingsPreferences) error {
						if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
							return fmt.Errorf("create UI settings directory: %w", err)
						}
						data, err := json.Marshal(preferences)
						if err != nil {
							return fmt.Errorf("encode UI settings: %w", err)
						}
						data = append(data, '\n')
						temporary, err := os.CreateTemp(filepath.Dir(path), "ui-*.tmp")
						if err != nil {
							return fmt.Errorf("create UI settings file: %w", err)
						}
						temporaryPath := temporary.Name()
						defer os.Remove(temporaryPath)
						if err := temporary.Chmod(0o600); err != nil {
							_ = temporary.Close()
							return fmt.Errorf("secure UI settings file: %w", err)
						}
						if _, err := temporary.Write(data); err != nil {
							_ = temporary.Close()
							return fmt.Errorf("write UI settings: %w", err)
						}
						if err := temporary.Close(); err != nil {
							return fmt.Errorf("close UI settings: %w", err)
						}
						if err := os.Rename(temporaryPath, path); err != nil {
							return fmt.Errorf("replace UI settings: %w", err)
						}
						return nil
					})(path, preferences)
				})(settingsPreferences{SideBySide: enabled})
			}, size: inlineLocalSize4}
			inlineLocalSize11 := layout.size
			if inlineLocalSize11.Width <= 0 || inlineLocalSize11.Height <= 0 {
				inlineLocalSize11 = appDefaultSize
			}
			m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
				v := &commentscreenView{width: 80, height: 30}
				v.Update(items)
				return v
			})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize11.Width, height: inlineLocalSize11.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
				comment.Repository = current.Root
				if comment.ID != "" {
					if err := appCommentStore(store).Update(comment); err != nil {
						return commentsComment{}, err
					}
					return comment, nil
				}
				return appCommentStore(store).Add(comment)
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
		m.delete = func(comment commentsComment, current patchPatch) error {
			return appCommentStore(store).Delete(current.Root, comment.ID)
		}
		m.load = func() ([]commentsComment, error) { return appCommentStore(store).List(p.Root) }
		return m, nil
	}()
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	anchor := commentsAnchor{
		FilePath:    "main.go",
		NewStart:    2,
		QuotedLines: []string{"+new()"},
	}
	m.edit.body = "check this"
	m.edit.anchor = anchor
	func() {
		body := strings.TrimSpace(m.edit.body)
		if body == "" {
			if m.edit.index >= 0 {
				func() {
					var index int = m.edit.index
					if index < 0 || index >= len(m.comments.items) {
						return
					}
					if m.delete == nil {
						m.err = errors.New("comment storage is unavailable")
						return
					}
					if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
						m.err = err
						return
					}
					m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
					m.commentView.Update(m.comments.items)
					m.comments.revision++
					m.err = nil
				}()
			}
			m.edit = appCommentEdit{index: -1}
			m.diffView.ClearSelection()
			return
		}
		if m.save == nil {
			m.err = errors.New("comment storage is unavailable")
			m.edit = appCommentEdit{index: -1}
			return
		}
		var comment commentsComment
		if m.edit.index >= 0 {
			comment = m.comments.items[m.edit.index]
			comment.Body = body
		} else {
			comment = commentsComment{Anchor: m.edit.anchor, Body: body}
		}
		saved, err := m.save(comment, m.currentPatch)
		if err != nil {
			m.err = err
			m.edit = appCommentEdit{index: -1}
			return
		}
		if m.edit.index >= 0 {
			m.comments.items[m.edit.index] = saved
		} else {
			m.comments.items = append(m.comments.items, saved)
			m.commentView.Update(m.comments.items)
			m.commentView.Move(len(m.comments.items))
		}
		m.comments.revision++
		m.commentView.Update(m.comments.items)
		m.edit = appCommentEdit{index: -1}
		m.err = nil
		m.diffView.ClearSelection()
	}()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err, nil, "%s", m.err)

	loaded, err := m.load()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "loaded = %#v, error = %v", loaded, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(loaded), 1, "loaded = %#v, error = %v", loaded,
		err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, loaded[0].Repository, "/repo",
		"loaded = %#v, error = %v", loaded, err,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, loaded[0].ID != "", true,
		"loaded = %#v, error = %v", loaded, err,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		slices.Equal(loaded[0].Anchor.QuotedLines, anchor.QuotedLines), true,
		"loaded = %#v, error = %v", loaded, err,
	)

	id := loaded[0].ID
	m.edit.index = 0
	m.edit.body = "edited"
	func() {
		body := strings.TrimSpace(m.edit.body)
		if body == "" {
			if m.edit.index >= 0 {
				func() {
					var index int = m.edit.index
					if index < 0 || index >= len(m.comments.items) {
						return
					}
					if m.delete == nil {
						m.err = errors.New("comment storage is unavailable")
						return
					}
					if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
						m.err = err
						return
					}
					m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
					m.commentView.Update(m.comments.items)
					m.comments.revision++
					m.err = nil
				}()
			}
			m.edit = appCommentEdit{index: -1}
			m.diffView.ClearSelection()
			return
		}
		if m.save == nil {
			m.err = errors.New("comment storage is unavailable")
			m.edit = appCommentEdit{index: -1}
			return
		}
		var comment commentsComment
		if m.edit.index >= 0 {
			comment = m.comments.items[m.edit.index]
			comment.Body = body
		} else {
			comment = commentsComment{Anchor: m.edit.anchor, Body: body}
		}
		saved, err := m.save(comment, m.currentPatch)
		if err != nil {
			m.err = err
			m.edit = appCommentEdit{index: -1}
			return
		}
		if m.edit.index >= 0 {
			m.comments.items[m.edit.index] = saved
		} else {
			m.comments.items = append(m.comments.items, saved)
			m.commentView.Update(m.comments.items)
			m.commentView.Move(len(m.comments.items))
		}
		m.comments.revision++
		m.commentView.Update(m.comments.items)
		m.edit = appCommentEdit{index: -1}
		m.err = nil
		m.diffView.ClearSelection()
	}()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err, nil, "%s", m.err)

	loaded, err = m.load()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "edited = %#v, error = %v", loaded, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(loaded), 1, "edited = %#v, error = %v", loaded,
		err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, loaded[0].ID, id, "edited = %#v, error = %v",
		loaded, err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, loaded[0].Body, "edited",
		"edited = %#v, error = %v", loaded, err,
	)

	func() {
		if 0 < 0 || 0 >= len(m.comments.items) {
			return
		}
		if m.delete == nil {
			m.err = errors.New("comment storage is unavailable")
			return
		}
		if err := m.delete(m.comments.items[0], m.currentPatch); err != nil {
			m.err = err
			return
		}
		m.comments.items = append(m.comments.items[:0], m.comments.items[0+1:]...)
		m.commentView.Update(m.comments.items)
		m.comments.revision++
		m.err = nil
	}()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err, nil, "%s", m.err)

	loaded, err = m.load()
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, err, nil, "remaining = %#v, error = %v", loaded,
		err,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(loaded), 0, "remaining = %#v, error = %v",
		loaded, err,
	)
}

func TestAppNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := func() appModel {
		var (
			p      patchPatch       = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
			layout appInitialLayout = appInitialLayout{
				SideBySide: true,
				size:       appSize{Width: 120, Height: 30},
			}
		)
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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
	}(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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

func TestAppNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := func() appModel {
		var (
			p      patchPatch       = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
			layout appInitialLayout = appInitialLayout{
				SideBySide: true,
				size:       appSize{Width: 80, Height: 30},
			}
		)
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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
	}(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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
	}(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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

func TestAppSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := func() appModel {
		var (
			p      patchPatch       = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
			layout appInitialLayout = appInitialLayout{
				SaveSideBySide: func(enabled bool) error {
					saved = append(saved, enabled)
					return nil
				},
				size: appSize{Width: 120, Height: 30},
			}
		)
		inlineLocalSize7 := layout.size
		if inlineLocalSize7.Width <= 0 || inlineLocalSize7.Height <= 0 {
			inlineLocalSize7 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize7.Width, height: inlineLocalSize7.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	func() appModel {
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
	})(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestAppRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
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
	m.diffView.Move(diffscreenNextLine)
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreenNextLine)
	_, want, _ := m.diffView.Current()
	refreshed := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.currentPatch = refreshed
	m.diffView.Update(refreshed)
	_, got, ok := m.diffView.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "focused line=%#v, want %#v", got, want)
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, want, "focused line=%#v, want %#v", got, want)

	_, lines, ok := m.diffView.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "selection=%#v", lines)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 2, "selection=%#v", lines)
}

func TestAppUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := func() patchPatch {
		lines := make([]patchLine, 30)
		for index := range lines {
			lines[index] = patchLine{Kind: patchContext, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patchLineNumber(index + 1), NewNumber: patchLineNumber(index + 1)}
		}
		return patchPatch{Files: []patchFile{{DisplayPath: "long.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
	}()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := func() appModel {
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 40, Height: 8}}
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
	for range 8 {
		m.diffView.Move(diffscreenNextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreenNextLine)
	m.diffView.ScrollHorizontal(12)
	before := m.diffView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = func() appModel {
		var m appModel = m
		t.Helper()
		next, _ := m.Update(appRefreshDiffMsg{patch: p})
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
	if got := m.diffView.Render(nil); got != before {
		t.Fatal("refresh changed presentation state")
	}
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err, nil, "refresh did not clear error: %v",
		m.err,
	)
}

func TestAppRefreshFailureRetainsView(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
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
	before := m.diffView.Render(nil)
	m = func() appModel {
		var (
			m   appModel = m
			msg tea.Msg  = appRefreshDiffMsg{err: errors.New("git failed")}
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
	})(t, m.err != nil, true, "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.diffView.Render(nil), before,
		"patch=%#v error=%v", m.currentPatch, m.err,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.currentPatch.Root, "/repo", "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
}

func TestAppViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
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
	m.diffView.Move(diffscreenNextLine)
	m.diffView.Move(diffscreenNextLine)
	_, want, _ := m.diffView.Current()
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	_, got, ok := m.diffView.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "focused line=%#v, want %#v", got, want)
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, want, "focused line=%#v, want %#v", got, want)
}

func TestAppCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patchPatch
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize15 := layout.size
		if inlineLocalSize15.Width <= 0 || inlineLocalSize15.Height <= 0 {
			inlineLocalSize15 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize15.Width, height: inlineLocalSize15.Height, save: func(stored commentsComment, p patchPatch) (commentsComment, error) {
			savedPatch = p
			stored.ID = "1"
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
	m.edit.body = "comment"
	m.edit.anchor = commentsAnchor{FilePath: "main.go"}
	func() {
		body := strings.TrimSpace(m.edit.body)
		if body == "" {
			if m.edit.index >= 0 {
				func() {
					var index int = m.edit.index
					if index < 0 || index >= len(m.comments.items) {
						return
					}
					if m.delete == nil {
						m.err = errors.New("comment storage is unavailable")
						return
					}
					if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
						m.err = err
						return
					}
					m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
					m.commentView.Update(m.comments.items)
					m.comments.revision++
					m.err = nil
				}()
			}
			m.edit = appCommentEdit{index: -1}
			m.diffView.ClearSelection()
			return
		}
		if m.save == nil {
			m.err = errors.New("comment storage is unavailable")
			m.edit = appCommentEdit{index: -1}
			return
		}
		var comment commentsComment
		if m.edit.index >= 0 {
			comment = m.comments.items[m.edit.index]
			comment.Body = body
		} else {
			comment = commentsComment{Anchor: m.edit.anchor, Body: body}
		}
		saved, err := m.save(comment, m.currentPatch)
		if err != nil {
			m.err = err
			m.edit = appCommentEdit{index: -1}
			return
		}
		if m.edit.index >= 0 {
			m.comments.items[m.edit.index] = saved
		} else {
			m.comments.items = append(m.comments.items, saved)
			m.commentView.Update(m.comments.items)
			m.commentView.Move(len(m.comments.items))
		}
		m.comments.revision++
		m.commentView.Update(m.comments.items)
		m.edit = appCommentEdit{index: -1}
		m.err = nil
		m.diffView.ClearSelection()
	}()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, savedPatch.Root, "/repo",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(m.comments.items), 1,
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.comments.items[0].Anchor.FilePath, "main.go",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
}

func TestAppRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize12 := layout.size
		if inlineLocalSize12.Width <= 0 || inlineLocalSize12.Height <= 0 {
			inlineLocalSize12 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize12.Width, height: inlineLocalSize12.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	rendered := func() string {
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
	for _, value := range []string{
		"review-my-slop",
		"+1-1",
		"old()",
		"new()",
		"local changes",
	} {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, strings.Contains(rendered, value), true,
			"render missing %q: %q", value, rendered,
		)
	}
	m.mode = appModeHelp
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
	}(), "Ctrl-w h/l/w"), true,
		"%s", "help lost diffPane binding",
	)
}

func TestAppEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := func() appModel {
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize14 := layout.size
		if inlineLocalSize14.Width <= 0 || inlineLocalSize14.Height <= 0 {
			inlineLocalSize14 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: patchPatch{}, comments: appCommentState{items: nil}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(nil), edit: appCommentEdit{index: -1}, width: inlineLocalSize14.Width, height: inlineLocalSize14.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: patchPatch{}.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)

	lines := strings.Split(func() string {
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
	}(), "\n")
	if got, want := lines[m.height-2], "j/k/h/l move"; !strings.Contains(
		got,
		want,
	) {
		t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, want)
	}
	if got := lines[2]; !strings.Contains(
		got,
		"No unstaged or untracked changes.",
	) {
		t.Fatalf("empty-viewState line = %q", got)
	}
}

func TestAppMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := func() appModel {
		var (
			p     patchPatch        = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
			items []commentsComment = []commentsComment{
				{
					Body:   "first",
					Anchor: commentsAnchor{FilePath: "main.go", NewStart: 2},
				},
			}
		)
		var layout appInitialLayout = appInitialLayout{size: appSize{Width: 100, Height: 30}}
		inlineLocalSize6 := layout.size
		if inlineLocalSize6.Width <= 0 || inlineLocalSize6.Height <= 0 {
			inlineLocalSize6 = appDefaultSize
		}
		m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: items}, commentView: (func(items []commentsComment) *commentscreenView {
			v := &commentscreenView{width: 80, height: 30}
			v.Update(items)
			return v
		})(items), edit: appCommentEdit{index: -1}, width: inlineLocalSize6.Width, height: inlineLocalSize6.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)

	tests := []struct {
		name             string
		inlineLocalMode1 appMode
		hint             string
	}{
		{name: "comments", inlineLocalMode1: appModeComments, hint: "j/k move"},
		{name: "help", inlineLocalMode1: appModeHelp, hint: "? or Esc closes help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m.mode = test.inlineLocalMode1
			lines := strings.Split(func() string {
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
			}(), "\n")
			if got := lines[m.height-2]; !strings.Contains(got, test.hint) {
				t.Fatalf(
					"line %d = %q, want it to contain %q",
					m.height-1,
					got,
					test.hint,
				)
			}
		})
	}
}

func TestAppCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]commentsComment, 10)
	for index := range items {
		items[index] = commentsComment{
			Body:   fmt.Sprintf("comment %d", index),
			Anchor: commentsAnchor{FilePath: "main.go"},
		}
	}
	m := func() appModel {
		var p patchPatch = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
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
	m.width, m.height = 80, 7
	m.mode = appModeComments
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(func() string {
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
	}()), "\n")
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(
			strings.Join(rendered[1:m.height-2], "\n"),
			"comment 9",
		),
		true, "selected comment is outside the screen body: %q", rendered,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(rendered[m.height-2], "j/k move"),
		true, "footer line = %q", rendered[m.height-2],
	)
}

func TestAppNarrowLayoutToggleSavesPreferenceWithoutApplicationGeometryRules(
	t *testing.T,
) {
	var saved []bool
	m := func() appModel {
		var (
			p      patchPatch       = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
			layout appInitialLayout = appInitialLayout{
				size: appSize{Width: 80, Height: 30},
				SaveSideBySide: func(enabled bool) error {
					saved = append(saved, enabled)
					return nil
				},
			}
		)
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
	(func(t *testing.T, got, want error, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, m.err, nil,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, func() string {
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
	})(t, slices.Equal(saved, []bool{true}), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, func() string {
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
	}(), " │ "), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, func() string {
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
	}(), " │ "), true, "%s",
		"widening did not activate preferred layout",
	)
}
