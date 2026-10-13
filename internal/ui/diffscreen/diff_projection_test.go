package diffscreen

import (
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"slices"
	"testing"
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
			v := newSideBySideView(p, true)
			got := projectedTexts(v)
			testutil.RequireEqual(t, slices.Equal(got, tc.want), true,
				fmt.Sprintf("paired source lines=%v, want %v", got, tc.want),
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
