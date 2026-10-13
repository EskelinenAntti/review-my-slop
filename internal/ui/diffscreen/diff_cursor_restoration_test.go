package diffscreen

import (
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"testing"
)

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
			testutil.RequireEqual(t, ok, true,
				fmt.Sprintf("restored=%#v, want %#v", line, want),
			)
			testutil.RequireEqual(t, line, want,
				fmt.Sprintf("restored=%#v, want %#v", line, want),
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
