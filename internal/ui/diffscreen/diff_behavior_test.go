package diffscreen

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitPairsUnequalChangeBlocksAndKeepsHunksSeparate(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{
			{
				Header: "one",
				Lines: []patch.Line{
					{Kind: patch.Deletion, Text: "d1", OldNumber: 1},
					{Kind: patch.Deletion, Text: "d2", OldNumber: 2},
					{Kind: patch.Addition, Text: "a1", NewNumber: 1},
					{Kind: patch.Addition, Text: "a2", NewNumber: 2},
					{Kind: patch.Addition, Text: "a3", NewNumber: 3},
					{
						Kind:      patch.Context,
						Text:      "c",
						OldNumber: 3,
						NewNumber: 4,
					},
				},
			},
			{
				Header: "two",
				Lines: []patch.Line{
					{Kind: patch.Addition, Text: "separate", NewNumber: 5},
				},
			},
		}}},
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
	var code []entry
	for _, current := range v.rows {
		if current.kind == lineRow {
			code = append(code, current)
		}
	}
	testutil.RequireEqual(t, len(code), 5, "visual code entries = %d",
		len(code),
	)
	testutil.RequireEqual(t, code[0].leftLine, 0, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[0].rightLine, 2, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[1].leftLine, 1, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[1].rightLine, 3, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[2].leftLine, -1, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[2].rightLine, 4, "pairing = %#v", code[:3])
	testutil.RequireEqual(t, code[4].hunk != code[3].hunk, true,
		"hunks were paired: %#v", code[4],
	)
	testutil.RequireEqual(t, code[4].leftLine, -1, "hunks were paired: %#v",
		code[4],
	)
}

func TestSplitSelectionOnlyIncludesActivePane(t *testing.T) {
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
	})(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, true)
	first := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	removed, _ := func() (diffCursor, bool) {
		var query string = "removed one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(forward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if v.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: first.pane}
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
										return candidate, true
									}
									if v.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if first.pane == left {
												return right
											}
											return left
										}()}
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
									}
									return diffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffCursor{}, false
					}
					return diffCursor{}, false
				}
				panes := []diffPane{first.pane}
				if v.split {
					panes = append(panes, func() diffPane {
						if first.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
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
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
							var current entry = v.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := v.rows[candidate.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !v.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffCursor{}, false
	}()
	added, _ := func() (diffCursor, bool) {
		var query string = "added one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(forward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if v.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: first.pane}
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
										return candidate, true
									}
									if v.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if first.pane == left {
												return right
											}
											return left
										}()}
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
									}
									return diffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffCursor{}, false
					}
					return diffCursor{}, false
				}
				panes := []diffPane{first.pane}
				if v.split {
					panes = append(panes, func() diffPane {
						if first.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
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
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(v.rows) && func() int {
							var current entry = v.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := v.rows[candidate.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !v.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffCursor{}, false
	}()
	inlineLocalLeft2 := diffSelection{First: removed, Last: removed}
	inlineLocalLeft2, ok := func() (diffSelection, bool) {
		var selection diffSelection = inlineLocalLeft2
		if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
			var (
				current entry    = v.rows[selection.First.row]
				pane    diffPane = selection.First.pane
			)
			if current.kind != lineRow {
				return -1
			}
			if !v.split || pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) || !(removed.row >= 0 && removed.row < len(v.rows) && func() int {
			var current entry = v.rows[removed.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || removed.pane == right {
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
	testutil.RequireEqual(t, ok, true, "left lines = %#v", func() []patch.Line {
		var selection diffSelection = inlineLocalLeft2
		if _, ok := func() (diffSelection, bool) {
			var selection diffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current entry    = v.rows[selection.First.row]
					pane    diffPane = selection.First.pane
				)
				if current.kind != lineRow {
					return -1
				}
				if !v.split || pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current entry = v.rows[selection.Last.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || selection.Last.pane == right {
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
			first, _ := func() (patch.Line, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.First.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patch.Line, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patch.Line{first}
			}
			return []patch.Line{first, last}
		}
		lines := make([]patch.Line, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patch.Line, bool) {
				var cursor diffCursor = diffCursor{row: y, pane: pane}
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
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}())
	testutil.RequireEqual(t, len(func() []patch.Line {
		var selection diffSelection = inlineLocalLeft2
		if _, ok := func() (diffSelection, bool) {
			var selection diffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current entry    = v.rows[selection.First.row]
					pane    diffPane = selection.First.pane
				)
				if current.kind != lineRow {
					return -1
				}
				if !v.split || pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current entry = v.rows[selection.Last.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || selection.Last.pane == right {
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
			first, _ := func() (patch.Line, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.First.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patch.Line, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patch.Line{first}
			}
			return []patch.Line{first, last}
		}
		lines := make([]patch.Line, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patch.Line, bool) {
				var cursor diffCursor = diffCursor{row: y, pane: pane}
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
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}()), 1, "left lines = %#v",
		func() []patch.Line {
			var selection diffSelection = inlineLocalLeft2
			if _, ok := func() (diffSelection, bool) {
				var selection diffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current entry    = v.rows[selection.First.row]
						pane    diffPane = selection.First.pane
					)
					if current.kind != lineRow {
						return -1
					}
					if !v.split || pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
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
				first, _ := func() (patch.Line, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.First.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patch.Line, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.Last.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patch.Line{first}
				}
				return []patch.Line{first, last}
			}
			lines := make([]patch.Line, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patch.Line, bool) {
					var cursor diffCursor = diffCursor{row: y, pane: pane}
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
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}(),
	)
	testutil.RequireEqual(t, func() []patch.Line {
		var selection diffSelection = inlineLocalLeft2
		if _, ok := func() (diffSelection, bool) {
			var selection diffSelection = selection
			if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
				var (
					current entry    = v.rows[selection.First.row]
					pane    diffPane = selection.First.pane
				)
				if current.kind != lineRow {
					return -1
				}
				if !v.split || pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
				var current entry = v.rows[selection.Last.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || selection.Last.pane == right {
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
			first, _ := func() (patch.Line, bool) {
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.First.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.First.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.First.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			last, _ := func() (patch.Line, bool) {
				if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.Line{}, false
				}
				current := v.rows[selection.Last.row]
				return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}()], true
			}()
			if first == last {
				return []patch.Line{first}
			}
			return []patch.Line{first, last}
		}
		lines := make([]patch.Line, 0, last-first+1)
		for y := first; y <= last; y++ {
			pane := func() diffPane {
				if y == selection.Last.row {
					return selection.Last.pane
				}
				return selection.First.pane
			}()
			if line, ok := func() (patch.Line, bool) {
				var cursor diffCursor = diffCursor{row: y, pane: pane}
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
			}(); ok {
				lines = append(lines, line)
			}
		}
		return lines
	}()[0].Kind, patch.Deletion,
		"left lines = %#v", func() []patch.Line {
			var selection diffSelection = inlineLocalLeft2
			if _, ok := func() (diffSelection, bool) {
				var selection diffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current entry    = v.rows[selection.First.row]
						pane    diffPane = selection.First.pane
					)
					if current.kind != lineRow {
						return -1
					}
					if !v.split || pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
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
				first, _ := func() (patch.Line, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.First.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patch.Line, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.Last.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patch.Line{first}
				}
				return []patch.Line{first, last}
			}
			lines := make([]patch.Line, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patch.Line, bool) {
					var cursor diffCursor = diffCursor{row: y, pane: pane}
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
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}(),
	)

	inlineLocalRight2 := diffSelection{First: added, Last: added}
	testutil.RequireEqual(t, len((func(v *diffView) func(selection diffSelection) []patch.Line {
		return func(selection diffSelection) []patch.Line {
			if _, ok := func() (diffSelection, bool) {
				var selection diffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current entry    = v.rows[selection.First.row]
						pane    diffPane = selection.First.pane
					)
					if current.kind != lineRow {
						return -1
					}
					if !v.split || pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
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
				first, _ := func() (patch.Line, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.First.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patch.Line, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.Last.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patch.Line{first}
				}
				return []patch.Line{first, last}
			}
			lines := make([]patch.Line, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patch.Line, bool) {
					var cursor diffCursor = diffCursor{row: y, pane: pane}
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
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}
	}(v))(inlineLocalRight2)), 1, "right lines = %#v",
		(func(v *diffView) func(selection diffSelection) []patch.Line {
			return func(selection diffSelection) []patch.Line {
				if _, ok := func() (diffSelection, bool) {
					var selection diffSelection = selection
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current entry    = v.rows[selection.First.row]
							pane    diffPane = selection.First.pane
						)
						if current.kind != lineRow {
							return -1
						}
						if !v.split || pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.Last.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
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
					first, _ := func() (patch.Line, bool) {
						if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
							var current entry = v.rows[selection.First.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.First.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := v.rows[selection.First.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.First.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					last, _ := func() (patch.Line, bool) {
						if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
							var current entry = v.rows[selection.Last.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.Last.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := v.rows[selection.Last.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.Last.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if first == last {
						return []patch.Line{first}
					}
					return []patch.Line{first, last}
				}
				lines := make([]patch.Line, 0, last-first+1)
				for y := first; y <= last; y++ {
					pane := func() diffPane {
						if y == selection.Last.row {
							return selection.Last.pane
						}
						return selection.First.pane
					}()
					if line, ok := func() (patch.Line, bool) {
						var cursor diffCursor = diffCursor{row: y, pane: pane}
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
					}(); ok {
						lines = append(lines, line)
					}
				}
				return lines
			}
		}(v))(inlineLocalRight2),
	)
	testutil.RequireEqual(t, (func(v *diffView) func(selection diffSelection) []patch.Line {
		return func(selection diffSelection) []patch.Line {
			if _, ok := func() (diffSelection, bool) {
				var selection diffSelection = selection
				if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
					var (
						current entry    = v.rows[selection.First.row]
						pane    diffPane = selection.First.pane
					)
					if current.kind != lineRow {
						return -1
					}
					if !v.split || pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
					var current entry = v.rows[selection.Last.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || selection.Last.pane == right {
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
				first, _ := func() (patch.Line, bool) {
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.First.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.First.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.First.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				last, _ := func() (patch.Line, bool) {
					if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.Last.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) {
						return patch.Line{}, false
					}
					current := v.rows[selection.Last.row]
					return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
							return current.rightLine
						}
						return current.leftLine
					}()], true
				}()
				if first == last {
					return []patch.Line{first}
				}
				return []patch.Line{first, last}
			}
			lines := make([]patch.Line, 0, last-first+1)
			for y := first; y <= last; y++ {
				pane := func() diffPane {
					if y == selection.Last.row {
						return selection.Last.pane
					}
					return selection.First.pane
				}()
				if line, ok := func() (patch.Line, bool) {
					var cursor diffCursor = diffCursor{row: y, pane: pane}
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
				}(); ok {
					lines = append(lines, line)
				}
			}
			return lines
		}
	}(v))(inlineLocalRight2)[0].Kind, patch.Addition,
		"right lines = %#v", (func(v *diffView) func(selection diffSelection) []patch.Line {
			return func(selection diffSelection) []patch.Line {
				if _, ok := func() (diffSelection, bool) {
					var selection diffSelection = selection
					if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
						var (
							current entry    = v.rows[selection.First.row]
							pane    diffPane = selection.First.pane
						)
						if current.kind != lineRow {
							return -1
						}
						if !v.split || pane == right {
							return current.rightLine
						}
						return current.leftLine
					}() >= 0) || !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
						var current entry = v.rows[selection.Last.row]
						if current.kind != lineRow {
							return -1
						}
						if !v.split || selection.Last.pane == right {
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
					first, _ := func() (patch.Line, bool) {
						if !(selection.First.row >= 0 && selection.First.row < len(v.rows) && func() int {
							var current entry = v.rows[selection.First.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.First.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := v.rows[selection.First.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.First.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					last, _ := func() (patch.Line, bool) {
						if !(selection.Last.row >= 0 && selection.Last.row < len(v.rows) && func() int {
							var current entry = v.rows[selection.Last.row]
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.Last.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := v.rows[selection.Last.row]
						return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !v.split || selection.Last.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					if first == last {
						return []patch.Line{first}
					}
					return []patch.Line{first, last}
				}
				lines := make([]patch.Line, 0, last-first+1)
				for y := first; y <= last; y++ {
					pane := func() diffPane {
						if y == selection.Last.row {
							return selection.Last.pane
						}
						return selection.First.pane
					}()
					if line, ok := func() (patch.Line, bool) {
						var cursor diffCursor = diffCursor{row: y, pane: pane}
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
					}(); ok {
						lines = append(lines, line)
					}
				}
				return lines
			}
		}(v))(inlineLocalRight2),
	)
}

func TestSplitPaneSwitchingFindsRowsAboveAndBelowEmptyTargets(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
					{Kind: patch.Addition, Text: "right", NewNumber: 1},
					{
						Kind:      patch.Context,
						Text:      "both",
						OldNumber: 1,
						NewNumber: 2,
					},
					{Kind: patch.Deletion, Text: "left", OldNumber: 2},
				}}},
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
	first := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	leftCursor, ok := func() (diffCursor, bool) {
		if !v.split || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = first.row
			cursor := diffCursor{row: y, pane: left}
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
			return candidate, true
		}
		if candidate, ok := func() (diffCursor, bool) {
			for y := first.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[first.row].file {
					break
				}
				if candidate, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return candidate, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(forward)
		for y := first.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: left}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	testutil.RequireEqual(t, ok, true, "%s", "did not find later left pane")

	line, _ := func() (patch.Line, bool) {
		if !(leftCursor.row >= 0 && leftCursor.row < len(v.rows) && func() int {
			var current entry = v.rows[leftCursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || leftCursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[leftCursor.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || leftCursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, line.Text, "both", "later left line = %q",
		line.Text,
	)

	last, _ := func() (diffCursor, bool) {
		if cursor, ok := func() (diffCursor, bool) {
			var start int = len(v.rows)
			step := int(backward)
			for y := start + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: right}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return cursor, true
		}
		var start int = len(v.rows)
		step := int(backward)
		for y := start + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: left}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	inlineLocalRight1, ok := func() (diffCursor, bool) {
		if !v.split || !(last.row >= 0 && last.row < len(v.rows) && func() int {
			var current entry = v.rows[last.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || last.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = last.row
			cursor := diffCursor{row: y, pane: right}
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
			return candidate, true
		}
		if candidate, ok := func() (diffCursor, bool) {
			for y := last.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[last.row].file {
					break
				}
				if candidate, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: right}
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
					return candidate, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(forward)
		for y := last.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: right}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	testutil.RequireEqual(t, ok, true, "%s", "did not find earlier right pane")

	line, _ = (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
		return func(cursor diffCursor) (patch.Line, bool) {
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
		}
	}(v))(inlineLocalRight1)
	testutil.RequireEqual(t, line.Text, "both", "earlier right line = %q",
		line.Text,
	)
}

func TestSplitPaneSwitchDoesNothingWhenTargetPaneIsEmpty(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Addition, Text: "one", NewNumber: 1},
							{Kind: patch.Addition, Text: "two", NewNumber: 2},
						},
					},
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
	cursor := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	if _, ok := func() (diffCursor, bool) {
		if !v.split || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = cursor.row
			cursor := diffCursor{row: y, pane: left}
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
			return candidate, true
		}
		if candidate, ok := func() (diffCursor, bool) {
			for y := cursor.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[cursor.row].file {
					break
				}
				if candidate, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return candidate, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(forward)
		for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: left}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}(); ok {
		t.Fatal("switched to empty pane")
	}
}

func TestSplitVerticalMovementSkipsEmptyActivePane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "one",
								OldNumber: 1,
								NewNumber: 1,
							},
							{Kind: patch.Addition, Text: "right", NewNumber: 2},
							{
								Kind:      patch.Context,
								Text:      "two",
								OldNumber: 2,
								NewNumber: 3,
							},
							{Kind: patch.Deletion, Text: "left", OldNumber: 3},
							{
								Kind:      patch.Context,
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
	first := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	inlineLocalLeft1, _ := func() (diffCursor, bool) {
		if !v.split || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = first.row
			cursor := diffCursor{row: y, pane: left}
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
			return candidate, true
		}
		if candidate, ok := func() (diffCursor, bool) {
			for y := first.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[first.row].file {
					break
				}
				if candidate, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return candidate, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(forward)
		for y := first.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: left}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	nextLeft, _ := func() (diffCursor, bool) {
		if !(inlineLocalLeft1.row >= 0 && inlineLocalLeft1.row < len(v.rows) && func() int {
			var current entry = v.rows[inlineLocalLeft1.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || inlineLocalLeft1.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		step := int(forward)
		for y := inlineLocalLeft1.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: inlineLocalLeft1.pane}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	line, _ := (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
		return func(cursor diffCursor) (patch.Line, bool) {
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
		}
	}(v))(nextLeft)
	testutil.RequireEqual(t, line.Text, "two", "left movement = %q", line.Text)

	rightAtContext, _ := func() (diffCursor, bool) {
		if !v.split || !(nextLeft.row >= 0 && nextLeft.row < len(v.rows) && func() int {
			var current entry = v.rows[nextLeft.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || nextLeft.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = nextLeft.row
			cursor := diffCursor{row: y, pane: right}
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
			return candidate, true
		}
		if candidate, ok := func() (diffCursor, bool) {
			for y := nextLeft.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[nextLeft.row].file {
					break
				}
				if candidate, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: right}
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
					return candidate, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return candidate, true
		}
		step := int(forward)
		for y := nextLeft.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: right}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	nextRight, _ := func() (diffCursor, bool) {
		if !(rightAtContext.row >= 0 && rightAtContext.row < len(v.rows) && func() int {
			var current entry = v.rows[rightAtContext.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || rightAtContext.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		step := int(forward)
		for y := rightAtContext.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: rightAtContext.pane}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	line, _ = (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
		return func(cursor diffCursor) (patch.Line, bool) {
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
		}
	}(v))(nextRight)
	testutil.RequireEqual(t, line.Text, "three", "right movement = %q",
		line.Text,
	)
}

func TestSplitVerticalMovementAndHalfPageUseVisualRows(t *testing.T) {
	lines := []patch.Line{
		{
			Kind:      patch.Deletion,
			Text:      "d1",
			OldNumber: 1,
		}, {
			Kind:      patch.Addition,
			Text:      "a1",
			NewNumber: 1,
		}, {Kind: patch.Context, Text: "c1", OldNumber: 2, NewNumber: 2},
		{
			Kind:      patch.Deletion,
			Text:      "d2",
			OldNumber: 3,
		}, {
			Kind:      patch.Addition,
			Text:      "a2",
			NewNumber: 3,
		}, {Kind: patch.Context, Text: "c2", OldNumber: 4, NewNumber: 4},
		{
			Kind:      patch.Deletion,
			Text:      "d3",
			OldNumber: 5,
		}, {
			Kind:      patch.Addition,
			Text:      "a3",
			NewNumber: 5,
		}, {Kind: patch.Context, Text: "c3", OldNumber: 6, NewNumber: 6},
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
	})(
		patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "file",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		},
		true,
	)
	cursor := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 120), max(1, 4)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	viewport = func() diffViewport {
		var viewport diffViewport = viewport
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
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
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
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
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
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
	viewport, moved := func() (diffViewport, diffCursor) {
		var (
			viewport diffViewport = viewport
			cursor   diffCursor   = cursor
		)
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var (
				current entry    = v.rows[cursor.row]
				pane    diffPane = cursor.pane
			)
			if current.kind != lineRow {
				return -1
			}
			if !v.split || pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return viewport, cursor
		}
		distance := int(forward) * max(1, viewport.Height/2)
		viewport.top += distance
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}()
		target := min(len(v.rows)-1, max(0, cursor.row+distance))
		candidate, ok := func() (diffCursor, bool) {
			var (
				pane     diffPane     = cursor.pane
				viewport diffViewport = viewport
			)
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}()
			for distance := 0; distance < height; distance++ {
				for _, y := range []int{target + int(forward)*distance, target - int(forward)*distance} {
					if cursor, ok := func() (diffCursor, bool) {
						if y < viewport.top || y >= viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								height--
							}
							return max(1, height)
						}() {
							return diffCursor{}, false
						}
						cursor := diffCursor{row: y, pane: pane}
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
						return cursor, true
					}
				}
			}
			return diffCursor{}, false
		}()
		if ok {
			cursor = candidate
		}
		return viewport, cursor
	}()
	testutil.RequireEqual(t, viewport.top > originalTop, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)
	testutil.RequireEqual(t, moved.row > cursor.row, true,
		"viewport=%#v cursor=%#v", viewport, moved,
	)

	viewport, moved = func() (diffViewport, diffCursor) {
		var (
			viewport diffViewport = viewport
			cursor   diffCursor   = moved
		)
		if !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var (
				current entry    = v.rows[cursor.row]
				pane    diffPane = cursor.pane
			)
			if current.kind != lineRow {
				return -1
			}
			if !v.split || pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return viewport, cursor
		}
		distance := int(backward) * max(1, viewport.Height/2)
		viewport.top += distance
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}()
		target := min(len(v.rows)-1, max(0, cursor.row+distance))
		candidate, ok := func() (diffCursor, bool) {
			var (
				pane     diffPane     = cursor.pane
				viewport diffViewport = viewport
			)
			height := func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}()
			for distance := 0; distance < height; distance++ {
				for _, y := range []int{target + int(backward)*distance, target - int(backward)*distance} {
					if cursor, ok := func() (diffCursor, bool) {
						if y < viewport.top || y >= viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								height--
							}
							return max(1, height)
						}() {
							return diffCursor{}, false
						}
						cursor := diffCursor{row: y, pane: pane}
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
						return cursor, true
					}
				}
			}
			return diffCursor{}, false
		}()
		if ok {
			cursor = candidate
		}
		return viewport, cursor
	}()
	testutil.RequireEqual(t, viewport.top, originalTop,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
	testutil.RequireEqual(t, moved.row, cursor.row,
		"round trip viewport=%#v cursor=%#v", viewport, moved,
	)
}

func TestFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := func() patch.Patch {
		p := patch.Patch{Files: []patch.File{{DisplayPath: "first.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "first one", OldNumber: 1, NewNumber: 1}, {Kind: patch.Context, Text: "first two", OldNumber: 2, NewNumber: 2}, {Kind: patch.Context, Text: "first three", OldNumber: 3, NewNumber: 3}}}}}, {DisplayPath: "second.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "second one", OldNumber: 1, NewNumber: 1}}}}}}}
		return p
	}()
	v := (func(p patch.Patch, dark bool) *diffView {
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
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 60), max(1, 3)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
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
		ansi.Strip((func(v *diffView) func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			return func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return fileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == lineRow {
						lines = append(
							lines,
							(func(v *diffView) func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								return func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || left == right {
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
										if left == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: left}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffView) func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										return func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || pane == right {
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
											if pane == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: pane}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										right,
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
								case fileRow:
									return fileStyle.Width(max(20, width)).Render(current.text)
								case metadataRow:
									return metadataStyle.Render("  " + current.text)
								case hunkRow:
									return hunkStyle.Render(current.text)
								case lineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+")
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-")
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: cursor.pane}
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
										}(), discardForeground
									}
									if cursor.row == y {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, len(rendered), viewport.Height,
		"sticky render = %#v", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[0], "first.go"), true,
		"sticky render = %#v", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[1], "first two"), true,
		"sticky header covered content rows: %#v", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[2], "first three"), true,
		"sticky header covered content rows: %#v", rendered,
	)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip((func(v *diffView) func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			return func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return fileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == lineRow {
						lines = append(
							lines,
							(func(v *diffView) func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								return func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || left == right {
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
										if left == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: left}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffView) func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										return func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || pane == right {
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
											if pane == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: pane}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										right,
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
								case fileRow:
									return fileStyle.Width(max(20, width)).Render(current.text)
								case metadataRow:
									return metadataStyle.Render("  " + current.text)
								case hunkRow:
									return hunkStyle.Render(current.text)
								case lineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+")
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-")
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: cursor.pane}
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
										}(), discardForeground
									}
									if cursor.row == y {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, strings.Count(
		strings.Join(rendered, "\n"),
		"second.go",
	), 1,
		"file header was duplicated at its natural position: %#v", rendered)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(
		ansi.Strip((func(v *diffView) func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			return func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return fileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == lineRow {
						lines = append(
							lines,
							(func(v *diffView) func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								return func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || left == right {
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
										if left == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: left}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffView) func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										return func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || pane == right {
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
											if pane == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: pane}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										right,
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
								case fileRow:
									return fileStyle.Width(max(20, width)).Render(current.text)
								case metadataRow:
									return metadataStyle.Render("  " + current.text)
								case hunkRow:
									return hunkStyle.Render(current.text)
								case lineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+")
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-")
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: cursor.pane}
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
										}(), discardForeground
									}
									if cursor.row == y {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		}(v))(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, strings.Contains(rendered[0], "second.go"), true,
		"sticky header did not change with the file: %#v", rendered,
	)
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := (func(p patch.Patch, dark bool) *diffView {
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
	})((func() patch.Patch {
		lines := make([]patch.Line, 20)
		for index := range lines {
			lines[index] = patch.Line{
				Kind:      patch.Context,
				Text:      strings.Repeat("long", 20),
				OldNumber: patch.LineNumber(index + 1),
				NewNumber: patch.LineNumber(index + 1),
			}
		}
		return patch.Patch{
			Files: []patch.File{
				{
					DisplayPath: "long.go",
					Hunks:       []patch.Hunk{{Header: "@@", Lines: lines}},
				},
			},
		}
	})(), true)
	cursor, _ := func() (diffCursor, bool) {
		if cursor, ok := func() (diffCursor, bool) {
			var start int = len(v.rows)
			step := int(backward)
			for y := start + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: right}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}(); ok {
			return cursor, true
		}
		var start int = len(v.rows)
		step := int(backward)
		for y := start + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: left}
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
				return cursor, true
			}
		}
		return diffCursor{}, false
	}()
	viewport := func() diffViewport {
		var viewport diffViewport = func() diffViewport {
			var viewport diffViewport = diffViewport{}
			viewport.Width, viewport.Height = max(1, 50), max(1, 4)
			{
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
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
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
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
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
	rendered := ansi.Strip((func(v *diffView) func(
		viewport diffViewport,
		cursor diffCursor,
		selection *diffSelection,
	) string {
		return func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return fileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == lineRow {
					lines = append(
						lines,
						(func(v *diffView) func(
							current entry,
							y int,
							viewport diffViewport,
							cursor diffCursor,
							selection *diffSelection,
						) string {
							return func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != lineRow {
											return -1
										}
										if !v.split || left == right {
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
									if left == left {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+") + " "
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-") + " "
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: left}
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
										}(), discardForeground
									}
									if cursor == candidate {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffView) func(
									current entry,
									y int,
									pane diffPane,
									width, offset int,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									return func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || pane == right {
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
										if pane == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: pane}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									right,
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
							case fileRow:
								return fileStyle.Width(max(20, width)).Render(current.text)
							case metadataRow:
								return metadataStyle.Render("  " + current.text)
							case hunkRow:
								return hunkStyle.Render(current.text)
							case lineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patch.Addition {
									prefix = addedStyle.Render("+")
								}
								if line.Kind == patch.Deletion {
									prefix = removedStyle.Render("-")
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
									case patch.Addition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patch.Deletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return contextStyle
									}
								}()
								colors := preserveForeground
								candidate := diffCursor{row: y, pane: cursor.pane}
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
									}(), discardForeground
								}
								if cursor.row == y {
									style, colors = cursorStyle, discardForeground
								}
								{
									var (
										style  lipgloss.Style   = style
										value  string           = value
										colors foregroundPolicy = colors
									)
									value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == discardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	line, _ := (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
		return func(cursor diffCursor) (patch.Line, bool) {
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
		}
	}(v))(cursor)
	testutil.RequireEqual(t,
		cursor.row < viewport.top+func() int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				height--
			}
			return max(1, height)
		}(), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
	testutil.RequireEqual(t,
		strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))), true,
		"last cursor row is hidden by sticky header: viewport=%#v render=%q",
		viewport, rendered,
	)
}

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
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
	cursor := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	rendered := ansi.Strip((func(
		v *diffView,
		cursor diffCursor,
		width int,
		selection *diffSelection,
	) string {
		return (func(
			v *diffView,
			target, active diffCursor,
			width int,
			selection *diffSelection,
		) string {
			viewport := func() diffViewport {
				var viewport diffViewport = diffViewport{}
				viewport.Width, viewport.Height = max(1, width), max(1, 1)
				{
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
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
			return (func(v *diffView) func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				return func(
					viewport diffViewport,
					cursor diffCursor,
					selection *diffSelection,
				) string {
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
					lines := make([]string, 0, viewport.Height)
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						current := v.rows[viewport.top]
						lines = append(
							lines,
							func() string {
								var path string = v.patch.Files[current.file].DisplayPath
								return fileStyle.Width(max(20, viewport.Width)).Render(path)
							}(),
						)
					}
					end := min(len(v.rows), viewport.top+func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}())
					for y := viewport.top; y < end; y++ {
						current := v.rows[y]
						if v.split && current.kind == lineRow {
							lines = append(
								lines,
								(func(v *diffView) func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									return func(
										current entry,
										y int,
										viewport diffViewport,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
											inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
											return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
										}()
										inlineLocalLeft1 := func() string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || left == right {
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
											if left == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: left}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										inlineLocalRight1 := (func(v *diffView) func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											return func(
												current entry,
												y int,
												pane diffPane,
												width, offset int,
												cursor diffCursor,
												selection *diffSelection,
											) string {
												index := func() int {
													if current.kind != lineRow {
														return -1
													}
													if !v.split || pane == right {
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
												if pane == left {
													text, numberValue = current.left, line.OldNumber
												}
												prefix := "  "
												if line.Kind == patch.Addition {
													prefix = addedStyle.Render("+") + " "
												}
												if line.Kind == patch.Deletion {
													prefix = removedStyle.Render("-") + " "
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
													case patch.Addition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patch.Deletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return contextStyle
													}
												}()
												colors := preserveForeground
												candidate := diffCursor{row: y, pane: pane}
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
													}(), discardForeground
												}
												if cursor == candidate {
													style, colors = cursorStyle, discardForeground
												}
												{
													var (
														style  lipgloss.Style   = style
														value  string           = value
														colors foregroundPolicy = colors
													)
													value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == discardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
											right,
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
									case fileRow:
										return fileStyle.Width(max(20, width)).Render(current.text)
									case metadataRow:
										return metadataStyle.Render("  " + current.text)
									case hunkRow:
										return hunkStyle.Render(current.text)
									case lineRow:
										hunk := v.patch.Files[current.file].Hunks[current.hunk]
										line := hunk.Lines[current.rightLine]
										prefix := " "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+")
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-")
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: cursor.pane}
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
											}(), discardForeground
										}
										if cursor.row == y {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	testutil.RequireEqual(t, !strings.ContainsRune(rendered, '\t'), true,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	testutil.RequireEqual(t, strings.Index(rendered, "│"), 59,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
	testutil.RequireEqual(t, lipgloss.Width(rendered), 120,
		"rendered = %q width=%d", rendered, lipgloss.Width(rendered),
	)
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{DisplayPath: "long", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "abcdefghijklmnopqrstuvwxyz" + "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}}}}}}}
	v := (func(p patch.Patch, dark bool) *diffView {
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
	cursor := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 37), max(1, 1)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	viewport = func() diffViewport {
		var viewport diffViewport = viewport
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
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
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
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
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
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
	before := ansi.Strip((func(v *diffView) func(
		viewport diffViewport,
		cursor diffCursor,
		selection *diffSelection,
	) string {
		return func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return fileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == lineRow {
					lines = append(
						lines,
						(func(v *diffView) func(
							current entry,
							y int,
							viewport diffViewport,
							cursor diffCursor,
							selection *diffSelection,
						) string {
							return func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != lineRow {
											return -1
										}
										if !v.split || left == right {
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
									if left == left {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+") + " "
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-") + " "
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: left}
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
										}(), discardForeground
									}
									if cursor == candidate {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffView) func(
									current entry,
									y int,
									pane diffPane,
									width, offset int,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									return func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || pane == right {
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
										if pane == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: pane}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									right,
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
							case fileRow:
								return fileStyle.Width(max(20, width)).Render(current.text)
							case metadataRow:
								return metadataStyle.Render("  " + current.text)
							case hunkRow:
								return hunkStyle.Render(current.text)
							case lineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patch.Addition {
									prefix = addedStyle.Render("+")
								}
								if line.Kind == patch.Deletion {
									prefix = removedStyle.Render("-")
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
									case patch.Addition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patch.Deletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return contextStyle
									}
								}()
								colors := preserveForeground
								candidate := diffCursor{row: y, pane: cursor.pane}
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
									}(), discardForeground
								}
								if cursor.row == y {
									style, colors = cursorStyle, discardForeground
								}
								{
									var (
										style  lipgloss.Style   = style
										value  string           = value
										colors foregroundPolicy = colors
									)
									value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == discardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	viewport = func() diffViewport {
		var (
			viewport diffViewport = viewport
			columns  int          = 4
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if v.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range v.rows {
				if current.kind != lineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	after := ansi.Strip((func(v *diffView) func(
		viewport diffViewport,
		cursor diffCursor,
		selection *diffSelection,
	) string {
		return func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return fileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == lineRow {
					lines = append(
						lines,
						(func(v *diffView) func(
							current entry,
							y int,
							viewport diffViewport,
							cursor diffCursor,
							selection *diffSelection,
						) string {
							return func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != lineRow {
											return -1
										}
										if !v.split || left == right {
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
									if left == left {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+") + " "
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-") + " "
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: left}
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
										}(), discardForeground
									}
									if cursor == candidate {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffView) func(
									current entry,
									y int,
									pane diffPane,
									width, offset int,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									return func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || pane == right {
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
										if pane == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: pane}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									right,
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
							case fileRow:
								return fileStyle.Width(max(20, width)).Render(current.text)
							case metadataRow:
								return metadataStyle.Render("  " + current.text)
							case hunkRow:
								return hunkStyle.Render(current.text)
							case lineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patch.Addition {
									prefix = addedStyle.Render("+")
								}
								if line.Kind == patch.Deletion {
									prefix = removedStyle.Render("-")
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
									case patch.Addition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patch.Deletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return contextStyle
									}
								}()
								colors := preserveForeground
								candidate := diffCursor{row: y, pane: cursor.pane}
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
									}(), discardForeground
								}
								if cursor.row == y {
									style, colors = cursorStyle, discardForeground
								}
								{
									var (
										style  lipgloss.Style   = style
										value  string           = value
										colors foregroundPolicy = colors
									)
									value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == discardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	testutil.RequireEqual(t, before[:14], after[:14],
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	testutil.RequireEqual(t, strings.Contains(after[14:], "efghij"), true,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
	testutil.RequireEqual(t, viewport.LeftColumn, 4,
		"before=%q after=%q viewport=%#v", before, after, viewport,
	)
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
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
	})(patch.Patch{Files: []patch.File{{DisplayPath: "long", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "abcdefghijklmnopqrstuvwxyz" + "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}}}}}}}, true)
	cursor := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 120), max(1, 1)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	viewport = func() diffViewport {
		var viewport diffViewport = viewport
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
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
		viewport = func() diffViewport {
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
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
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
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
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
	viewport = func() diffViewport {
		var (
			viewport diffViewport = viewport
			columns  int          = 8
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if v.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range v.rows {
				if current.kind != lineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	rendered := ansi.Strip((func(v *diffView) func(
		viewport diffViewport,
		cursor diffCursor,
		selection *diffSelection,
	) string {
		return func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			viewport = func() diffViewport {
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
							continue
						}
						longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
					}
					return max(0, longest-contentWidth)
				}()))
				return viewport
			}()
			lines := make([]string, 0, viewport.Height)
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				current := v.rows[viewport.top]
				lines = append(
					lines,
					func() string {
						var path string = v.patch.Files[current.file].DisplayPath
						return fileStyle.Width(max(20, viewport.Width)).Render(path)
					}(),
				)
			}
			end := min(len(v.rows), viewport.top+func() int {
				height := viewport.Height
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					height--
				}
				return max(1, height)
			}())
			for y := viewport.top; y < end; y++ {
				current := v.rows[y]
				if v.split && current.kind == lineRow {
					lines = append(
						lines,
						(func(v *diffView) func(
							current entry,
							y int,
							viewport diffViewport,
							cursor diffCursor,
							selection *diffSelection,
						) string {
							return func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
									inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
									return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
								}()
								inlineLocalLeft1 := func() string {
									index := func() int {
										if current.kind != lineRow {
											return -1
										}
										if !v.split || left == right {
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
									if left == left {
										text, numberValue = current.left, line.OldNumber
									}
									prefix := "  "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+") + " "
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-") + " "
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: left}
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
										}(), discardForeground
									}
									if cursor == candidate {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
								inlineLocalRight1 := (func(v *diffView) func(
									current entry,
									y int,
									pane diffPane,
									width, offset int,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									return func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || pane == right {
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
										if pane == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: pane}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									right,
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
							case fileRow:
								return fileStyle.Width(max(20, width)).Render(current.text)
							case metadataRow:
								return metadataStyle.Render("  " + current.text)
							case hunkRow:
								return hunkStyle.Render(current.text)
							case lineRow:
								hunk := v.patch.Files[current.file].Hunks[current.hunk]
								line := hunk.Lines[current.rightLine]
								prefix := " "
								if line.Kind == patch.Addition {
									prefix = addedStyle.Render("+")
								}
								if line.Kind == patch.Deletion {
									prefix = removedStyle.Render("-")
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
									case patch.Addition:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
									case patch.Deletion:
										return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
									default:
										return contextStyle
									}
								}()
								colors := preserveForeground
								candidate := diffCursor{row: y, pane: cursor.pane}
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
									}(), discardForeground
								}
								if cursor.row == y {
									style, colors = cursorStyle, discardForeground
								}
								{
									var (
										style  lipgloss.Style   = style
										value  string           = value
										colors foregroundPolicy = colors
									)
									value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
												if code == 48 || colors == discardForeground && code == 38 {
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
												if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	testutil.RequireEqual(t, strings.Index(rendered, "│"), 59,
		"gutters moved: %q", rendered,
	)
	testutil.RequireEqual(t, rendered[:6], "    1 ", "gutters moved: %q",
		rendered,
	)
	testutil.RequireEqual(t, rendered[63:69], "    1 ", "gutters moved: %q",
		rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[6:59], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[69:], "ghijkl"), true,
		"offset differs: %q", rendered,
	)
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := (func(p patch.Patch, dark bool) *diffView {
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
	})(patch.Patch{Files: []patch.File{{DisplayPath: "long", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "abcdefghijklmnopqrstuvwxyz" + "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}}}}}}}, true)
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 37), max(1, 1)
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	viewport = func() diffViewport {
		var (
			viewport diffViewport = viewport
			columns  int          = int(^uint(0) >> 1)
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if v.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range v.rows {
				if current.kind != lineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true, "%s",
		"end did not scroll",
	)

	viewport = func() diffViewport {
		var (
			viewport diffViewport = viewport
			columns  int          = -int(^uint(0) >> 1)
		)
		columns = max(-viewport.LeftColumn, min(columns, func() int {
			contentWidth := max(1, viewport.Width-14)
			extra := 0
			if v.split {
				contentWidth, extra = max(1, (viewport.Width-3)/2-6), 2
			}
			longest := 0
			for _, current := range v.rows {
				if current.kind != lineRow {
					continue
				}
				longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
			}
			return max(0, longest-contentWidth)
		}()-viewport.LeftColumn))
		viewport.LeftColumn += columns
		{
			var viewport diffViewport = viewport
			viewport.top = max(0, min(viewport.top, func() int {
				end := max(0, len(v.rows)-viewport.Height)
				if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
					if current.kind != lineRow {
						continue
					}
					longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
				}
				return max(0, longest-contentWidth)
			}()))
			return viewport
		}
	}()
	testutil.RequireEqual(t, viewport.LeftColumn, 0, "start = %d",
		viewport.LeftColumn,
	)
}

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	v := (func(p patch.Patch, dark bool) *diffView {
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
	})(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, true)
	first := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	added, _ := func() (diffCursor, bool) {
		var query string = "added one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(forward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if v.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: first.pane}
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
										return candidate, true
									}
									if v.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if first.pane == left {
												return right
											}
											return left
										}()}
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
									}
									return diffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffCursor{}, false
					}
					return diffCursor{}, false
				}
				panes := []diffPane{first.pane}
				if v.split {
					panes = append(panes, func() diffPane {
						if first.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
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
					}()
					if !ok {
						continue
					}
					line, _ := (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
						return func(cursor diffCursor) (patch.Line, bool) {
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
						}
					}(v))(candidate)
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffCursor{}, false
	}()
	removed, _ := func() (diffCursor, bool) {
		var query string = "removed one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(forward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if v.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: first.pane}
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
										return candidate, true
									}
									if v.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if first.pane == left {
												return right
											}
											return left
										}()}
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
									}
									return diffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffCursor{}, false
					}
					return diffCursor{}, false
				}
				panes := []diffPane{first.pane}
				if v.split {
					panes = append(panes, func() diffPane {
						if first.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
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
					}()
					if !ok {
						continue
					}
					line, _ := (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
						return func(cursor diffCursor) (patch.Line, bool) {
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
						}
					}(v))(candidate)
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffCursor{}, false
	}()
	addedRender := (func(
		v *diffView,
		target, active diffCursor,
		width int,
		selection *diffSelection,
	) string {
		viewport := func() diffViewport {
			var viewport diffViewport = diffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, 1)
			{
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
		return (func(v *diffView) func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			return func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return fileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == lineRow {
						lines = append(
							lines,
							(func(v *diffView) func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								return func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || left == right {
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
										if left == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: left}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffView) func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										return func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || pane == right {
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
											if pane == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: pane}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										right,
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
								case fileRow:
									return fileStyle.Width(max(20, width)).Render(current.text)
								case metadataRow:
									return metadataStyle.Render("  " + current.text)
								case hunkRow:
									return hunkStyle.Render(current.text)
								case lineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+")
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-")
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: cursor.pane}
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
										}(), discardForeground
									}
									if cursor.row == y {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		v *diffView,
		target, active diffCursor,
		width int,
		selection *diffSelection,
	) string {
		viewport := func() diffViewport {
			var viewport diffViewport = diffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, 1)
			{
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
		return (func(v *diffView) func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			return func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return fileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == lineRow {
						lines = append(
							lines,
							(func(v *diffView) func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								return func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || left == right {
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
										if left == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: left}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffView) func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										return func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || pane == right {
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
											if pane == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: pane}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										right,
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
								case fileRow:
									return fileStyle.Width(max(20, width)).Render(current.text)
								case metadataRow:
									return metadataStyle.Render("  " + current.text)
								case hunkRow:
									return hunkStyle.Render(current.text)
								case lineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+")
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-")
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: cursor.pane}
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
										}(), discardForeground
									}
									if cursor.row == y {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	testutil.RequireEqual(t, strings.Contains(addedRender, "\x1b[32m+\x1b[m"),
		true, "added=%q removed=%q", addedRender, removedRender,
	)
	testutil.RequireEqual(t, strings.Contains(removedRender, "\x1b[31m-\x1b[m"),
		true, "added=%q removed=%q", addedRender, removedRender,
	)

	var (
		rendered string = (func(
			v *diffView,
			cursor diffCursor,
			width int,
			selection *diffSelection,
		) string {
			return (func(
				v *diffView,
				target, active diffCursor,
				width int,
				selection *diffSelection,
			) string {
				viewport := func() diffViewport {
					var viewport diffViewport = diffViewport{}
					viewport.Width, viewport.Height = max(1, width), max(1, 1)
					{
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
								if current.kind != lineRow {
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
				return (func(v *diffView) func(
					viewport diffViewport,
					cursor diffCursor,
					selection *diffSelection,
				) string {
					return func(
						viewport diffViewport,
						cursor diffCursor,
						selection *diffSelection,
					) string {
						viewport = func() diffViewport {
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
									if current.kind != lineRow {
										continue
									}
									longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
								}
								return max(0, longest-contentWidth)
							}()))
							return viewport
						}()
						lines := make([]string, 0, viewport.Height)
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							current := v.rows[viewport.top]
							lines = append(
								lines,
								func() string {
									var path string = v.patch.Files[current.file].DisplayPath
									return fileStyle.Width(max(20, viewport.Width)).Render(path)
								}(),
							)
						}
						end := min(len(v.rows), viewport.top+func() int {
							height := viewport.Height
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								height--
							}
							return max(1, height)
						}())
						for y := viewport.top; y < end; y++ {
							current := v.rows[y]
							if v.split && current.kind == lineRow {
								lines = append(
									lines,
									(func(v *diffView) func(
										current entry,
										y int,
										viewport diffViewport,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										return func(
											current entry,
											y int,
											viewport diffViewport,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
												inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
												return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
											}()
											inlineLocalLeft1 := func() string {
												index := func() int {
													if current.kind != lineRow {
														return -1
													}
													if !v.split || left == right {
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
												if left == left {
													text, numberValue = current.left, line.OldNumber
												}
												prefix := "  "
												if line.Kind == patch.Addition {
													prefix = addedStyle.Render("+") + " "
												}
												if line.Kind == patch.Deletion {
													prefix = removedStyle.Render("-") + " "
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
													case patch.Addition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patch.Deletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return contextStyle
													}
												}()
												colors := preserveForeground
												candidate := diffCursor{row: y, pane: left}
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
													}(), discardForeground
												}
												if cursor == candidate {
													style, colors = cursorStyle, discardForeground
												}
												{
													var (
														style  lipgloss.Style   = style
														value  string           = value
														colors foregroundPolicy = colors
													)
													value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == discardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
											inlineLocalRight1 := (func(v *diffView) func(
												current entry,
												y int,
												pane diffPane,
												width, offset int,
												cursor diffCursor,
												selection *diffSelection,
											) string {
												return func(
													current entry,
													y int,
													pane diffPane,
													width, offset int,
													cursor diffCursor,
													selection *diffSelection,
												) string {
													index := func() int {
														if current.kind != lineRow {
															return -1
														}
														if !v.split || pane == right {
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
													if pane == left {
														text, numberValue = current.left, line.OldNumber
													}
													prefix := "  "
													if line.Kind == patch.Addition {
														prefix = addedStyle.Render("+") + " "
													}
													if line.Kind == patch.Deletion {
														prefix = removedStyle.Render("-") + " "
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
														case patch.Addition:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
														case patch.Deletion:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
														default:
															return contextStyle
														}
													}()
													colors := preserveForeground
													candidate := diffCursor{row: y, pane: pane}
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
														}(), discardForeground
													}
													if cursor == candidate {
														style, colors = cursorStyle, discardForeground
													}
													{
														var (
															style  lipgloss.Style   = style
															value  string           = value
															colors foregroundPolicy = colors
														)
														value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																	if code == 48 || colors == discardForeground && code == 38 {
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
																	if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
												right,
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
										case fileRow:
											return fileStyle.Width(max(20, width)).Render(current.text)
										case metadataRow:
											return metadataStyle.Render("  " + current.text)
										case hunkRow:
											return hunkStyle.Render(current.text)
										case lineRow:
											hunk := v.patch.Files[current.file].Hunks[current.hunk]
											line := hunk.Lines[current.rightLine]
											prefix := " "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+")
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-")
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: cursor.pane}
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
												}(), discardForeground
											}
											if cursor.row == y {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		expected sgrExpectation = sgrExpectation{reverse: true}
	)
	t.Helper()
	state := sgrState{}
	column := 0
	for len(rendered) > 0 {
		location := sgrPattern.FindStringSubmatchIndex(rendered)
		if location == nil {
			column = func() int {
				var (
					text   string   = rendered
					column int      = column
					state  sgrState = state
				)
				t.Helper()
				for range text {
					column++
					var column int = column
					t.Helper()
					testutil.RequireEqual(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
					testutil.RequireEqual(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
				}
				return column
			}()
			break
		}
		column = func() int {
			var (
				text   string   = rendered[:location[0]]
				column int      = column
				state  sgrState = state
			)
			t.Helper()
			for range text {
				column++
				var column int = column
				t.Helper()
				testutil.RequireEqual(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
				testutil.RequireEqual(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
			}
			return column
		}()
		var (
			s          *sgrState = &state
			parameters string    = rendered[location[2]:location[3]]
		)
		if parameters == "" {
			parameters = "0"
		}
		values := strings.Split(parameters, ";")
		for index := 0; index < len(values); index++ {
			value, _ := strconv.Atoi(values[index])
			switch value {
			case 0:
				*s = sgrState{}
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
	testutil.RequireEqual(t, column, 80, "styled columns = %d, want %d", column, 80)
}

func TestSelectionBackgroundKeepsDefaultWeight(t *testing.T) {
	v := (func(p patch.Patch, dark bool) *diffView {
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
	})(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, false)
	first := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	removed, _ := func() (diffCursor, bool) {
		var query string = "removed one"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(forward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if v.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: first.pane}
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
										return candidate, true
									}
									if v.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if first.pane == left {
												return right
											}
											return left
										}()}
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
									}
									return diffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffCursor{}, false
					}
					return diffCursor{}, false
				}
				panes := []diffPane{first.pane}
				if v.split {
					panes = append(panes, func() diffPane {
						if first.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
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
					}()
					if !ok {
						continue
					}
					line, _ := (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
						return func(cursor diffCursor) (patch.Line, bool) {
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
						}
					}(v))(candidate)
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffCursor{}, false
	}()
	selection := diffSelection{First: removed, Last: removed}
	rendered := (func(
		v *diffView,
		target, active diffCursor,
		width int,
		selection *diffSelection,
	) string {
		viewport := func() diffViewport {
			var viewport diffViewport = diffViewport{}
			viewport.Width, viewport.Height = max(1, width), max(1, 1)
			{
				var viewport diffViewport = viewport
				viewport.top = max(0, min(viewport.top, func() int {
					end := max(0, len(v.rows)-viewport.Height)
					if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
						if current.kind != lineRow {
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
		return (func(v *diffView) func(
			viewport diffViewport,
			cursor diffCursor,
			selection *diffSelection,
		) string {
			return func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				viewport = func() diffViewport {
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
								continue
							}
							longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
						}
						return max(0, longest-contentWidth)
					}()))
					return viewport
				}()
				lines := make([]string, 0, viewport.Height)
				if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
					current := v.rows[viewport.top]
					lines = append(
						lines,
						func() string {
							var path string = v.patch.Files[current.file].DisplayPath
							return fileStyle.Width(max(20, viewport.Width)).Render(path)
						}(),
					)
				}
				end := min(len(v.rows), viewport.top+func() int {
					height := viewport.Height
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						height--
					}
					return max(1, height)
				}())
				for y := viewport.top; y < end; y++ {
					current := v.rows[y]
					if v.split && current.kind == lineRow {
						lines = append(
							lines,
							(func(v *diffView) func(
								current entry,
								y int,
								viewport diffViewport,
								cursor diffCursor,
								selection *diffSelection,
							) string {
								return func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
										inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
										return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
									}()
									inlineLocalLeft1 := func() string {
										index := func() int {
											if current.kind != lineRow {
												return -1
											}
											if !v.split || left == right {
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
										if left == left {
											text, numberValue = current.left, line.OldNumber
										}
										prefix := "  "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+") + " "
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-") + " "
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: left}
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
											}(), discardForeground
										}
										if cursor == candidate {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
									inlineLocalRight1 := (func(v *diffView) func(
										current entry,
										y int,
										pane diffPane,
										width, offset int,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										return func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || pane == right {
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
											if pane == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: pane}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										right,
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
								case fileRow:
									return fileStyle.Width(max(20, width)).Render(current.text)
								case metadataRow:
									return metadataStyle.Render("  " + current.text)
								case hunkRow:
									return hunkStyle.Render(current.text)
								case lineRow:
									hunk := v.patch.Files[current.file].Hunks[current.hunk]
									line := hunk.Lines[current.rightLine]
									prefix := " "
									if line.Kind == patch.Addition {
										prefix = addedStyle.Render("+")
									}
									if line.Kind == patch.Deletion {
										prefix = removedStyle.Render("-")
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
										case patch.Addition:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
										case patch.Deletion:
											return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
										default:
											return contextStyle
										}
									}()
									colors := preserveForeground
									candidate := diffCursor{row: y, pane: cursor.pane}
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
										}(), discardForeground
									}
									if cursor.row == y {
										style, colors = cursorStyle, discardForeground
									}
									{
										var (
											style  lipgloss.Style   = style
											value  string           = value
											colors foregroundPolicy = colors
										)
										value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
													if code == 48 || colors == discardForeground && code == 38 {
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
													if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
	testutil.RequireEqual(t, !strings.Contains(rendered, "\x1b[1m"), true,
		"selection is bold: %q", rendered,
	)

	{
		var (
			rendered string         = rendered
			expected sgrExpectation = sgrExpectation{background: "219;234;254"}
		)
		t.Helper()
		state := sgrState{}
		column := 0
		for len(rendered) > 0 {
			location := sgrPattern.FindStringSubmatchIndex(rendered)
			if location == nil {
				column = func() int {
					var (
						text   string   = rendered
						column int      = column
						state  sgrState = state
					)
					t.Helper()
					for range text {
						column++
						var column int = column
						t.Helper()
						testutil.RequireEqual(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
						testutil.RequireEqual(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
					}
					return column
				}()
				break
			}
			column = func() int {
				var (
					text   string   = rendered[:location[0]]
					column int      = column
					state  sgrState = state
				)
				t.Helper()
				for range text {
					column++
					var column int = column
					t.Helper()
					testutil.RequireEqual(t, state.background, expected.background, "column %d state=%#v want=%#v", column, state, expected)
					testutil.RequireEqual(t, state.reverse, expected.reverse, "column %d state=%#v want=%#v", column, state, expected)
				}
				return column
			}()
			var (
				s          *sgrState = &state
				parameters string    = rendered[location[2]:location[3]]
			)
			if parameters == "" {
				parameters = "0"
			}
			values := strings.Split(parameters, ";")
			for index := 0; index < len(values); index++ {
				value, _ := strconv.Atoi(values[index])
				switch value {
				case 0:
					*s = sgrState{}
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
		testutil.RequireEqual(t, column, 72, "styled columns = %d, want %d", column, 72)
	}
}

func TestSyntaxHighlightingSurvivesDiffStyling(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "main.go",
				OldPath:     "main.go",
				NewPath:     "main.go",
				OldSource:   "package main\nold()\n",
				NewSource:   "package main\nnew()\n",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
							{Kind: patch.Addition, Text: "new()", NewNumber: 2},
						},
					},
				},
			},
		},
	}
	v := (func(p patch.Patch, dark bool) *diffView {
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
	first := func() diffCursor {
		t.Helper()
		cursor, ok := func() (diffCursor, bool) {
			if cursor, ok := func() (diffCursor, bool) {
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: right}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return cursor, true
			}
			step := int(forward)
			for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
				if cursor, ok := func() (diffCursor, bool) {
					cursor := diffCursor{row: y, pane: left}
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
					return cursor, true
				}
			}
			return diffCursor{}, false
		}()
		testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
		return cursor
	}()
	added, _ := func() (diffCursor, bool) {
		var query string = "new()"
		if query == "" || !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := first.row
		for count := 0; count < len(v.rows)-1; count++ {
			y = (y + int(forward) + len(v.rows)) % len(v.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if v.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(v.rows[y].text)), query) {
						var y int = y
						for distance := 1; distance <= len(v.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(v.rows) || v.rows[candidateY].file != v.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: first.pane}
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
										return candidate, true
									}
									if v.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if first.pane == left {
												return right
											}
											return left
										}()}
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
									}
									return diffCursor{}, false
								}(); ok {
									return candidate, true
								}
							}
						}
						return diffCursor{}, false
					}
					return diffCursor{}, false
				}
				panes := []diffPane{first.pane}
				if v.split {
					panes = append(panes, func() diffPane {
						if first.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
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
					}()
					if !ok {
						continue
					}
					line, _ := (func(v *diffView) func(cursor diffCursor) (patch.Line, bool) {
						return func(cursor diffCursor) (patch.Line, bool) {
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
						}
					}(v))(candidate)
					if strings.Contains(strings.ToLower(line.Text), query) {
						return candidate, true
					}
				}
				return diffCursor{}, false
			}(); ok {
				return candidate, true
			}
		}
		return diffCursor{}, false
	}()
	for _, cursor := range []diffCursor{first, added} {
		rendered := (func(
			v *diffView,
			target, active diffCursor,
			width int,
			selection *diffSelection,
		) string {
			viewport := func() diffViewport {
				var viewport diffViewport = diffViewport{}
				viewport.Width, viewport.Height = max(1, width), max(1, 1)
				{
					var viewport diffViewport = viewport
					viewport.top = max(0, min(viewport.top, func() int {
						end := max(0, len(v.rows)-viewport.Height)
						if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
							if current.kind != lineRow {
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
			return (func(v *diffView) func(
				viewport diffViewport,
				cursor diffCursor,
				selection *diffSelection,
			) string {
				return func(
					viewport diffViewport,
					cursor diffCursor,
					selection *diffSelection,
				) string {
					viewport = func() diffViewport {
						var viewport diffViewport = viewport
						viewport.top = max(0, min(viewport.top, func() int {
							end := max(0, len(v.rows)-viewport.Height)
							if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
								if current.kind != lineRow {
									continue
								}
								longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
							}
							return max(0, longest-contentWidth)
						}()))
						return viewport
					}()
					lines := make([]string, 0, viewport.Height)
					if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
						current := v.rows[viewport.top]
						lines = append(
							lines,
							func() string {
								var path string = v.patch.Files[current.file].DisplayPath
								return fileStyle.Width(max(20, viewport.Width)).Render(path)
							}(),
						)
					}
					end := min(len(v.rows), viewport.top+func() int {
						height := viewport.Height
						if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
							height--
						}
						return max(1, height)
					}())
					for y := viewport.top; y < end; y++ {
						current := v.rows[y]
						if v.split && current.kind == lineRow {
							lines = append(
								lines,
								(func(v *diffView) func(
									current entry,
									y int,
									viewport diffViewport,
									cursor diffCursor,
									selection *diffSelection,
								) string {
									return func(
										current entry,
										y int,
										viewport diffViewport,
										cursor diffCursor,
										selection *diffSelection,
									) string {
										leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
											inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
											return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
										}()
										inlineLocalLeft1 := func() string {
											index := func() int {
												if current.kind != lineRow {
													return -1
												}
												if !v.split || left == right {
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
											if left == left {
												text, numberValue = current.left, line.OldNumber
											}
											prefix := "  "
											if line.Kind == patch.Addition {
												prefix = addedStyle.Render("+") + " "
											}
											if line.Kind == patch.Deletion {
												prefix = removedStyle.Render("-") + " "
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
												case patch.Addition:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
												case patch.Deletion:
													return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
												default:
													return contextStyle
												}
											}()
											colors := preserveForeground
											candidate := diffCursor{row: y, pane: left}
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
												}(), discardForeground
											}
											if cursor == candidate {
												style, colors = cursorStyle, discardForeground
											}
											{
												var (
													style  lipgloss.Style   = style
													value  string           = value
													colors foregroundPolicy = colors
												)
												value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
															if code == 48 || colors == discardForeground && code == 38 {
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
															if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
										inlineLocalRight1 := (func(v *diffView) func(
											current entry,
											y int,
											pane diffPane,
											width, offset int,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											return func(
												current entry,
												y int,
												pane diffPane,
												width, offset int,
												cursor diffCursor,
												selection *diffSelection,
											) string {
												index := func() int {
													if current.kind != lineRow {
														return -1
													}
													if !v.split || pane == right {
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
												if pane == left {
													text, numberValue = current.left, line.OldNumber
												}
												prefix := "  "
												if line.Kind == patch.Addition {
													prefix = addedStyle.Render("+") + " "
												}
												if line.Kind == patch.Deletion {
													prefix = removedStyle.Render("-") + " "
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
													case patch.Addition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patch.Deletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return contextStyle
													}
												}()
												colors := preserveForeground
												candidate := diffCursor{row: y, pane: pane}
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
													}(), discardForeground
												}
												if cursor == candidate {
													style, colors = cursorStyle, discardForeground
												}
												{
													var (
														style  lipgloss.Style   = style
														value  string           = value
														colors foregroundPolicy = colors
													)
													value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == discardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
											right,
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
									case fileRow:
										return fileStyle.Width(max(20, width)).Render(current.text)
									case metadataRow:
										return metadataStyle.Render("  " + current.text)
									case hunkRow:
										return hunkStyle.Render(current.text)
									case lineRow:
										hunk := v.patch.Files[current.file].Hunks[current.hunk]
										line := hunk.Lines[current.rightLine]
										prefix := " "
										if line.Kind == patch.Addition {
											prefix = addedStyle.Render("+")
										}
										if line.Kind == patch.Deletion {
											prefix = removedStyle.Render("-")
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
											case patch.Addition:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
											case patch.Deletion:
												return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
											default:
												return contextStyle
											}
										}()
										colors := preserveForeground
										candidate := diffCursor{row: y, pane: cursor.pane}
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
											}(), discardForeground
										}
										if cursor.row == y {
											style, colors = cursorStyle, discardForeground
										}
										{
											var (
												style  lipgloss.Style   = style
												value  string           = value
												colors foregroundPolicy = colors
											)
											value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
														if code == 48 || colors == discardForeground && code == 38 {
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
														if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		})(v, cursor, diffCursor{}, 80, nil)
		testutil.RequireEqual(t, strings.Contains(rendered, "[38;2;"), true,
			"syntax highlighting missing: %q", rendered,
		)
	}
}

func TestRenderedCodeRowsHaveExactTerminalWidth(t *testing.T) {
	for _, test := range []struct {
		constructor func(patch.Patch, bool) *diffView
		width       int
	}{{(func(p patch.Patch, dark bool) *diffView {
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
	}), 37}, {(func(p patch.Patch, dark bool) *diffView {
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
	}), 120}} {
		v := test.constructor(patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}, true)
		cursor := func() diffCursor {
			t.Helper()
			cursor, ok := func() (diffCursor, bool) {
				if cursor, ok := func() (diffCursor, bool) {
					step := int(forward)
					for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
						if cursor, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: right}
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
							return cursor, true
						}
					}
					return diffCursor{}, false
				}(); ok {
					return cursor, true
				}
				step := int(forward)
				for y := -1 + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: left}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}()
			testutil.RequireEqual(t, ok, true, "%s", "no first cursor")
			return cursor
		}()
		for {
			width := lipgloss.Width((func(
				v *diffView,
				cursor diffCursor,
				width int,
				selection *diffSelection,
			) string {
				return (func(
					v *diffView,
					target, active diffCursor,
					width int,
					selection *diffSelection,
				) string {
					viewport := func() diffViewport {
						var viewport diffViewport = diffViewport{}
						viewport.Width, viewport.Height = max(1, width), max(1, 1)
						{
							var viewport diffViewport = viewport
							viewport.top = max(0, min(viewport.top, func() int {
								end := max(0, len(v.rows)-viewport.Height)
								if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
									if current.kind != lineRow {
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
					return (func(v *diffView) func(
						viewport diffViewport,
						cursor diffCursor,
						selection *diffSelection,
					) string {
						return func(
							viewport diffViewport,
							cursor diffCursor,
							selection *diffSelection,
						) string {
							viewport = func() diffViewport {
								var viewport diffViewport = viewport
								viewport.top = max(0, min(viewport.top, func() int {
									end := max(0, len(v.rows)-viewport.Height)
									if viewport.Height > 1 && end >= 0 && end < len(v.rows) && v.rows[end].kind != fileRow {
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
										if current.kind != lineRow {
											continue
										}
										longest = max(longest, lipgloss.Width(strings.ReplaceAll(ansi.Strip(current.text+current.left+current.right), "\t", "    "))+extra)
									}
									return max(0, longest-contentWidth)
								}()))
								return viewport
							}()
							lines := make([]string, 0, viewport.Height)
							if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
								current := v.rows[viewport.top]
								lines = append(
									lines,
									func() string {
										var path string = v.patch.Files[current.file].DisplayPath
										return fileStyle.Width(max(20, viewport.Width)).Render(path)
									}(),
								)
							}
							end := min(len(v.rows), viewport.top+func() int {
								height := viewport.Height
								if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
									height--
								}
								return max(1, height)
							}())
							for y := viewport.top; y < end; y++ {
								current := v.rows[y]
								if v.split && current.kind == lineRow {
									lines = append(
										lines,
										(func(v *diffView) func(
											current entry,
											y int,
											viewport diffViewport,
											cursor diffCursor,
											selection *diffSelection,
										) string {
											return func(
												current entry,
												y int,
												viewport diffViewport,
												cursor diffCursor,
												selection *diffSelection,
											) string {
												leftWidth, rightWidth := func() (inlineLocalLeft2, inlineLocalRight2 int) {
													inlineLocalLeft2 = max(20, (viewport.Width-3)/2)
													return inlineLocalLeft2, max(20, viewport.Width-3-inlineLocalLeft2)
												}()
												inlineLocalLeft1 := func() string {
													index := func() int {
														if current.kind != lineRow {
															return -1
														}
														if !v.split || left == right {
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
													if left == left {
														text, numberValue = current.left, line.OldNumber
													}
													prefix := "  "
													if line.Kind == patch.Addition {
														prefix = addedStyle.Render("+") + " "
													}
													if line.Kind == patch.Deletion {
														prefix = removedStyle.Render("-") + " "
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
														case patch.Addition:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
														case patch.Deletion:
															return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
														default:
															return contextStyle
														}
													}()
													colors := preserveForeground
													candidate := diffCursor{row: y, pane: left}
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
														}(), discardForeground
													}
													if cursor == candidate {
														style, colors = cursorStyle, discardForeground
													}
													{
														var (
															style  lipgloss.Style   = style
															value  string           = value
															colors foregroundPolicy = colors
														)
														value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																	if code == 48 || colors == discardForeground && code == 38 {
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
																	if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
												inlineLocalRight1 := (func(v *diffView) func(
													current entry,
													y int,
													pane diffPane,
													width, offset int,
													cursor diffCursor,
													selection *diffSelection,
												) string {
													return func(
														current entry,
														y int,
														pane diffPane,
														width, offset int,
														cursor diffCursor,
														selection *diffSelection,
													) string {
														index := func() int {
															if current.kind != lineRow {
																return -1
															}
															if !v.split || pane == right {
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
														if pane == left {
															text, numberValue = current.left, line.OldNumber
														}
														prefix := "  "
														if line.Kind == patch.Addition {
															prefix = addedStyle.Render("+") + " "
														}
														if line.Kind == patch.Deletion {
															prefix = removedStyle.Render("-") + " "
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
															case patch.Addition:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
															case patch.Deletion:
																return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
															default:
																return contextStyle
															}
														}()
														colors := preserveForeground
														candidate := diffCursor{row: y, pane: pane}
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
															}(), discardForeground
														}
														if cursor == candidate {
															style, colors = cursorStyle, discardForeground
														}
														{
															var (
																style  lipgloss.Style   = style
																value  string           = value
																colors foregroundPolicy = colors
															)
															value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																		if code == 48 || colors == discardForeground && code == 38 {
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
																		if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
													right,
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
											case fileRow:
												return fileStyle.Width(max(20, width)).Render(current.text)
											case metadataRow:
												return metadataStyle.Render("  " + current.text)
											case hunkRow:
												return hunkStyle.Render(current.text)
											case lineRow:
												hunk := v.patch.Files[current.file].Hunks[current.hunk]
												line := hunk.Lines[current.rightLine]
												prefix := " "
												if line.Kind == patch.Addition {
													prefix = addedStyle.Render("+")
												}
												if line.Kind == patch.Deletion {
													prefix = removedStyle.Render("-")
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
													case patch.Addition:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
													case patch.Deletion:
														return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
													default:
														return contextStyle
													}
												}()
												colors := preserveForeground
												candidate := diffCursor{row: y, pane: cursor.pane}
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
													}(), discardForeground
												}
												if cursor.row == y {
													style, colors = cursorStyle, discardForeground
												}
												{
													var (
														style  lipgloss.Style   = style
														value  string           = value
														colors foregroundPolicy = colors
													)
													value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
																if code == 48 || colors == discardForeground && code == 38 {
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
																if code >= 40 && code <= 49 || code >= 100 && code <= 107 || colors == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
			testutil.RequireEqual(t, width, test.width, "width=%d want=%d",
				width, test.width,
			)
			next, ok := func() (diffCursor, bool) {
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
					return diffCursor{}, false
				}
				var pane diffPane = cursor.pane
				step := int(forward)
				for y := cursor.row + step; y >= 0 && y < len(v.rows); y += step {
					if cursor, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: pane}
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
						return cursor, true
					}
				}
				return diffCursor{}, false
			}()
			if !ok {
				break
			}
			cursor = next
		}
	}
}

func TestRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
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
				switch patch.Addition {
				case patch.Addition:
					return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823")))
				case patch.Deletion:
					return lipgloss.NewStyle().Background(lightDark(lipgloss.Color("#ffebe9"), lipgloss.Color("#402222")))
				default:
					return contextStyle
				}
			}()
			value string = value
		)
		value = ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
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
					if code == 48 || preserveForeground == discardForeground && code == 38 {
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
					if code >= 40 && code <= 49 || code >= 100 && code <= 107 || preserveForeground == discardForeground && (code >= 30 && code <= 39 || code >= 90 && code <= 97) {
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
		testutil.RequireEqual(t, !strings.Contains(rendered, forbidden), true,
			"retains %q: %q", forbidden, rendered,
		)
	}
}

type sgrExpectation struct {
	background string
	reverse    bool
}
type sgrState struct {
	background string
	reverse    bool
}

var sgrPattern = regexp.MustCompile(`\x1b\[([0-9;]*)m`)
