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
	strings "strings"
	testing "testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	quick "github.com/alecthomas/chroma/v2/quick"
	ansi "github.com/charmbracelet/x/ansi"
)

func TestAppRunLoadsCommentsAndWiresContext(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := commentsStore{Path: filepath.Join(t.TempDir(), "comments.db")}
	p := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
	pending := commentsComment{Repository: p.Root, Body: "pending"}
	if _, err := store.Add(pending); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sentinel := errors.New("program startup failed")
	err := func() error {
		var refresh func(context.Context, patchKind) (patchPatch, error) = func(
			received context.Context,
			kind patchKind,
		) (patchPatch, error) {
			(func(t *testing.T, got, want context.Context, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, received, ctx, "context=%v kind=%v", received,
				kind,
			)
			(func(t *testing.T, got, want patchKind, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, kind, p.Kind, "context=%v kind=%v", received,
				kind,
			)

			return p, nil
		}
		pending, err := appCommentStore(store).List(p.Root)
		if err != nil {
			return err
		}
		dimensions := func() appSize { return appSize{Width: 120, Height: 40} }()
		m, err := func() (appModel, error) {
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
				}, size: dimensions}
				inlineLocalSize1 := layout.size
				if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
					inlineLocalSize1 = appDefaultSize
				}
				m := appModel{ctx: context.Background(), currentPatch: p, comments: appCommentState{items: pending}, commentView: (func(items []commentsComment) *commentscreenView {
					v := &commentscreenView{width: 80, height: 30}
					v.Update(items)
					return v
				})(pending), edit: appCommentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
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
		if err != nil {
			return err
		}
		m.ctx = ctx
		if refresh != nil {
			m.refresh = func(kind patchKind) (patchPatch, error) { return refresh(ctx, kind) }
		}
		return func(m appModel, _ ...tea.ProgramOption) error {
			(func(t *testing.T, got, want context.Context, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, m.ctx, ctx, "model=%#v", m)
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, m.width, 120, "model=%#v", m)
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, m.height, 40, "model=%#v", m)
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, len(m.comments.items), 1, "model=%#v", m)
			result := func() tea.Cmd {
				if m.refresh == nil {
					return nil
				}
				kind := m.kind
				return func() tea.Msg { p, err := m.refresh(kind); return appRefreshDiffMsg{patch: p, kind: kind, err: err} }
			}()()
			msg, ok := result.(appRefreshDiffMsg)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, ok, true, "refresh message=%T", result)
			(func(t *testing.T, got, want error, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, msg.err, nil, "%s", msg.err)
			return sentinel
		}(m, tea.WithContext(ctx), tea.WithWindowSize(dimensions.Width, dimensions.Height))
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, errors.Is(err, sentinel), true, "error=%v", err)
}

func TestAppRunStartupFailures(t *testing.T) {
	for _, failure := range []string{"comments", "layout"} {
		t.Run(failure, func(t *testing.T) {
			config := t.TempDir()
			t.Setenv("HOME", config)
			t.Setenv("XDG_CONFIG_HOME", config)
			config, err := os.UserConfigDir()
			(func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			})(t, err)
			(func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			})(t, os.MkdirAll(config, 0700))

			store := commentsStore{
				Path: filepath.Join(t.TempDir(), "comments.db"),
			}
			{
				var config string = config
				t.Helper()
				if failure == "comments" {
					(func(t *testing.T, err error) {
						t.Helper()
						if err != nil {
							t.Fatal(err)
						}
					})(t, os.WriteFile(store.Path, []byte("invalid database"), 0600))
				} else {
					(func(t *testing.T, err error) {
						t.Helper()
						if err != nil {
							t.Fatal(err)
						}
					})(t, os.Mkdir(filepath.Join(config, "review-my-slop"), 0700))
					(func(t *testing.T, err error) {
						t.Helper()
						if err != nil {
							t.Fatal(err)
						}
					})(t, os.WriteFile(filepath.Join(config, "review-my-slop", "ui.json"), []byte("invalid json"), 0600))
				}
			}
			err = func() error {
				var (
					ctx          context.Context = context.Background()
					currentPatch patchPatch      = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
				)
				pending, err := appCommentStore(store).List(currentPatch.Root)
				if err != nil {
					return err
				}
				dimensions := func() appSize { return appDefaultSize }()
				m, err := func() (appModel, error) {
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
						}, size: dimensions}
						inlineLocalSize2 := layout.size
						if inlineLocalSize2.Width <= 0 || inlineLocalSize2.Height <= 0 {
							inlineLocalSize2 = appDefaultSize
						}
						m := appModel{ctx: context.Background(), currentPatch: currentPatch, comments: appCommentState{items: pending}, commentView: (func(items []commentsComment) *commentscreenView {
							v := &commentscreenView{width: 80, height: 30}
							v.Update(items)
							return v
						})(pending), edit: appCommentEdit{index: -1}, width: inlineLocalSize2.Width, height: inlineLocalSize2.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
							comment.Repository = current.Root
							if comment.ID != "" {
								if err := appCommentStore(store).Update(comment); err != nil {
									return commentsComment{}, err
								}
								return comment, nil
							}
							return appCommentStore(store).Add(comment)
						}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
						})(currentPatch, m.diffOptions)
						m.diffView.Resize(m.width, m.height)
						m.commentView.Resize(m.width, m.height)
						return m
					}()
					m.delete = func(comment commentsComment, current patchPatch) error {
						return appCommentStore(store).Delete(current.Root, comment.ID)
					}
					m.load = func() ([]commentsComment, error) { return appCommentStore(store).List(currentPatch.Root) }
					return m, nil
				}()
				if err != nil {
					return err
				}
				m.ctx = ctx
				if (func(context.Context, patchKind) (patchPatch, error))(nil) != nil {
					m.refresh = func(kind patchKind) (patchPatch, error) {
						return (func(context.Context, patchKind) (patchPatch, error))(nil)(ctx, kind)
					}
				}
				return func(appModel, ...tea.ProgramOption) error {
					t.Fatal("program started after startup failure")
					return nil
				}(m, tea.WithContext(ctx), tea.WithWindowSize(dimensions.Width, dimensions.Height))
			}()
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, err != nil, true, "%s",
				"expected startup error",
			)
		})
	}
}

func TestAppRunContextCancellation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := func() error {
		var (
			currentPatch patchPatch      = patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patchHunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patchLine{{Kind: patchContext, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "old()", OldNumber: 2}, {Kind: patchAddition, Text: "new()", NewNumber: 2}}}}}}}
			store        appCommentStore = commentsStore{
				Path: filepath.Join(t.TempDir(), "comments.db"),
			}
		)
		pending, err := store.List(currentPatch.Root)
		if err != nil {
			return err
		}
		dimensions := func() appSize { return appDefaultSize }()
		m, err := func() (appModel, error) {
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
				}, size: dimensions}
				inlineLocalSize3 := layout.size
				if inlineLocalSize3.Width <= 0 || inlineLocalSize3.Height <= 0 {
					inlineLocalSize3 = appDefaultSize
				}
				m := appModel{ctx: context.Background(), currentPatch: currentPatch, comments: appCommentState{items: pending}, commentView: (func(items []commentsComment) *commentscreenView {
					v := &commentscreenView{width: 80, height: 30}
					v.Update(items)
					return v
				})(pending), edit: appCommentEdit{index: -1}, width: inlineLocalSize3.Width, height: inlineLocalSize3.Height, save: func(comment commentsComment, current patchPatch) (commentsComment, error) {
					comment.Repository = current.Root
					if comment.ID != "" {
						if err := store.Update(comment); err != nil {
							return commentsComment{}, err
						}
						return comment, nil
					}
					return store.Add(comment)
				}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreenOptions{SideBySide: layout.SideBySide, Dark: true}, kind: currentPatch.Kind, keys: (func(sequences ...keymapSequence) keymapMatcher {
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
				})(currentPatch, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			m.delete = func(comment commentsComment, current patchPatch) error {
				return store.Delete(current.Root, comment.ID)
			}
			m.load = func() ([]commentsComment, error) { return store.List(currentPatch.Root) }
			return m, nil
		}()
		if err != nil {
			return err
		}
		m.ctx = ctx
		if (func(context.Context, patchKind) (patchPatch, error))(nil) != nil {
			m.refresh = func(kind patchKind) (patchPatch, error) {
				return (func(context.Context, patchKind) (patchPatch, error))(nil)(ctx, kind)
			}
		}
		return func(m appModel, options ...tea.ProgramOption) error {
			options = append(options, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer())
			_, err := tea.NewProgram(m, options...).Run()
			return err
		}(m, tea.WithContext(ctx), tea.WithWindowSize(dimensions.Width, dimensions.Height))
	}()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, errors.Is(err, tea.ErrProgramKilled), true,
		"error=%v", err,
	)
}

func TestAppTerminalSizeDiscoveryOrder(t *testing.T) {
	for _, tc := range []struct {
		name           string
		success, calls int
		want           appSize
	}{
		{"stdin", 0, 1, appSize{Width: 120, Height: 40}},
		{"stdout", 1, 2, appSize{Width: 120, Height: 40}},
		{"fallback", 2, 2, appDefaultSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			probe := appTerminalSizeProbe{t: t, success: tc.success}
			got := func() appSize {
				if width, height, err := func() (width, height int, err error) {
					var (
						p  *appTerminalSizeProbe = &probe
						fd uintptr               = os.Stdin.Fd()
					)
					p.t.Helper()
					descriptors := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(p.t, p.calls < len(descriptors), true, "unexpected terminal size call: %d", p.calls)
					(func(t *testing.T, got, want uintptr, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(p.t, fd, descriptors[p.calls], "fd=%v want=%v", fd, descriptors[p.calls])
					current := p.calls
					p.calls++
					if current == p.success {
						return 120, 40, nil
					}
					return 0, 0, errors.New("not a terminal")
				}(); err == nil {
					return appSize{Width: width, Height: height}
				}
				if width, height, err := func() (width, height int, err error) {
					var (
						p  *appTerminalSizeProbe = &probe
						fd uintptr               = os.Stdout.Fd()
					)
					p.t.Helper()
					descriptors := []uintptr{os.Stdin.Fd(), os.Stdout.Fd()}
					(func(t *testing.T, got, want bool, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(p.t, p.calls < len(descriptors), true, "unexpected terminal size call: %d", p.calls)
					(func(t *testing.T, got, want uintptr, format string, args ...any) {
						t.Helper()
						if got != want {
							t.Fatalf(format, args...)
						}
					})(p.t, fd, descriptors[p.calls], "fd=%v want=%v", fd, descriptors[p.calls])
					current := p.calls
					p.calls++
					if current == p.success {
						return 120, 40, nil
					}
					return 0, 0, errors.New("not a terminal")
				}(); err == nil {
					return appSize{Width: width, Height: height}
				}
				return appDefaultSize
			}()
			(func(t *testing.T, got, want appSize, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, got, tc.want, "size=%v want=%v", got,
				tc.want,
			)
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, probe.calls, tc.calls, "calls=%d want=%d",
				probe.calls, tc.calls,
			)
		})
	}
}

type appTerminalSizeProbe struct {
	t              *testing.T
	success, calls int
}
