package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
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
	old := newUnifiedPatchView(makePatch([]patch.Line{context, target}), true)
	cursor, _ := old.lastCursor()
	identity := identifyCursor(old, cursor)
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

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := twoFileReviewPatch()
	oldView := newUnifiedPatchView(original, true)
	cursor, _ := oldView.searchFromCursor(
		"removed two", mustFirstCursor(t, oldView), forward,
	)
	changed := twoFileReviewPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := newUnifiedPatchView(changed, true)
	fallback, ok := newView.restoreCursor(identifyCursor(oldView, cursor))
	testutil.RequireEqual(t, ok, true, "nearby cursor was not found")

	fallbackLine, _ := newView.lineAtCursor(fallback)
	testutil.RequireEqual(t, fallbackLine.Kind, patch.Deletion,
		fmt.Sprintf("fallback kind = %v", fallbackLine.Kind),
	)
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
			next := newUnifiedPatchView(p, true)
			restored, ok := next.restoreCursor(identity)
			line, _ := next.lineAtCursor(restored)
			testutil.RequireEqual(t, ok, true,
				fmt.Sprintf("restored=%#v, want %#v", line, want),
			)
			testutil.RequireEqual(t, line, want,
				fmt.Sprintf("restored=%#v, want %#v", line, want),
			)
		})
	}
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := twoFileReviewPatch()
	oldView := newUnifiedPatchView(original, true)
	cursor, _ := oldView.searchFromCursor(
		"added one", mustFirstCursor(t, oldView), forward,
	)
	line, _ := oldView.lineAtCursor(cursor)
	changed := twoFileReviewPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := newUnifiedPatchView(changed, true)
	translated, ok := newView.restoreCursor(identifyCursor(oldView, cursor))
	testutil.RequireEqual(t, ok, true, "semantic cursor was not found")
	testutil.RequireEqual(t, translated.row != cursor.row, true,
		"cursor coordinate was reused after rows shifted",
	)

	translatedLine, _ := newView.lineAtCursor(translated)
	testutil.RequireEqual(t, translatedLine, line,
		fmt.Sprintf("translated line = %#v, want %#v", translatedLine, line),
	)
}
