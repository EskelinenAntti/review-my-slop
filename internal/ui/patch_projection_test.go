package ui

import (
	"slices"
	"testing"

	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestDiffscreenSplitRowsPairOnlyAdjacentChangeBlocks(t *testing.T) {
	code := func(kind patches.LineKind, text string) patches.Line {
		return patches.Line{Kind: kind, Text: text}
	}
	d1, d2 := code(patches.Deletion, "old-a"), code(patches.Deletion, "old-b")
	a1, a2 := code(patches.Addition, "new-a"), code(patches.Addition, "new-b")
	context := code(patches.Context, "unchanged")
	for _, tc := range []struct {
		name  string
		lines []patches.Line
		want  [][2]string
	}{
		{
			"additions",
			[]patches.Line{a1, a2},
			[][2]string{{"", "new-a"}, {"", "new-b"}},
		},
		{
			"deletions",
			[]patches.Line{d1, d2},
			[][2]string{{"old-a", ""}, {"old-b", ""}},
		},
		{
			"more deletions",
			[]patches.Line{d1, d2, a1},
			[][2]string{{"old-a", "new-a"}, {"old-b", ""}},
		},
		{
			"more additions",
			[]patches.Line{d1, a1, a2},
			[][2]string{{"old-a", "new-a"}, {"", "new-b"}},
		},
		{
			"context separates blocks",
			[]patches.Line{d1, context, a1},
			[][2]string{
				{"old-a", ""},
				{"unchanged", "unchanged"},
				{"", "new-a"},
			},
		},
		{
			"addition before deletion",
			[]patches.Line{a1, d1, a2},
			[][2]string{{"", "new-a"}, {"old-a", "new-b"}},
		},
	} {
		t.Run(
			tc.name,
			func(t *testing.T) {
				p := patches.Patch{
					Files: []patches.File{
						{
							DisplayPath: "file.go",
							Hunks: []patches.Hunk{{
								Header: "@@", Lines: tc.lines,
							}},
						},
					},
				}
				v := diffscreenTestSplitViewWithTheme(p, true)
				got := projectedSplitRowTexts(v)
				requireEqual(t, slices.Equal(got, tc.want), true,
					"paired source lines=%v, want %v", got, tc.want,
				)
			},
		)
	}
}

func projectedSplitRowTexts(view *diffView) [][2]string {
	var rows [][2]string
	for row := range view.rows {
		if texts, ok := projectedSplitRow(view, row); ok {
			rows = append(rows, texts)
		}
	}
	return rows
}

func projectedSplitRow(view *diffView, row int) ([2]string, bool) {
	if view.rows[row].kind != lineRow {
		return [2]string{}, false
	}
	var texts [2]string
	for _, pane := range []diffPane{left, right} {
		cursor, ok := cursorAtRow(view, pane, row)
		if ok {
			_, line, _ := patchLineAtCursor(view, cursor)
			texts[pane] = line.Text
		}
	}
	return texts, true
}

type cursorRestorationCase struct {
	name  string
	lines []patches.Line
	want  patches.Line
}

func cursorRestorationCases(context patches.Line) []cursorRestorationCase {
	line := func(
		kind patches.LineKind, text string, number patches.LineNumber,
	) patches.Line {
		return patches.Line{Kind: kind, Text: text, NewNumber: number}
	}
	return []cursorRestorationCase{
		{
			"numbers before text",
			[]patches.Line{
				line(patches.Addition, "target", 20),
				line(patches.Addition, "changed text", 10),
			},
			line(patches.Addition, "changed text", 10),
		},
		{
			"text before distance",
			[]patches.Line{
				line(patches.Addition, "target", 20),
				line(patches.Addition, "different", 21),
			},
			line(patches.Addition, "target", 20),
		},
		{
			"source distance before kind",
			[]patches.Line{line(patches.Addition, "different", 20), context},
			context,
		},
		{
			"nearest matching text by source position",
			[]patches.Line{
				line(patches.Addition, "target", 20),
				line(patches.Addition, "target", 21),
			},
			line(patches.Addition, "target", 20),
		},
		{
			"equidistant matching text",
			[]patches.Line{
				line(patches.Addition, "target", 8),
				context,
				line(patches.Addition, "target", 12),
			},
			line(patches.Addition, "target", 8),
		},
		{
			"nearest remaining line",
			[]patches.Line{context, line(patches.Context, "nearby", 2)},
			line(patches.Context, "nearby", 2),
		},
	}
}

func TestDiffscreenCursorRestorationPrioritizesIdentityBeforeDistance(
	t *testing.T,
) {
	code := func(
		kind patches.LineKind,
		text string,
		number patches.LineNumber,
	) patches.Line {
		return patches.Line{Kind: kind, Text: text, NewNumber: number}
	}
	context := code(patches.Context, "unchanged", 1)
	target := code(patches.Addition, "target", 10)
	makePatch := (func(lines []patches.Line) patches.Patch {
		return patches.Patch{Files: []patches.File{{
			OldPath: "file.go",
			NewPath: "file.go",
			Hunks:   []patches.Hunk{{Header: "@@", Lines: lines}},
		}}}
	})
	old := diffscreenTestViewWithTheme(
		makePatch([]patches.Line{context, target}),
		true,
	)
	cursor, ok := cursorAtRow(old, right, len(old.rows)-1)
	requireEqual(t, ok, true, "initial cursor=%#v", cursor)
	identity := cursorIdentityAt(old, cursor)
	for _, tc := range cursorRestorationCases(context) {
		t.Run(
			tc.name,
			func(t *testing.T) {
				assertCursorRestorationCase(
					t, makePatch(tc.lines), identity, tc,
				)
			},
		)
	}
}

func assertCursorRestorationCase(
	t *testing.T,
	patch patches.Patch,
	identity cursorIdentity,
	tc cursorRestorationCase,
) {
	t.Helper()
	for _, header := range []string{"@@", "@@ -10,2 +10,3 @@"} {
		t.Run(header, func(t *testing.T) {
			patch.Files[0].Hunks[0].Header = header
			next := diffscreenTestViewWithTheme(patch, true)
			restored, ok := findCursorMatch(next, identity)
			line, lineOK := lineForCursor(restored, next)
			requireEqual(t, ok, true, "restored cursor=%#v", restored)
			requireEqual(t, lineOK, true, "restored=%#v, want %#v",
				line, tc.want,
			)
			requireEqual(t, line, tc.want, "restored=%#v, want %#v",
				line, tc.want,
			)
		})
	}
}
