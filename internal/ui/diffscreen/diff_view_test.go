package diffscreen

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestUnifiedNavigationSearchAndFileJumps(t *testing.T) {
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
	first, ok := func() (diffCursor, bool) {
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
	testutil.RequireEqual(t, ok, true, "%s", "First returned no cursor")

	line, _ := func() (patch.Line, bool) {
		if !(first.row >= 0 && first.row < len(v.rows) && func() int {
			var current entry = v.rows[first.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[first.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || first.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, line.Text, "before", "first line = %q", line.Text)

	next, ok := func() (diffCursor, bool) {
		if !(first.row >= 0 && first.row < len(v.rows) && func() int {
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
		step := int(forward)
		for y := first.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: first.pane}
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
	testutil.RequireEqual(t, ok, true, "%s", "Move returned no cursor")

	line, _ = func() (patch.Line, bool) {
		if !(next.row >= 0 && next.row < len(v.rows) && func() int {
			var current entry = v.rows[next.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || next.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[next.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || next.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, line.Kind, patch.Deletion, "next kind = %v",
		line.Kind,
	)

	match, ok := func() (diffCursor, bool) {
		var query string = "added"
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
	testutil.RequireEqual(t, ok, true, "%s", "Search returned no cursor")

	line, _ = func() (patch.Line, bool) {
		if !(match.row >= 0 && match.row < len(v.rows) && func() int {
			var current entry = v.rows[match.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || match.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[match.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || match.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, line.Text, "added one", "match = %q", line.Text)

	jumped, ok := func() (diffCursor, bool) {
		if !(first.row >= 0 && first.row < len(v.rows) && func() int {
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
		file, _ := func() (patch.File, bool) {
			if !(first.row >= 0 && first.row < len(v.rows) && func() int {
				var current entry = v.rows[first.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || first.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return patch.File{}, false
			}
			return v.patch.Files[v.rows[first.row].file], true
		}()
		y := first.row
		for {
			next, ok := func() (diffCursor, bool) {
				var (
					start int      = y
					pane  diffPane = first.pane
				)
				step := int(forward)
				for y := start + step; y >= 0 && y < len(v.rows); y += step {
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
				return diffCursor{}, false
			}
			nextFile, _ := func() (patch.File, bool) {
				if !(next.row >= 0 && next.row < len(v.rows) && func() int {
					var current entry = v.rows[next.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || next.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.File{}, false
				}
				return v.patch.Files[v.rows[next.row].file], true
			}()
			if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
				return next, true
			}
			y = next.row
		}
	}()
	testutil.RequireEqual(t, ok, true, "%s", "JumpFile returned no cursor")

	file, _ := func() (patch.File, bool) {
		if !(jumped.row >= 0 && jumped.row < len(v.rows) && func() int {
			var current entry = v.rows[jumped.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || jumped.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.File{}, false
		}
		return v.patch.Files[v.rows[jumped.row].file], true
	}()
	testutil.RequireEqual(t, file.DisplayPath, "second.go", "jumped file = %q",
		file.DisplayPath,
	)

	if _, ok := func() (diffCursor, bool) {
		if !(jumped.row >= 0 && jumped.row < len(v.rows) && func() int {
			var current entry = v.rows[jumped.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || jumped.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		file, _ := func() (patch.File, bool) {
			if !(jumped.row >= 0 && jumped.row < len(v.rows) && func() int {
				var current entry = v.rows[jumped.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || jumped.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return patch.File{}, false
			}
			return v.patch.Files[v.rows[jumped.row].file], true
		}()
		y := jumped.row
		for {
			next, ok := func() (diffCursor, bool) {
				var (
					start int      = y
					pane  diffPane = jumped.pane
				)
				step := int(forward)
				for y := start + step; y >= 0 && y < len(v.rows); y += step {
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
				return diffCursor{}, false
			}
			nextFile, _ := func() (patch.File, bool) {
				if !(next.row >= 0 && next.row < len(v.rows) && func() int {
					var current entry = v.rows[next.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || next.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.File{}, false
				}
				return v.patch.Files[v.rows[next.row].file], true
			}()
			if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
				return next, true
			}
			y = next.row
		}
	}(); ok {
		t.Fatal("JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
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
	first, _ := func() (diffCursor, bool) {
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
	removed, _ := func() (diffCursor, bool) {
		if !(first.row >= 0 && first.row < len(v.rows) && func() int {
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
		step := int(forward)
		for y := first.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: first.pane}
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
	testutil.RequireEqual(t, removed.pane, right, "initial pane = %v",
		removed.pane,
	)

	removed, ok := func() (diffCursor, bool) {
		if !v.split || !(removed.row >= 0 && removed.row < len(v.rows) && func() int {
			var current entry = v.rows[removed.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || removed.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = removed.row
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
			for y := removed.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[removed.row].file {
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
		for y := removed.row + step; y >= 0 && y < len(v.rows); y += step {
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
	testutil.RequireEqual(t, ok, true, "%s",
		"could not switch to deletion pane",
	)

	line, _ := func() (patch.Line, bool) {
		if !(removed.row >= 0 && removed.row < len(v.rows) && func() int {
			var current entry = v.rows[removed.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || removed.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[removed.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || removed.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, line.Text, "removed one", "left line = %q",
		line.Text,
	)

	added, ok := func() (diffCursor, bool) {
		if !v.split || !(removed.row >= 0 && removed.row < len(v.rows) && func() int {
			var current entry = v.rows[removed.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || removed.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = removed.row
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
			for y := removed.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[removed.row].file {
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
		for y := removed.row + step; y >= 0 && y < len(v.rows); y += step {
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
	testutil.RequireEqual(t, ok, true, "%s", "paired addition missing")

	line, _ = func() (patch.Line, bool) {
		if !(added.row >= 0 && added.row < len(v.rows) && func() int {
			var current entry = v.rows[added.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || added.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := v.rows[added.row]
		return v.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !v.split || added.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, line.Text, "added one", "right line = %q",
		line.Text,
	)

	secondRemoved, _ := func() (diffCursor, bool) {
		if !(removed.row >= 0 && removed.row < len(v.rows) && func() int {
			var current entry = v.rows[removed.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || removed.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		var pane diffPane = removed.pane
		step := int(forward)
		for y := removed.row + step; y >= 0 && y < len(v.rows); y += step {
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
	if _, ok := func() (diffCursor, bool) {
		if !v.split || !(secondRemoved.row >= 0 && secondRemoved.row < len(v.rows) && func() int {
			var current entry = v.rows[secondRemoved.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || secondRemoved.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		if candidate, ok := func() (diffCursor, bool) {
			var y int = secondRemoved.row
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
			for y := secondRemoved.row - 1; y >= 0; y-- {
				if v.rows[y].file != v.rows[secondRemoved.row].file {
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
		for y := secondRemoved.row + step; y >= 0 && y < len(v.rows); y += step {
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
	}(); !ok {
		t.Fatal("pane switch should find a nearby right line")
	}

	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 100), max(1, 20)
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
	rendered := (func(v *diffView) func(
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
	}(v))(viewport, added, nil)
	testutil.RequireEqual(t, strings.Contains(rendered, "removed one"), true,
		"paired render missing lines: %q", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered, "added one"), true,
		"paired render missing lines: %q", rendered,
	)

	for _, row := range strings.Split(rendered, "\n") {
		testutil.RequireEqual(t,
			!strings.Contains(row, " │ ") || lipgloss.Width(row) == 100, true,
			"rendered width = %d", lipgloss.Width(row),
		)
	}
}

func TestSelectionLines(t *testing.T) {
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
	first, _ := func() (diffCursor, bool) {
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
	last, _ := func() (diffCursor, bool) {
		if !(first.row >= 0 && first.row < len(v.rows) && func() int {
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
		step := int(forward)
		for y := first.row + step; y >= 0 && y < len(v.rows); y += step {
			if cursor, ok := func() (diffCursor, bool) {
				cursor := diffCursor{row: y, pane: first.pane}
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
	last, _ = func() (diffCursor, bool) {
		if !(last.row >= 0 && last.row < len(v.rows) && func() int {
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
		var pane diffPane = last.pane
		step := int(forward)
		for y := last.row + step; y >= 0 && y < len(v.rows); y += step {
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
	selection := diffSelection{First: first, Last: first}
	selection, ok := func() (diffSelection, bool) {
		var (
			selection diffSelection = selection
			cursor    diffCursor    = last
		)
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
		}() >= 0) || !(cursor.row >= 0 && cursor.row < len(v.rows) && func() int {
			var current entry = v.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return selection, false
		}
		first := v.rows[selection.First.row]
		last := v.rows[cursor.row]
		if first.file != last.file || first.hunk != last.hunk {
			return selection, false
		}
		selection.Last = cursor
		return selection, true
	}()
	testutil.RequireEqual(t, ok, true, "%s", "selection extension failed")

	lines := func() []patch.Line {
		var selection diffSelection = selection
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
	}()
	testutil.RequireEqual(t, len(lines), 3, "selected lines = %d", len(lines))

	nextFile, _ := func() (diffCursor, bool) {
		if !(first.row >= 0 && first.row < len(v.rows) && func() int {
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
		file, _ := func() (patch.File, bool) {
			if !(first.row >= 0 && first.row < len(v.rows) && func() int {
				var current entry = v.rows[first.row]
				if current.kind != lineRow {
					return -1
				}
				if !v.split || first.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return patch.File{}, false
			}
			return v.patch.Files[v.rows[first.row].file], true
		}()
		y := first.row
		for {
			next, ok := func() (diffCursor, bool) {
				var (
					start int      = y
					pane  diffPane = first.pane
				)
				step := int(forward)
				for y := start + step; y >= 0 && y < len(v.rows); y += step {
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
				return diffCursor{}, false
			}
			nextFile, _ := func() (patch.File, bool) {
				if !(next.row >= 0 && next.row < len(v.rows) && func() int {
					var current entry = v.rows[next.row]
					if current.kind != lineRow {
						return -1
					}
					if !v.split || next.pane == right {
						return current.rightLine
					}
					return current.leftLine
				}() >= 0) {
					return patch.File{}, false
				}
				return v.patch.Files[v.rows[next.row].file], true
			}()
			if nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath {
				return next, true
			}
			y = next.row
		}
	}()
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
		}() >= 0) || !(nextFile.row >= 0 && nextFile.row < len(v.rows) && func() int {
			var current entry = v.rows[nextFile.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || nextFile.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return selection, false
		}
		first := v.rows[selection.First.row]
		last := v.rows[nextFile.row]
		if first.file != last.file || first.hunk != last.hunk {
			return selection, false
		}
		selection.Last = nextFile
		return selection, true
	}(); ok {
		t.Fatal("selection crossed a hunk")
	}
}

func TestViewportAlignmentResizeAndScrolling(t *testing.T) {
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
	first, _ := func() (diffCursor, bool) {
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
	cursor := first
	for range 8 {
		cursor, _ = func() (diffCursor, bool) {
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
	}
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 30), max(1, 5)
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
	testutil.RequireEqual(t, cursor.row >= viewport.top, true,
		"cursor not visible: %#v %#v", cursor, viewport,
	)
	testutil.RequireEqual(t, cursor.row < viewport.top+viewport.Height, true,
		"cursor not visible: %#v %#v", cursor, viewport,
	)

	viewport = func() diffViewport {
		var (
			viewport diffViewport = viewport
			cursor   diffCursor   = cursor
		)
		headerHeight := 0
		if viewport.Height > 1 {
			headerHeight = 1
		}
		offset := max(0, func() int {
			var height int = viewport.Height
			if middle == middle {
				return height / 2
			}
			if middle == bottom {
				return height - 1
			}
			return 0
		}()-headerHeight)
		viewport.top = cursor.row - offset
		if !(viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow) {
			viewport.top = cursor.row - func() int {
				var height int = viewport.Height
				if middle == middle {
					return height / 2
				}
				if middle == bottom {
					return height - 1
				}
				return 0
			}()
		}
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
	headerHeight := 0
	if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
		headerHeight = 1
	}
	testutil.RequireEqual(t, headerHeight+cursor.row-viewport.top,
		viewport.Height/2, "middle alignment = %#v", viewport,
	)

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
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true, "%s",
		"horizontal scroll did not move",
	)

	viewport = func() diffViewport {
		var viewport diffViewport = viewport
		viewport.Width, viewport.Height = max(1, 20), max(1, 3)
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
	testutil.RequireEqual(t, viewport.Width, 20, "resize = %#v", viewport)
	testutil.RequireEqual(t, viewport.Height, 3, "resize = %#v", viewport)

	before := cursor
	viewport, cursor = func() (diffViewport, diffCursor) {
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
	testutil.RequireEqual(t, cursor.row >= before.row, true,
		"half page moved backward: %#v -> %#v", before, cursor,
	)
}

func TestViewportProgressUsesVisibleBottom(t *testing.T) {
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
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 30), max(1, 5)
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
	progress := func() int {
		var viewport diffViewport = viewport
		if len(v.rows) == 0 {
			return 0
		}
		inlineLocalBottom2 := min(len(v.rows), viewport.top+func() int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				height--
			}
			return max(1, height)
		}())
		return inlineLocalBottom2 * 100 / len(v.rows)
	}()
	if progress <= 0 || progress >= 100 {
		t.Fatalf("initial progress=%d", progress)
	}
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
	viewport = func() diffViewport {
		var viewport diffViewport = viewport
		if !(last.row >= 0 && last.row < len(v.rows) && func() int {
			var current entry = v.rows[last.row]
			if current.kind != lineRow {
				return -1
			}
			if !v.split || last.pane == right {
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
			if last.row < viewport.top {
				viewport.top = last.row
			}
			if last.row >= viewport.top+height {
				viewport.top = last.row - height + 1
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
	if progress := func() int {
		var viewport diffViewport = viewport
		if len(v.rows) == 0 {
			return 0
		}
		inlineLocalBottom1 := min(len(v.rows), viewport.top+func() int {
			height := viewport.Height
			if viewport.Height > 1 && viewport.top >= 0 && viewport.top < len(v.rows) && v.rows[viewport.top].kind != fileRow {
				height--
			}
			return max(1, height)
		}())
		return inlineLocalBottom1 * 100 / len(v.rows)
	}(); progress != 100 {
		t.Fatalf("final progress=%d", progress)
	}
}

func TestHalfPageScrollingMovesCursorToFileBoundaries(t *testing.T) {
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
	first, _ := func() (diffCursor, bool) {
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
	viewport := func() diffViewport {
		var viewport diffViewport = diffViewport{}
		viewport.Width, viewport.Height = max(1, 30), max(1, 5)
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
	cursor := first

	for range len((func() patch.Patch {
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
	})().Files[0].Hunks[0].Lines) {
		viewport, cursor = func() (diffViewport, diffCursor) {
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
	}
	testutil.RequireEqual(t, cursor, last,
		"cursor after scrolling down = %#v, want %#v", cursor, last,
	)

	for range len((func() patch.Patch {
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
	})().Files[0].Hunks[0].Lines) {
		viewport, cursor = func() (diffViewport, diffCursor) {
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
	}
	testutil.RequireEqual(t, cursor, first,
		"cursor after scrolling up = %#v, want %#v", cursor, first,
	)
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}
	oldView := (func(p patch.Patch, dark bool) *diffView {
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
	})(original, true)
	cursor, _ := func() (diffCursor, bool) {
		var (
			query  string     = "added one"
			cursor diffCursor = func() diffCursor {
				t.Helper()
				cursor, ok := func() (diffCursor, bool) {
					if cursor, ok := func() (diffCursor, bool) {
						step := int(forward)
						for y := -1 + step; y >= 0 && y < len(oldView.rows); y += step {
							if cursor, ok := func() (diffCursor, bool) {
								cursor := diffCursor{row: y, pane: right}
								return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
									var current entry = oldView.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !oldView.split || cursor.pane == right {
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
					for y := -1 + step; y >= 0 && y < len(oldView.rows); y += step {
						if cursor, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: left}
							return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
								var current entry = oldView.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !oldView.split || cursor.pane == right {
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
		)
		if query == "" || !(cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
			var current entry = oldView.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !oldView.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := cursor.row
		for count := 0; count < len(oldView.rows)-1; count++ {
			y = (y + int(forward) + len(oldView.rows)) % len(oldView.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if oldView.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(oldView.rows[y].text)), query) {
						var (
							y    int      = y
							pane diffPane = cursor.pane
						)
						for distance := 1; distance <= len(oldView.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(oldView.rows) || oldView.rows[candidateY].file != oldView.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
											var current entry = oldView.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !oldView.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
									if oldView.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if pane == left {
												return right
											}
											return left
										}()}
										return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
											var current entry = oldView.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !oldView.split || cursor.pane == right {
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
				panes := []diffPane{cursor.pane}
				if oldView.split {
					panes = append(panes, func() diffPane {
						if cursor.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
						return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
							var current entry = oldView.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !oldView.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(oldView.rows) && func() int {
							var current entry = oldView.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !oldView.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := oldView.rows[candidate.row]
						return oldView.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !oldView.split || candidate.pane == right {
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
	line, _ := func() (patch.Line, bool) {
		if !(cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
			var current entry = oldView.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !oldView.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := oldView.rows[cursor.row]
		return oldView.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !oldView.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	changed := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := (func(p patch.Patch, dark bool) *diffView {
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
	})(changed, true)
	translated, ok := func() (diffCursor, bool) {
		var target cursorIdentity = func() cursorIdentity {
			if !(cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
				var current entry = oldView.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !oldView.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return cursorIdentity{}
			}
			row := oldView.rows[cursor.row]
			file := oldView.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != lineRow {
					return -1
				}
				if !oldView.split || cursor.pane == right {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		if !target.valid {
			return diffCursor{}, false
		}
		best := cursorMatch{}
		for y := range newView.rows {
			candidate := func() cursorMatch {
				current := newView.rows[y]
				if !(current.hunk >= 0 && func() bool {
					var candidate patch.File = newView.patch.Files[current.file]
					return candidate.OldPath != "" && candidate.OldPath == target.file.OldPath || candidate.NewPath != "" && candidate.NewPath == target.file.NewPath
				}()) {
					return cursorMatch{}
				}
				best := cursorMatch{}
				for _, pane := range [2]diffPane{target.cursor.pane, func() diffPane {
					if target.cursor.pane == left {
						return right
					}
					return left
				}()} {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(newView.rows) && func() int {
							var current entry = newView.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !newView.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(newView.rows) && func() int {
							var current entry = newView.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !newView.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := newView.rows[candidate.row]
						return newView.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !newView.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					match := cursorMatch{cursor: candidate, rank: func() int {
						if line.Kind != target.line.Kind {
							return 1
						}
						if line.OldNumber == target.line.OldNumber && line.NewNumber == target.line.NewNumber {
							return 3
						}
						if line.Text == target.line.Text {
							return 2
						}
						return 1
					}(), distance: func() int {
						var rowDistance int = y - target.cursor.row
						if line.NewNumber > 0 && target.line.NewNumber > 0 {
							if int(line.NewNumber-target.line.NewNumber) < 0 {
								return -int(line.NewNumber - target.line.NewNumber)
							}
							return int(line.NewNumber - target.line.NewNumber)
						}
						if line.OldNumber > 0 && target.line.OldNumber > 0 {
							if int(line.OldNumber-target.line.OldNumber) < 0 {
								return -int(line.OldNumber - target.line.OldNumber)
							}
							return int(line.OldNumber - target.line.OldNumber)
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
	testutil.RequireEqual(t, ok, true, "%s", "semantic cursor was not found")
	testutil.RequireEqual(t, translated.row != cursor.row, true, "%s",
		"cursor coordinate was reused after rows shifted",
	)

	translatedLine, _ := func() (patch.Line, bool) {
		if !(translated.row >= 0 && translated.row < len(newView.rows) && func() int {
			var current entry = newView.rows[translated.row]
			if current.kind != lineRow {
				return -1
			}
			if !newView.split || translated.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := newView.rows[translated.row]
		return newView.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !newView.split || translated.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, translatedLine, line,
		"translated line = %#v, want %#v", translatedLine, line,
	)
}

func TestSplitViewWithOnlyDeletionsStartsInLeftPane(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "deleted.go",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{Kind: patch.Deletion, Text: "gone", OldNumber: 1},
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
	testutil.RequireEqual(t, ok, true, "first cursor = %#v, %v", cursor, ok)
	testutil.RequireEqual(t, cursor.pane, left, "first cursor = %#v, %v",
		cursor, ok,
	)

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
	testutil.RequireEqual(t, line.Text, "gone", "first line = %q", line.Text)
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}
	oldView := (func(p patch.Patch, dark bool) *diffView {
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
	})(original, true)
	cursor, _ := func() (diffCursor, bool) {
		var (
			query  string     = "removed two"
			cursor diffCursor = func() diffCursor {
				t.Helper()
				cursor, ok := func() (diffCursor, bool) {
					if cursor, ok := func() (diffCursor, bool) {
						step := int(forward)
						for y := -1 + step; y >= 0 && y < len(oldView.rows); y += step {
							if cursor, ok := func() (diffCursor, bool) {
								cursor := diffCursor{row: y, pane: right}
								return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
									var current entry = oldView.rows[cursor.row]
									if current.kind != lineRow {
										return -1
									}
									if !oldView.split || cursor.pane == right {
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
					for y := -1 + step; y >= 0 && y < len(oldView.rows); y += step {
						if cursor, ok := func() (diffCursor, bool) {
							cursor := diffCursor{row: y, pane: left}
							return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
								var current entry = oldView.rows[cursor.row]
								if current.kind != lineRow {
									return -1
								}
								if !oldView.split || cursor.pane == right {
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
		)
		if query == "" || !(cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
			var current entry = oldView.rows[cursor.row]
			if current.kind != lineRow {
				return -1
			}
			if !oldView.split || cursor.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return diffCursor{}, false
		}
		query = strings.ToLower(query)
		y := cursor.row
		for count := 0; count < len(oldView.rows)-1; count++ {
			y = (y + int(forward) + len(oldView.rows)) % len(oldView.rows)
			if candidate, ok := func() (diffCursor, bool) {
				var query string = query
				if oldView.rows[y].kind != lineRow {
					if strings.Contains(strings.ToLower(ansi.Strip(oldView.rows[y].text)), query) {
						var (
							y    int      = y
							pane diffPane = cursor.pane
						)
						for distance := 1; distance <= len(oldView.rows); distance++ {
							for _, candidateY := range []int{y + int(forward)*distance, y - int(forward)*distance} {
								if candidate, ok := func() (diffCursor, bool) {
									if candidateY < 0 || candidateY >= len(oldView.rows) || oldView.rows[candidateY].file != oldView.rows[y].file {
										return diffCursor{}, false
									}
									if candidate, ok := func() (diffCursor, bool) {
										cursor := diffCursor{row: candidateY, pane: pane}
										return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
											var current entry = oldView.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !oldView.split || cursor.pane == right {
												return current.rightLine
											}
											return current.leftLine
										}() >= 0
									}(); ok {
										return candidate, true
									}
									if oldView.split {
										cursor := diffCursor{row: candidateY, pane: func() diffPane {
											if pane == left {
												return right
											}
											return left
										}()}
										return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
											var current entry = oldView.rows[cursor.row]
											if current.kind != lineRow {
												return -1
											}
											if !oldView.split || cursor.pane == right {
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
				panes := []diffPane{cursor.pane}
				if oldView.split {
					panes = append(panes, func() diffPane {
						if cursor.pane == left {
							return right
						}
						return left
					}())
				}
				for _, candidatePane := range panes {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: candidatePane}
						return cursor, cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
							var current entry = oldView.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !oldView.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(oldView.rows) && func() int {
							var current entry = oldView.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !oldView.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := oldView.rows[candidate.row]
						return oldView.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !oldView.split || candidate.pane == right {
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
	changed := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "first.go", OldPath: "first.go", NewPath: "first.go", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "before", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "removed one", OldNumber: 2}, {Kind: patch.Deletion, Text: "removed two", OldNumber: 3}, {Kind: patch.Addition, Text: "added one", NewNumber: 2}, {Kind: patch.Context, Text: "after", OldNumber: 4, NewNumber: 3}}}}}, {DisplayPath: "second.go", OldPath: "second.go", NewPath: "second.go", Hunks: []patch.Hunk{{Header: "@@ -1 +1 @@", Lines: []patch.Line{{Kind: patch.Addition, Text: "other", NewNumber: 1}}}}}}}
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := (func(p patch.Patch, dark bool) *diffView {
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
	})(changed, true)
	fallback, ok := func() (diffCursor, bool) {
		var target cursorIdentity = func() cursorIdentity {
			if !(cursor.row >= 0 && cursor.row < len(oldView.rows) && func() int {
				var current entry = oldView.rows[cursor.row]
				if current.kind != lineRow {
					return -1
				}
				if !oldView.split || cursor.pane == right {
					return current.rightLine
				}
				return current.leftLine
			}() >= 0) {
				return cursorIdentity{}
			}
			row := oldView.rows[cursor.row]
			file := oldView.patch.Files[row.file]
			hunk := file.Hunks[row.hunk]
			line := hunk.Lines[func() int {
				if row.kind != lineRow {
					return -1
				}
				if !oldView.split || cursor.pane == right {
					return row.rightLine
				}
				return row.leftLine
			}()]
			return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: true}
		}()
		if !target.valid {
			return diffCursor{}, false
		}
		best := cursorMatch{}
		for y := range newView.rows {
			candidate := func() cursorMatch {
				current := newView.rows[y]
				if !(current.hunk >= 0 && func() bool {
					var candidate patch.File = newView.patch.Files[current.file]
					return candidate.OldPath != "" && candidate.OldPath == target.file.OldPath || candidate.NewPath != "" && candidate.NewPath == target.file.NewPath
				}()) {
					return cursorMatch{}
				}
				best := cursorMatch{}
				for _, pane := range [2]diffPane{target.cursor.pane, func() diffPane {
					if target.cursor.pane == left {
						return right
					}
					return left
				}()} {
					candidate, ok := func() (diffCursor, bool) {
						cursor := diffCursor{row: y, pane: pane}
						return cursor, cursor.row >= 0 && cursor.row < len(newView.rows) && func() int {
							var current entry = newView.rows[cursor.row]
							if current.kind != lineRow {
								return -1
							}
							if !newView.split || cursor.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0
					}()
					if !ok {
						continue
					}
					line, _ := func() (patch.Line, bool) {
						if !(candidate.row >= 0 && candidate.row < len(newView.rows) && func() int {
							var current entry = newView.rows[candidate.row]
							if current.kind != lineRow {
								return -1
							}
							if !newView.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}() >= 0) {
							return patch.Line{}, false
						}
						current := newView.rows[candidate.row]
						return newView.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
							if current.kind != lineRow {
								return -1
							}
							if !newView.split || candidate.pane == right {
								return current.rightLine
							}
							return current.leftLine
						}()], true
					}()
					match := cursorMatch{cursor: candidate, rank: func() int {
						if line.Kind != target.line.Kind {
							return 1
						}
						if line.OldNumber == target.line.OldNumber && line.NewNumber == target.line.NewNumber {
							return 3
						}
						if line.Text == target.line.Text {
							return 2
						}
						return 1
					}(), distance: func() int {
						var rowDistance int = y - target.cursor.row
						if line.NewNumber > 0 && target.line.NewNumber > 0 {
							if int(line.NewNumber-target.line.NewNumber) < 0 {
								return -int(line.NewNumber - target.line.NewNumber)
							}
							return int(line.NewNumber - target.line.NewNumber)
						}
						if line.OldNumber > 0 && target.line.OldNumber > 0 {
							if int(line.OldNumber-target.line.OldNumber) < 0 {
								return -int(line.OldNumber - target.line.OldNumber)
							}
							return int(line.OldNumber - target.line.OldNumber)
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
	testutil.RequireEqual(t, ok, true, "%s", "nearby cursor was not found")

	fallbackLine, _ := func() (patch.Line, bool) {
		if !(fallback.row >= 0 && fallback.row < len(newView.rows) && func() int {
			var current entry = newView.rows[fallback.row]
			if current.kind != lineRow {
				return -1
			}
			if !newView.split || fallback.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}() >= 0) {
			return patch.Line{}, false
		}
		current := newView.rows[fallback.row]
		return newView.patch.Files[current.file].Hunks[current.hunk].Lines[func() int {
			if current.kind != lineRow {
				return -1
			}
			if !newView.split || fallback.pane == right {
				return current.rightLine
			}
			return current.leftLine
		}()], true
	}()
	testutil.RequireEqual(t, fallbackLine.Kind, patch.Deletion,
		"fallback kind = %v", fallbackLine.Kind,
	)
}
