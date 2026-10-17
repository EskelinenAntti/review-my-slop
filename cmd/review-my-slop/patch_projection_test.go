package main

import (
	bytes "bytes"
	io "io"
	slices "slices"
	strings "strings"
	testing "testing"

	quick "github.com/alecthomas/chroma/v2/quick"
)

func TestDiffscreenSplitRowsPairOnlyAdjacentChangeBlocks(t *testing.T) {
	code := func(kind patchLineKind, text string) patchLine {
		return patchLine{Kind: kind, Text: text}
	}
	d1, d2 := code(patchDeletion, "old-a"), code(patchDeletion, "old-b")
	a1, a2 := code(patchAddition, "new-a"), code(patchAddition, "new-b")
	context := code(patchContext, "unchanged")
	for _, tc := range []struct {
		name  string
		lines []patchLine
		want  [][2]string
	}{
		{
			"additions",
			[]patchLine{a1, a2},
			[][2]string{{"", "new-a"}, {"", "new-b"}},
		},
		{
			"deletions",
			[]patchLine{d1, d2},
			[][2]string{{"old-a", ""}, {"old-b", ""}},
		},
		{
			"more deletions",
			[]patchLine{d1, d2, a1},
			[][2]string{{"old-a", "new-a"}, {"old-b", ""}},
		},
		{
			"more additions",
			[]patchLine{d1, a1, a2},
			[][2]string{{"old-a", "new-a"}, {"", "new-b"}},
		},
		{
			"context separates blocks",
			[]patchLine{d1, context, a1},
			[][2]string{
				{"old-a", ""},
				{"unchanged", "unchanged"},
				{"", "new-a"},
			},
		},
		{
			"addition before deletion",
			[]patchLine{a1, d1, a2},
			[][2]string{{"", "new-a"}, {"old-a", "new-b"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := patchPatch{
				Files: []patchFile{
					{
						DisplayPath: "file.go",
						Hunks: []patchHunk{
							{Header: "@@", Lines: tc.lines},
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
			got := (func(v *diffscreenDiffView) [][2]string {
				var got [][2]string
				for row, inlineLocalEntry1 := range v.rows {
					if inlineLocalEntry1.kind != diffscreenLineRow {
						continue
					}
					texts := (func(v *diffscreenDiffView, row int) [2]string {
						var texts [2]string
						for _, pane := range []diffscreenDiffPane{diffscreenLeft, diffscreenRight} {
							if cursor, ok := func() (diffscreenDiffCursor, bool) {
								cursor := diffscreenDiffCursor{row: row, pane: pane}
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
								line, _ := func() (patchLine, bool) {
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
								}()
								texts[pane] = line.Text
							}
						}
						return texts
					})(v, row)
					got = append(got, texts)
				}
				return got
			})(v)
			(func(t *testing.T, got, want bool, format string, args ...any) {
				t.Helper()
				if got != want {
					t.Fatalf(format, args...)
				}
			})(t, slices.Equal(got, tc.want), true,
				"paired source lines=%v, want %v", got, tc.want,
			)
		})
	}
}

func TestDiffscreenCursorRestorationPrioritizesIdentityBeforeDistance(t *testing.T) {
	code := func(
		kind patchLineKind,
		text string,
		number patchLineNumber,
	) patchLine {
		return patchLine{Kind: kind, Text: text, NewNumber: number}
	}
	context := code(patchContext, "unchanged", 1)
	target := code(patchAddition, "target", 10)
	makePatch := (func(lines []patchLine) patchPatch {
		return patchPatch{Files: []patchFile{{OldPath: "file.go", NewPath: "file.go", Hunks: []patchHunk{{Header: "@@", Lines: lines}}}}}
	})
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
	})(makePatch([]patchLine{context, target}), true)
	cursor, _ := func() (diffscreenDiffCursor, bool) {
		if cursor, ok := func() (diffscreenDiffCursor, bool) {
			step := int(diffscreenPrivateBackward)
			for y := len(old.rows) + step; y >= 0 && y < len(old.rows); y += step {
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
		step := int(diffscreenPrivateBackward)
		for y := len(old.rows) + step; y >= 0 && y < len(old.rows); y += step {
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
	identity := func() diffscreenCursorIdentity {
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
	for _, tc := range []struct {
		name  string
		lines []patchLine
		want  patchLine
	}{
		{
			"numbers before text",
			[]patchLine{
				code(patchAddition, "target", 20),
				code(patchAddition, "changed text", 10),
			},
			code(patchAddition, "changed text", 10),
		},
		{
			"text before distance",
			[]patchLine{
				code(patchAddition, "target", 20),
				code(patchAddition, "different", 21),
			},
			code(patchAddition, "target", 20),
		},
		{
			"source distance before kind",
			[]patchLine{code(patchAddition, "different", 20), context},
			context,
		},
		{
			"nearest matching text by source position",
			[]patchLine{
				code(patchAddition, "target", 20),
				code(patchAddition, "target", 21),
			},
			code(patchAddition, "target", 20),
		},
		{
			"equidistant matching text",
			[]patchLine{
				code(patchAddition, "target", 8),
				context,
				code(patchAddition, "target", 12),
			},
			code(patchAddition, "target", 8),
		},
		{
			"nearest remaining line",
			[]patchLine{context, code(patchContext, "nearby", 2)},
			code(patchContext, "nearby", 2),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			(func(
				t *testing.T,
				p patchPatch,
				identity diffscreenCursorIdentity, want patchLine,
			) {
				t.Helper()
				for _, header := range []string{"@@", "@@ -10,2 +10,3 @@"} {
					t.Run(header, func(t *testing.T) {
						p.Files[0].Hunks[0].Header = header
						next := (func(p patchPatch, dark bool) *diffscreenDiffView {
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
						restored, ok := func() (diffscreenDiffCursor, bool) {
							if !identity.valid {
								return diffscreenDiffCursor{}, false
							}
							best := diffscreenCursorMatch{}
							for y := range next.rows {
								candidate := func() diffscreenCursorMatch {
									current := next.rows[y]
									if !(current.hunk >= 0 && func() bool {
										var candidate patchFile = next.patch.Files[current.file]
										return candidate.OldPath != "" && candidate.OldPath == identity.file.OldPath || candidate.NewPath != "" && candidate.NewPath == identity.file.NewPath
									}()) {
										return diffscreenCursorMatch{}
									}
									best := diffscreenCursorMatch{}
									for _, pane := range [2]diffscreenDiffPane{identity.cursor.pane, func() diffscreenDiffPane {
										if identity.cursor.pane == diffscreenLeft {
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
											if line.Kind != identity.line.Kind {
												return 1
											}
											if line.OldNumber == identity.line.OldNumber && line.NewNumber == identity.line.NewNumber {
												return 3
											}
											if line.Text == identity.line.Text {
												return 2
											}
											return 1
										}(), distance: func() int {
											var rowDistance int = y - identity.cursor.row
											if line.NewNumber > 0 && identity.line.NewNumber > 0 {
												if int(line.NewNumber-identity.line.NewNumber) < 0 {
													return -int(line.NewNumber - identity.line.NewNumber)
												}
												return int(line.NewNumber - identity.line.NewNumber)
											}
											if line.OldNumber > 0 && identity.line.OldNumber > 0 {
												if int(line.OldNumber-identity.line.OldNumber) < 0 {
													return -int(line.OldNumber - identity.line.OldNumber)
												}
												return int(line.OldNumber - identity.line.OldNumber)
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
						line, _ := func() (patchLine, bool) {
							if !(restored.row >= 0 && restored.row < len(next.rows) && func() int {
								var current diffscreenEntry = next.rows[restored.row]
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || restored.pane == diffscreenRight {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patchLine{}, false
							}
							current := next.rows[restored.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != diffscreenLineRow {
									return -1
								}
								if !next.split || restored.pane == diffscreenRight {
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
						})(t, ok, true, "restored=%#v, want %#v", line,
							want,
						)
						(func(t *testing.T, got, want patchLine, format string, args ...any) {
							t.Helper()
							if got != want {
								t.Fatalf(format, args...)
							}
						})(t, line, want, "restored=%#v, want %#v", line,
							want,
						)
					})
				}
			})(t, makePatch(tc.lines), identity, tc.want)
		})
	}
}
