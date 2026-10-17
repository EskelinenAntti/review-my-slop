package diffscreen

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitRowsPairOnlyAdjacentChangeBlocks(t *testing.T) {
	code := func(kind patch.LineKind, text string) patch.Line {
		return patch.Line{Kind: kind, Text: text}
	}
	d1, d2 := code(patch.Deletion, "old-a"), code(patch.Deletion, "old-b")
	a1, a2 := code(patch.Addition, "new-a"), code(patch.Addition, "new-b")
	context := code(patch.Context, "unchanged")
	for _, tc := range []struct {
		name  string
		lines []patch.Line
		want  [][2]string
	}{
		{
			"additions",
			[]patch.Line{a1, a2},
			[][2]string{{"", "new-a"}, {"", "new-b"}},
		},
		{
			"deletions",
			[]patch.Line{d1, d2},
			[][2]string{{"old-a", ""}, {"old-b", ""}},
		},
		{
			"more deletions",
			[]patch.Line{d1, d2, a1},
			[][2]string{{"old-a", "new-a"}, {"old-b", ""}},
		},
		{
			"more additions",
			[]patch.Line{d1, a1, a2},
			[][2]string{{"old-a", "new-a"}, {"", "new-b"}},
		},
		{
			"context separates blocks",
			[]patch.Line{d1, context, a1},
			[][2]string{
				{"old-a", ""},
				{"unchanged", "unchanged"},
				{"", "new-a"},
			},
		},
		{
			"addition before deletion",
			[]patch.Line{a1, d1, a2},
			[][2]string{{"", "new-a"}, {"old-a", "new-b"}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := patch.Patch{
				Files: []patch.File{
					{
						DisplayPath: "file.go",
						Hunks: []patch.Hunk{
							{Header: "@@", Lines: tc.lines},
						},
					},
				},
			}
			v := (func(p patch.Patch, dark bool) *diffView {
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
			})(p, true)
			got := (func(v *diffView) [][2]string {
				var got [][2]string
				for row, inlineLocalEntry1 := range v.rows {
					if inlineLocalEntry1.kind != lineRow {
						continue
					}
					texts := (func(v *diffView, row int) [2]string {
						var texts [2]string
						for _, pane := range []diffPane{left, right} {
							if cursor, ok := func() (diffCursor, bool) {
								cursor := diffCursor{row: row, pane: pane}
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
								line, _ := func() (patch.Line, bool) {
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
										return patch.Line{}, false
									}
									current := v.rows[cursor.row]
									return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
										if current.kind != lineRow {
											return -1
										}
										if !v.split || cursor.pane == right {
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
			testutil.RequireEqual(t, slices.Equal(got, tc.want), true,
				"paired source lines=%v, want %v", got, tc.want,
			)
		})
	}
}

func TestCursorRestorationPrioritizesIdentityBeforeDistance(t *testing.T) {
	code := func(
		kind patch.LineKind,
		text string,
		number patch.LineNumber,
	) patch.Line {
		return patch.Line{Kind: kind, Text: text, NewNumber: number}
	}
	context := code(patch.Context, "unchanged", 1)
	target := code(patch.Addition, "target", 10)
	makePatch := (func(lines []patch.Line) patch.Patch {
		return patch.Patch{Files: []patch.File{{OldPath: "file.go", NewPath: "file.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
	})
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
	})(makePatch([]patch.Line{context, target}), true)
	cursor, _ := func() (diffCursor, bool) {
		if cursor, ok := func() (diffCursor, bool) {
			step := int(backward)
			for y := len(old.rows) + step; y >= 0 && y < len(old.rows); y += step {
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
		step := int(backward)
		for y := len(old.rows) + step; y >= 0 && y < len(old.rows); y += step {
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
	identity := func() cursorIdentity {
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
	for _, tc := range []struct {
		name  string
		lines []patch.Line
		want  patch.Line
	}{
		{
			"numbers before text",
			[]patch.Line{
				code(patch.Addition, "target", 20),
				code(patch.Addition, "changed text", 10),
			},
			code(patch.Addition, "changed text", 10),
		},
		{
			"text before distance",
			[]patch.Line{
				code(patch.Addition, "target", 20),
				code(patch.Addition, "different", 21),
			},
			code(patch.Addition, "target", 20),
		},
		{
			"source distance before kind",
			[]patch.Line{code(patch.Addition, "different", 20), context},
			context,
		},
		{
			"nearest matching text by source position",
			[]patch.Line{
				code(patch.Addition, "target", 20),
				code(patch.Addition, "target", 21),
			},
			code(patch.Addition, "target", 20),
		},
		{
			"equidistant matching text",
			[]patch.Line{
				code(patch.Addition, "target", 8),
				context,
				code(patch.Addition, "target", 12),
			},
			code(patch.Addition, "target", 8),
		},
		{
			"nearest remaining line",
			[]patch.Line{context, code(patch.Context, "nearby", 2)},
			code(patch.Context, "nearby", 2),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			(func(
				t *testing.T,
				p patch.Patch,
				identity cursorIdentity,
				want patch.Line,
			) {
				t.Helper()
				for _, header := range []string{"@@", "@@ -10,2 +10,3 @@"} {
					t.Run(header, func(t *testing.T) {
						p.Files[0].Hunks[0].Header = header
						next := (func(p patch.Patch, dark bool) *diffView {
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
						restored, ok := func() (diffCursor, bool) {
							if !identity.valid {
								return diffCursor{}, false
							}
							best := cursorMatch{}
							for y := range next.rows {
								candidate := func() cursorMatch {
									current := next.rows[y]
									if !(current.hunk >= 0 && func() bool {
										var candidate patch.File = next.patch.Files[current.file]
										return candidate.OldPath != "" && candidate.OldPath == identity.file.OldPath || candidate.NewPath != "" && candidate.NewPath == identity.file.NewPath
									}()) {
										return cursorMatch{}
									}
									best := cursorMatch{}
									for _, pane := range [2]diffPane{identity.cursor.pane, func() diffPane {
										if identity.cursor.pane == left {
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
						line, _ := func() (patch.Line, bool) {
							if !(restored.row >= 0 && restored.row < len(next.rows) && func() int {
								var current entry = next.rows[restored.row]
								if current.kind != lineRow {
									return -1
								}
								if !next.split || restored.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}() >= 0) {
								return patch.Line{}, false
							}
							current := next.rows[restored.row]
							return next.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
								if current.kind != lineRow {
									return -1
								}
								if !next.split || restored.pane == right {
									return current.rightLine
								}
								return current.leftLine
							}()], true
						}()
						testutil.RequireEqual(t, ok, true, "restored=%#v, want %#v", line,
							want,
						)
						testutil.RequireEqual(t, line, want, "restored=%#v, want %#v", line,
							want,
						)
					})
				}
			})(t, makePatch(tc.lines), identity, tc.want)
		})
	}
}
