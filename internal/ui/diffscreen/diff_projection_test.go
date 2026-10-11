package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestSplitRowsPairOnlyAdjacentChangeBlocks(t *testing.T) {
	for _, tc := range pairingCases() {
		t.Run(
			tc.name,
			func(t *testing.T) {
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
				got := pairedTexts(v)
				requireEqual(
					t,
					slices.Equal(got, tc.want), true,
					"paired source lines=%v, want %v",
					got,
					tc.want,
				)
			},
		)
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
	context := code(patch.Context, fixtureUnchanged, one)
	target := code(patch.Addition, fixtureTarget, ten)

	old := newUnifiedView(linePatch([]patch.Line{context, target}), true)
	cursor, _ := old.last()
	identity := identify(old, cursor)
	for _, tc := range restorationCases(context) {
		t.Run(
			tc.name,
			func(t *testing.T) {
				assertRestoredCursor(t, tc, identity)
			},
		)
	}
}

type pairingCase struct {
	name  string
	lines []patch.Line
	want  [][two]string
}

func pairingCases() []pairingCase {
	code := func(kind patch.LineKind, text string) patch.Line {
		return patch.Line{Kind: kind, Text: text}
	}
	d1, d2 := code(patch.Deletion, fixtureOldA), code(
		patch.Deletion,
		fixtureOldB,
	)
	a1, a2 := code(patch.Addition, fixtureNewA), code(
		patch.Addition,
		fixtureNewB,
	)
	context := code(patch.Context, fixtureUnchanged)

	return []pairingCase{
		{
			"additions",
			[]patch.Line{a1, a2},
			[][two]string{{"", fixtureNewA}, {"", fixtureNewB}},
		},

		{
			"deletions",
			[]patch.Line{d1, d2},
			[][two]string{{fixtureOldA, ""}, {fixtureOldB, ""}},
		},

		{
			"more deletions",
			[]patch.Line{d1, d2, a1},
			[][two]string{{fixtureOldA, fixtureNewA}, {fixtureOldB, ""}},
		},

		{
			"more additions",
			[]patch.Line{d1, a1, a2},
			[][two]string{{fixtureOldA, fixtureNewA}, {"", fixtureNewB}},
		},

		{
			"context separates blocks",
			[]patch.Line{d1, context, a1},
			[][two]string{
				{fixtureOldA, ""},
				{fixtureUnchanged, fixtureUnchanged},
				{"", fixtureNewA},
			},
		},

		{
			"addition before deletion",
			[]patch.Line{a1, d1, a2},
			[][two]string{{"", fixtureNewA}, {fixtureOldA, fixtureNewB}},
		},
	}
}

func pairedTexts(v *diffView) [][two]string {
	var got [][two]string
	for row, entry := range v.rows {
		if entry.kind != lineRow {
			continue
		}
		var texts [two]string
		for _, pane := range []diffPane{left, right} {
			texts[pane] = paneText(v, row, pane)
		}
		got = append(got, texts)
	}
	return got
}

func paneText(v *diffView, row int, pane diffPane) string {
	if cursor, ok := v.cursorAt(row, pane); ok {
		line, _ := v.line(cursor)
		return line.Text
	}
	return ""
}

type restorationCase struct {
	name  string
	lines []patch.Line
	want  patch.Line
}

func restorationCases(context patch.Line) []restorationCase {
	code := numberedLine
	return []restorationCase{
		{
			"numbers before text",
			[]patch.Line{
				code(patch.Addition, fixtureTarget, minimumWidth),
				code(patch.Addition, "changed text", ten),
			},
			code(patch.Addition, "changed text", ten),
		},

		{
			"text before distance",
			[]patch.Line{
				code(patch.Addition, fixtureTarget, minimumWidth),
				code(patch.Addition, "different", fixtureNumber21),
			},
			code(patch.Addition, fixtureTarget, minimumWidth),
		},

		{
			"source distance before kind",
			[]patch.Line{
				code(patch.Addition, "different", minimumWidth),
				context,
			},
			context,
		},

		{
			"nearest matching text by source position",
			[]patch.Line{
				code(patch.Addition, fixtureTarget, minimumWidth),
				code(patch.Addition, fixtureTarget, fixtureNumber21),
			},
			code(patch.Addition, fixtureTarget, minimumWidth),
		},

		{
			"equidistant matching text",
			[]patch.Line{
				code(patch.Addition, fixtureTarget, eight),
				context,
				code(patch.Addition, fixtureTarget, fixtureNumber12),
			},
			code(patch.Addition, fixtureTarget, eight),
		},

		{
			"nearest remaining line",
			[]patch.Line{context, code(patch.Context, "nearby", two)},
			code(patch.Context, "nearby", two),
		},
	}
}

func numberedLine(
	kind patch.LineKind,
	text string,
	number patch.LineNumber,
) patch.Line {
	return patch.Line{Kind: kind, Text: text, NewNumber: number}
}

func assertRestoredCursor(
	t *testing.T,
	tc restorationCase,
	identity cursorIdentity,
) {
	for _, header := range []string{"@@", "@@ -10,2 +10,3 @@"} {
		t.Run(
			header,
			func(t *testing.T) {
				p := linePatch(tc.lines)
				p.Files[0].Hunks[0].Header = header
				next := newUnifiedView(p, true)
				restored, ok := next.findCursor(identity)
				line, _ := next.line(restored)
				requireEqual(
					t,
					ok && line == tc.want, true,
					"restored=%#v, want %#v",
					line,
					tc.want,
				)
			},
		)
	}
}

func linePatch(lines []patch.Line) patch.Patch {
	return patch.Patch{
		Files: []patch.File{
			{
				OldPath: "file.go",
				NewPath: "file.go",
				Hunks:   []patch.Hunk{{Header: "@@", Lines: lines}},
			},
		},
	}
}
