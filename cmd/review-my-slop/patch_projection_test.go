package main

import (
	"slices"
	"testing"
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
		t.Run(
			tc.name,
			func(t *testing.T) {
				p := patchPatch{
					Files: []patchFile{
						{
							DisplayPath: "file.go",
							Hunks: []patchHunk{{
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

func projectedSplitRowTexts(view *diffscreenDiffView) [][2]string {
	var rows [][2]string
	for row := range view.rows {
		if texts, ok := projectedSplitRow(view, row); ok {
			rows = append(rows, texts)
		}
	}
	return rows
}

func projectedSplitRow(view *diffscreenDiffView, row int) ([2]string, bool) {
	if view.rows[row].kind != diffscreenLineRow {
		return [2]string{}, false
	}
	var texts [2]string
	for _, pane := range []diffscreenDiffPane{diffscreenLeft, diffscreenRight} {
		cursor, ok := diffscreenCursorAtRow(view, pane, row)
		if ok {
			_, line, _ := diffscreenPatchLineAtCursor(view, cursor)
			texts[pane] = line.Text
		}
	}
	return texts, true
}

type cursorRestorationCase struct {
	name  string
	lines []patchLine
	want  patchLine
}

func cursorRestorationCases(context patchLine) []cursorRestorationCase {
	line := func(
		kind patchLineKind, text string, number patchLineNumber,
	) patchLine {
		return patchLine{Kind: kind, Text: text, NewNumber: number}
	}
	return []cursorRestorationCase{
		{
			"numbers before text",
			[]patchLine{
				line(patchAddition, "target", 20),
				line(patchAddition, "changed text", 10),
			},
			line(patchAddition, "changed text", 10),
		},
		{
			"text before distance",
			[]patchLine{
				line(patchAddition, "target", 20),
				line(patchAddition, "different", 21),
			},
			line(patchAddition, "target", 20),
		},
		{
			"source distance before kind",
			[]patchLine{line(patchAddition, "different", 20), context},
			context,
		},
		{
			"nearest matching text by source position",
			[]patchLine{
				line(patchAddition, "target", 20),
				line(patchAddition, "target", 21),
			},
			line(patchAddition, "target", 20),
		},
		{
			"equidistant matching text",
			[]patchLine{
				line(patchAddition, "target", 8),
				context,
				line(patchAddition, "target", 12),
			},
			line(patchAddition, "target", 8),
		},
		{
			"nearest remaining line",
			[]patchLine{context, line(patchContext, "nearby", 2)},
			line(patchContext, "nearby", 2),
		},
	}
}

func TestDiffscreenCursorRestorationPrioritizesIdentityBeforeDistance(
	t *testing.T,
) {
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
		return patchPatch{Files: []patchFile{{
			OldPath: "file.go",
			NewPath: "file.go",
			Hunks:   []patchHunk{{Header: "@@", Lines: lines}},
		}}}
	})
	old := diffscreenTestViewWithTheme(
		makePatch([]patchLine{context, target}),
		true,
	)
	cursor, ok := diffscreenCursorAtRow(old, diffscreenRight, len(old.rows)-1)
	requireEqual(t, ok, true, "initial cursor=%#v", cursor)
	identity := diffscreenCursorIdentityAt(old, cursor)
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
	patch patchPatch,
	identity diffscreenCursorIdentity,
	tc cursorRestorationCase,
) {
	t.Helper()
	for _, header := range []string{"@@", "@@ -10,2 +10,3 @@"} {
		t.Run(header, func(t *testing.T) {
			patch.Files[0].Hunks[0].Header = header
			next := diffscreenTestViewWithTheme(patch, true)
			restored, ok := diffscreenTranslateCursorIdentity(next, identity)
			line, lineOK := diffscreenLineForCursor(restored, next)
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
