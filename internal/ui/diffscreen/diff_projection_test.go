package diffscreen

import (
	"slices"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestNonLineEntriesPreserveProjectionData(t *testing.T) {
	p := patch.Patch{Files: []patch.File{{
		DisplayPath: "file.go",
		Metadata:    []string{"file metadata"},
		Hunks:       []patch.Hunk{{Header: "header"}},
	}}}
	for _, split := range []bool{false, true} {
		v := newDiffView(p, Options{SideBySide: split})
		want := []entry{
			{kind: fileRow, file: 0, hunk: -1,
				leftLine: -1, rightLine: -1, text: "file.go"},
			{kind: metadataRow, file: 0, hunk: -1,
				leftLine: -1, rightLine: -1, text: "file metadata"},
			{kind: hunkRow, file: 0, hunk: 0,
				leftLine: -1, rightLine: -1, text: "@@ header"},
		}
		testutil.RequireEqual(t, slices.Equal(v.rows, want), true,
			"split=%v: non-code rows = %#v, want %#v", split, v.rows, want)
		sticky, row, ok := v.rowAtViewportOffset(
			diffViewport{top: 1, height: 3}, 0,
		)
		testutil.RequireEqual(t, ok, true, "sticky header exists")
		testutil.CheckEqual(t, row, -1, "sticky header has no source row")
		testutil.CheckEqual(t, sticky, want[0],
			"sticky and projected headers agree")
		for _, current := range append(v.rows, sticky) {
			for _, pane := range []diffPane{left, right} {
				testutil.CheckEqual(t, v.lineIndex(current, pane), -1,
					"split=%v: non-code row is not selectable in pane %v",
					split, pane)
			}
		}
	}
}

func TestAppendHunkRowsPreservesProjectionData(t *testing.T) {
	oldText := "\x1b[31mold\tone\x1b[0m"
	longText := strings.Repeat("x", 24)
	newText := "\x1b[32mnew\x1b[0m"
	hunk := patch.Hunk{Header: "header", Lines: []patch.Line{
		{Kind: patch.Deletion, OldNumber: 1},
		{Kind: patch.Deletion, OldNumber: 2},
		{Kind: patch.Addition, NewNumber: 1},
	}}
	highlighted := highlightedSources{
		old: []string{oldText, longText},
		new: []string{newText},
	}
	for _, tc := range []struct {
		name  string
		split bool
		want  []entry
	}{
		{
			name: "unified",
			want: []entry{
				{leftLine: 0, rightLine: 0, text: oldText},
				{leftLine: 1, rightLine: 1, text: longText},
				{leftLine: 2, rightLine: 2, text: newText},
			},
		},
		{
			name:  "split",
			split: true,
			want: []entry{
				{leftLine: 0, rightLine: 2, left: oldText, right: newText},
				{leftLine: 1, rightLine: -1, left: longText},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := &diffView{split: tc.split}
			v.appendHunkRows(2, 3, hunk, highlighted)
			header := entry{
				kind: hunkRow, file: 2, hunk: 3,
				leftLine: -1, rightLine: -1, text: "@@ header",
			}
			testutil.RequireEqual(t, v.rows[0], header,
				"header = %#v, want %#v", v.rows[0], header,
			)
			testutil.RequireEqual(t, len(v.rows), len(tc.want)+1,
				"row count = %d, want %d", len(v.rows), len(tc.want)+1,
			)
			for index, want := range tc.want {
				want.kind, want.file, want.hunk = lineRow, 2, 3
				testutil.RequireEqual(t, v.rows[index+1], want,
					"row %d = %#v, want %#v", index, v.rows[index+1], want,
				)
			}
			v.appendHunkRows(2, 4, patch.Hunk{}, highlighted)
			testutil.RequireEqual(t, v.maxLineWidth, 24,
				"maximum line width = %d, want 24", v.maxLineWidth,
			)
		})
	}
}

func TestUnifiedLinePairs(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context},
		{Kind: patch.Deletion},
		{Kind: patch.Addition},
	}
	for length := 0; length <= len(lines); length++ {
		var got [][2]int
		for leftLine, rightLine := range unifiedLinePairs(lines[:length]) {
			got = append(got, [2]int{leftLine, rightLine})
		}
		want := [][2]int{{0, 0}, {1, 1}, {2, 2}}
		if !slices.Equal(got, want[:length]) {
			t.Fatalf("line pairs = %v, want %v", got, want[:length])
		}
	}
}

func TestSplitLinePairs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kinds []patch.LineKind
		want  [][2]int
	}{
		{name: "empty"},
		{
			name:  "split additions only",
			kinds: []patch.LineKind{patch.Addition, patch.Addition},
			want:  [][2]int{{-1, 0}, {-1, 1}},
		},
		{
			name:  "split deletions only",
			kinds: []patch.LineKind{patch.Deletion, patch.Deletion},
			want:  [][2]int{{0, -1}, {1, -1}},
		},
		{
			name: "split more deletions",
			kinds: []patch.LineKind{
				patch.Deletion, patch.Deletion, patch.Addition,
			},
			want: [][2]int{{0, 2}, {1, -1}},
		},
		{
			name: "split more additions",
			kinds: []patch.LineKind{
				patch.Deletion, patch.Addition, patch.Addition,
			},
			want: [][2]int{{0, 1}, {-1, 2}},
		},
		{
			name: "split independent change blocks",
			kinds: []patch.LineKind{
				patch.Addition, patch.Deletion, patch.Addition,
				patch.Context, patch.Deletion,
			},
			want: [][2]int{{-1, 0}, {1, 2}, {3, 3}, {4, -1}},
		},
		{
			name:  "unknown kind retains both panes",
			kinds: []patch.LineKind{patch.LineKind(255)},
			want:  [][2]int{{0, 0}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := make([]patch.Line, len(tc.kinds))
			for index, kind := range tc.kinds {
				lines[index].Kind = kind
			}
			var got [][2]int
			for leftLine, rightLine := range splitLinePairs(lines) {
				got = append(got, [2]int{leftLine, rightLine})
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("line pairs = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLinePairsStopsEarly(t *testing.T) {
	lines := []patch.Line{
		{Kind: patch.Context},
		{Kind: patch.Deletion},
		{Kind: patch.Deletion},
		{Kind: patch.Addition},
		{Kind: patch.Addition},
		{Kind: patch.Context},
	}
	for _, split := range []bool{false, true} {
		for stopAfter := 1; stopAfter <= 3; stopAfter++ {
			count := 0
			pairs := unifiedLinePairs(lines)
			if split {
				pairs = splitLinePairs(lines)
			}
			for range pairs {
				count++
				if count == stopAfter {
					break
				}
			}
			if count != stopAfter {
				t.Fatalf("split=%v: yielded %d pairs, want %d",
					split, count, stopAfter,
				)
			}
		}
	}
}

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
			"unknown line kind advances as a context row",
			[]patch.Line{code(patch.LineKind(255), "unknown")},
			[][2]string{{"unknown", "unknown"}},
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
			v := newSideBySideView(p, true)
			got := projectedTexts(v)
			testutil.RequireEqual(t, slices.Equal(got, tc.want), true,
				"paired source lines=%v, want %v", got, tc.want,
			)
		})
	}
}

func projectedTexts(v *diffView) [][2]string {
	var got [][2]string
	for row, entry := range v.rows {
		if entry.kind != lineRow {
			continue
		}
		texts := projectedRowTexts(v, row)
		got = append(got, texts)
	}
	return got
}

func projectedRowTexts(v *diffView, row int) [2]string {
	var texts [2]string
	for _, pane := range []diffPane{left, right} {
		if cursor, ok := v.cursorAt(row, pane); ok {
			line, _ := v.line(cursor)
			texts[pane] = line.Text
		}
	}
	return texts
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
	makePatch := patchWithLines
	old := newUnifiedView(makePatch([]patch.Line{context, target}), true)
	cursor, _ := old.last()
	identity := identify(old, cursor)
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
			assertCursorRestoration(t, makePatch(tc.lines), identity, tc.want)
		})
	}
}

func assertCursorRestoration(
	t *testing.T,
	p patch.Patch,
	identity cursorIdentity,
	want patch.Line,
) {
	t.Helper()
	for _, header := range []string{"@@", "@@ -10,2 +10,3 @@"} {
		t.Run(header, func(t *testing.T) {
			p.Files[0].Hunks[0].Header = header
			next := newUnifiedView(p, true)
			restored, ok := next.findCursor(identity)
			line, _ := next.line(restored)
			testutil.RequireEqual(t, ok, true, "restored=%#v, want %#v", line,
				want,
			)
			testutil.RequireEqual(t, line, want, "restored=%#v, want %#v", line,
				want,
			)
		})
	}
}

func patchWithLines(lines []patch.Line) patch.Patch {
	return patchWithHunks([]patch.Hunk{{Header: "@@", Lines: lines}})
}

func patchWithHunks(hunks []patch.Hunk) patch.Patch {
	return patch.Patch{Files: []patch.File{
		{OldPath: "file.go", NewPath: "file.go", Hunks: hunks},
	}}
}
