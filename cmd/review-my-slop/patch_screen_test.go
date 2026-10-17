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

func TestDiffscreenSelectedDefaultsToCurrentAndMovementExtendsRange(t *testing.T) {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, diffscreenOptions{Dark: true})
	file, lines, ok := v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "default selection=%#v %#v %v", file,
		lines, ok,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.NewPath, "first.go",
		"default selection=%#v %#v %v", file, lines, ok,
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 1, "default selection=%#v %#v %v",
		file, lines, ok,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, lines[0].Text, "before",
		"default selection=%#v %#v %v", file, lines, ok,
	)

	v.ToggleSelection()
	v.Move(diffscreenNextLine)
	_, lines, ok = v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "extended selection=%#v", lines)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 2, "extended selection=%#v", lines)

	before := v.cursor
	for range 20 {
		v.Move(diffscreenNextLine)
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor.row >= before.row, true, "%s",
		"selection moved backward",
	)

	file, _, _ = v.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, file.NewPath, "first.go", "%s",
		"selection crossed file",
	)

	v.ClearSelection()
	_, lines, ok = v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "cleared selection=%#v", lines)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 1, "cleared selection=%#v", lines)
}

func TestDiffscreenUpdateAndConfigurePreserveState(t *testing.T) {
	p := patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}
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
	})(p, diffscreenOptions{Dark: true})
	v.Resize(120, 10)
	v.Move(diffscreenNextLine)
	v.ToggleSelection()
	v.Move(diffscreenNextLine)
	_, want, _ := v.Current()
	p.Files[0].Metadata = []string{"new metadata"}
	v.Update(p)
	_, got, ok := v.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "updated current=%#v, want %#v", got,
		want,
	)
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, want, "updated current=%#v, want %#v", got,
		want,
	)

	_, lines, ok := v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "updated selection=%#v", lines)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(lines), 2, "updated selection=%#v", lines)

	v.Configure(diffscreenOptions{SideBySide: true, Dark: false})
	_, got, ok = v.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "configured current=%#v, want %#v", got,
		want,
	)
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, want, "configured current=%#v, want %#v", got,
		want,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(v.Render(nil), " │ "), true,
		"%s", "split layout not rendered",
	)

	before := v.Render(nil)
	v.Update(p)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.Render(nil), before, "%s",
		"unchanged refresh changed presentation",
	)
}

func TestDiffscreenNarrowLayoutFallbackPreservesPreferenceAndPosition(t *testing.T) {
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
	})(p, diffscreenOptions{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	for range 10 {
		v.Move(diffscreenNextLine)
	}
	v.Align(diffscreenCenter)
	offset := v.cursor.row - v.viewport.top
	_, want, _ := v.Current()
	v.Resize(80, 10)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(v.Render(nil), " │ "), true,
		"%s", "narrow layout remained split",
	)

	v.Resize(120, 10)
	_, got, _ := v.Current()
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, want, "%s", "resize lost meaningful position")
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor.row-v.viewport.top, offset, "%s",
		"resize lost meaningful position",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(v.Render(nil), " │ "), true,
		"%s", "resize lost split preference",
	)
}

func TestDiffscreenPageMovementDoesNotPartiallyChangeRejectedSelection(t *testing.T) {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, diffscreenOptions{Dark: true})
	v.Resize(80, 30)
	v.ToggleSelection()
	beforeCursor, beforeViewport := v.cursor, v.viewport
	v.Move(diffscreenNextPage)
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, beforeCursor, "%s",
		"rejected selection partially changed state",
	)
	(func(t *testing.T, got, want diffscreenDiffViewport, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.viewport, beforeViewport, "%s",
		"rejected selection partially changed state",
	)
}

func TestDiffscreenSearchPreviewAcceptRepeatAndCancel(t *testing.T) {
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
	})(patchPatch{Root: "/repo", Files: []patchFile{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{{Kind: patchContext, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patchDeletion, Text: "removed one", OldNumber: 2}, {Kind: patchDeletion, Text: "removed two", OldNumber: 3}, {Kind: patchAddition, Text: "added one", NewNumber: 2}, {Kind: patchContext, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patchHunk{{Header: "@@ -1 +1 @@", Lines: []patchLine{{Kind: patchAddition, Text: "other", NewNumber: 1}}}}}}}, diffscreenOptions{SideBySide: true, Dark: true})
	v.Resize(120, 10)
	origin := v.cursor
	v.BeginSearch()
	v.InsertSearch("removed")
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "deletion not found",
	)
	(func(t *testing.T, got, want diffscreenDiffPane, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor.pane, diffscreenLeft, "%s",
		"search did not activate deletion pane",
	)

	first := v.cursor
	v.AcceptSearch()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.Find(diffscreenForward), true, "%s",
		"next match not found",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor != first, true, "%s",
		"next match not found",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.Find(diffscreenBackward), true, "%s",
		"previous match not found",
	)
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, first, "%s", "previous match not found")

	v.BeginSearch()
	v.InsertSearch("missing")
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "missing query matched",
	)

	v.CancelSearch()
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, first, "%s",
		"cancel did not restore focus",
	)

	v.BeginSearch()
	v.InsertSearch("before")
	for range len("before") {
		v.BackspaceSearch()
	}
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, first, "%s",
		"empty query did not restore focus",
	)

	v.CancelSearch()
	v.Move(diffscreenFirstLine)
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, origin, "%s", "first line changed")
}

func TestDiffscreenSearchOriginSurvivesPatchAndLayoutChanges(t *testing.T) {
	p := (func() patchPatch {
		return patchPatch{Root: "/repo", Files: []patchFile{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{
					{
						Kind:      patchContext,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patchDeletion, Text: "removed one", OldNumber: 2},
					{Kind: patchDeletion, Text: "removed two", OldNumber: 3},
					{Kind: patchAddition, Text: "added one", NewNumber: 2},
					{
						Kind:      patchContext,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patchHunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patchLine{
							{Kind: patchAddition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})()
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
	})(p, diffscreenOptions{Dark: true})
	v.Move(diffscreenNextLine)
	_, origin, _ := v.Current()
	v.BeginSearch()
	v.InsertSearch("added one")
	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(diffscreenOptions{SideBySide: true, Dark: true})
	v.CancelSearch()
	_, got, ok := v.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "restored source=%#v, want %#v", got,
		origin,
	)
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, got, origin, "restored source=%#v, want %#v", got,
		origin,
	)
}

func TestDiffscreenEmptyAndMetadataOnlyPatches(t *testing.T) {
	for _, p := range []patchPatch{
		{},
		{
			Files: []patchFile{
				{
					DisplayPath: "binary",
					Metadata:    []string{"Binary files differ"},
				},
			},
		},
	} {
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
		})(p, diffscreenOptions{})
		v.Resize(10, 1)
		for motion := diffscreenPreviousLine; motion <= diffscreenOtherPane; motion++ {
			v.Move(motion)
		}
		v.Align(diffscreenCenter)
		v.ToggleSelection()
		v.ScrollHorizontal(100)
		v.BeginSearch()
		v.InsertSearch("anything")
		v.CancelSearch()
		if _, _, ok := v.Current(); ok {
			t.Fatal("empty view returned focused code")
		}
		if _, _, ok := v.Selected(); ok {
			t.Fatal("empty view returned selected code")
		}
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, !v.Find(diffscreenForward), true, "%s",
			"empty view found a match",
		)

		_ = v.Render(nil)
	}
}

func TestDiffscreenHorizontalScrollEdgesAreIdempotent(t *testing.T) {
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
	})(), diffscreenOptions{})
	v.Resize(30, 10)
	distance := int(^uint(0) >> 1)
	v.ScrollHorizontal(distance)
	before := v.viewport.LeftColumn
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, before != 0, true, "%s",
		"fixture has no horizontal overflow",
	)

	v.ScrollHorizontal(distance)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.viewport.LeftColumn, before, "%s",
		"scroll end overflowed",
	)

	v.ScrollHorizontal(-distance)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.viewport.LeftColumn, 0, "%s",
		"scroll start not clamped",
	)
}

func TestDiffscreenVerticalScrollKeepsFocusInViewport(t *testing.T) {
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
		})(p, diffscreenOptions{SideBySide: split})
		v.Resize(120, 10)
		v.ToggleSelection()
		for range 4 {
			v.Move(diffscreenNextLine)
		}
		cursor, selection := v.cursor, *v.selection
		before := v.Render(nil)
		v.ScrollVertical(3)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.viewport.top, 3, "%s",
			"scroll did not move content",
		)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.Render(nil) != before, true, "%s",
			"scroll did not move content",
		)
		(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.cursor, cursor, "%s",
			"scroll moved focus while it was still visible",
		)
		(func(t *testing.T, got, want diffscreenDiffSelection, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, *v.selection, selection, "%s",
			"scroll moved focus while it was still visible",
		)

		v.ScrollVertical(6)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.cursor.row, v.viewport.top, "%s",
			"scroll did not clamp focus and extend selection",
		)
		(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.selection.Last, v.cursor, "%s",
			"scroll did not clamp focus and extend selection",
		)
		(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.selection.First, selection.First, "%s",
			"scroll did not clamp focus and extend selection",
		)

		before = v.Render(nil)
		v.Update(p)
		(func(t *testing.T, got, want string, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.Render(nil), before, "%s",
			"refresh reset scrolled content",
		)

		v.Resize(120, 11)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.viewport.top, 9, "%s",
			"resize reset scrolled content",
		)

		v.Move(diffscreenNextLine)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, func(v *diffscreenDiffView, viewport diffscreenDiffViewport, cursor diffscreenDiffCursor) bool {
			return cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
				var current diffscreenEntry = v.rows[cursor.row]
				if current.kind != diffscreenLineRow {
					return -1
				}
				if !v.split || cursor.pane == diffscreenRight {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0 && cursor.row >= viewport.top && cursor.row < viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}()
		}(v.view, v.viewport, v.cursor),
			true, "%s", "keyboard navigation did not reveal focus",
		)

		v.ScrollVertical(-8)
		(func(t *testing.T, got, want int, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, v.cursor.row,
			v.viewport.top+func(v *diffscreenDiffView, viewport diffscreenDiffViewport) int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
					height--
				}
				return max(1, height)
			}(v.view, v.viewport)-1, "%s",
			"scroll up did not clamp focus to bottom of viewport",
		)
	}
}

func TestDiffscreenVerticalScrollDoesNotCrossSelectionHunk(t *testing.T) {
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
		return patchPatch{Root: "/repo", Files: []patchFile{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{
					{
						Kind:      patchContext,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patchDeletion, Text: "removed one", OldNumber: 2},
					{Kind: patchDeletion, Text: "removed two", OldNumber: 3},
					{Kind: patchAddition, Text: "added one", NewNumber: 2},
					{
						Kind:      patchContext,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patchHunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patchLine{
							{Kind: patchAddition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})(), diffscreenOptions{})
	v.Resize(80, 6)
	v.ToggleSelection()
	cursor, viewport, selection := v.cursor, v.viewport, *v.selection
	v.ScrollVertical(100)
	(func(t *testing.T, got, want diffscreenDiffCursor, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.cursor, cursor, "%s",
		"scroll partially changed selection across hunks",
	)
	(func(t *testing.T, got, want diffscreenDiffViewport, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.viewport, viewport, "%s",
		"scroll partially changed selection across hunks",
	)
	(func(t *testing.T, got, want diffscreenDiffSelection, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, *v.selection, selection, "%s",
		"scroll partially changed selection across hunks",
	)
}

func TestDiffscreenVerticalScrollClampsAtContentEdges(t *testing.T) {
	for _, p := range []patchPatch{(func() patchPatch {
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
	})(), (func() patchPatch {
		return patchPatch{Root: "/repo", Files: []patchFile{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{
					{
						Kind:      patchContext,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patchDeletion, Text: "removed one", OldNumber: 2},
					{Kind: patchDeletion, Text: "removed two", OldNumber: 3},
					{Kind: patchAddition, Text: "added one", NewNumber: 2},
					{
						Kind:      patchContext,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patchHunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patchLine{
							{Kind: patchAddition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})(), {}} {
		for _, height := range []int{1, 10, 100} {
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
			})(p, diffscreenOptions{})
			v.Resize(80, height)
			distance := int(^uint(0) >> 1)
			v.ScrollVertical(distance)
			end := v.viewport.top
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t,
				len(v.view.rows) <= v.viewport.Height || end != 0, true, "%s",
				"overflowing content did not scroll",
			)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(
				t,
				len(v.view.rows) <= 0 ||
					end+func(v *diffscreenDiffView, viewport diffscreenDiffViewport) int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != diffscreenFileRow {
							height--
						}
						return max(1, height)
					}(v.view, v.viewport) >= len(v.view.rows),
				true,
				"%s",
				"scroll did not reach final row",
			)

			v.ScrollVertical(distance)
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, v.viewport.top, end, "%s",
				"scroll past end changed viewport",
			)

			v.ScrollVertical(-distance)
			v.ScrollVertical(-distance)
			(func(t *testing.T, got, want int, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, v.viewport.top, 0, "%s",
				"scroll start not clamped",
			)
		}
	}
}

func TestDiffscreenSearchEditingOwnsUnicodeQueryAcrossUpdates(t *testing.T) {
	p := (func() patchPatch {
		return patchPatch{Root: "/repo", Files: []patchFile{
			{
				DisplayPath: "first.go",
				OldPath:     "first.go",
				NewPath:     "first.go",
				Hunks: []patchHunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patchLine{
					{
						Kind:      patchContext,
						Text:      "before",
						OldNumber: 1,
						NewNumber: 1,
					},
					{Kind: patchDeletion, Text: "removed one", OldNumber: 2},
					{Kind: patchDeletion, Text: "removed two", OldNumber: 3},
					{Kind: patchAddition, Text: "added one", NewNumber: 2},
					{
						Kind:      patchContext,
						Text:      "after",
						OldNumber: 4,
						NewNumber: 3,
					},
				}}},
			},
			{
				DisplayPath: "second.go",
				OldPath:     "second.go",
				NewPath:     "second.go",
				Hunks: []patchHunk{
					{
						Header: "@@ -1 +1 @@",
						Lines: []patchLine{
							{Kind: patchAddition, Text: "other", NewNumber: 1},
						},
					},
				},
			},
		}}
	})()
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
	})(p, diffscreenOptions{Dark: true})
	_, origin, _ := v.Current()
	v.InsertSearch("rem")
	v.InsertSearch("oved界")
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(v.Render(nil), "/removed界"),
		true, "%s", "query chunks were not appended",
	)

	v.BackspaceSearch()
	_, found, ok := v.Current()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "%s",
		"Unicode backspace did not restore a matching query",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(found.Text, "removed"), true,
		"%s", "Unicode backspace did not restore a matching query",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(v.Render(nil), "no matches"),
		true, "%s", "Unicode backspace did not restore a matching query",
	)

	p.Files[0].Metadata = []string{"inserted"}
	v.Update(p)
	v.Resize(120, 10)
	v.Configure(diffscreenOptions{SideBySide: true, Dark: true})
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(v.Render(nil), "/removed"), true,
		"%s", "reconfiguration lost the active query",
	)

	v.CancelSearch()
	_, restored, _ := v.Current()
	(func(t *testing.T, got, want patchLine, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, restored, origin, "%s",
		"cancel did not restore source focus",
	)

	v.BeginSearch()
	v.BackspaceSearch()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !strings.Contains(v.Render(nil), "/removed"), true,
		"%s", "new search retained the previous query",
	)
}
