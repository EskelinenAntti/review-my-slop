package diffscreen

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestLineMatchRank(t *testing.T) {
	target := patch.Line{
		Kind: patch.Context, OldNumber: 10, NewNumber: 20, Text: "target",
	}
	for _, test := range []struct {
		name string
		line patch.Line
		want cursorMatchRank
	}{
		{
			"line numbers before text",
			patch.Line{
				Kind: patch.Context, OldNumber: 10, NewNumber: 20,
				Text: "changed",
			},
			cursorMatchByLineNumbers,
		},
		{
			"text after numbers change",
			patch.Line{
				Kind: patch.Context, OldNumber: 11, NewNumber: 21,
				Text: "target",
			},
			cursorMatchByText,
		},
		{
			"different kind",
			patch.Line{
				Kind: patch.Addition, OldNumber: 10, NewNumber: 20,
				Text: "target",
			},
			cursorMatchInFile,
		},
		{
			"unrelated line",
			patch.Line{
				Kind: patch.Context, OldNumber: 11, NewNumber: 21,
				Text: "other",
			},
			cursorMatchInFile,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			testutil.CheckEqual(t, lineMatchRank(test.line, target),
				test.want, "match rank")
		})
	}
}

func TestLineDistancePrefersSourceNumbers(t *testing.T) {
	for _, test := range []struct {
		name         string
		line, target patch.Line
		rows, want   int
	}{
		{
			"new before old",
			patch.Line{OldNumber: 10, NewNumber: 20},
			patch.Line{OldNumber: 11, NewNumber: 25}, 9, 5,
		},
		{
			"old fallback", patch.Line{OldNumber: 10},
			patch.Line{OldNumber: 13, NewNumber: 20}, 9, 3,
		},
		{
			"row fallback", patch.Line{OldNumber: 10},
			patch.Line{NewNumber: 20}, -9, 9,
		},
		{
			"new absolute distance", patch.Line{NewNumber: 25},
			patch.Line{NewNumber: 20}, 9, 5,
		},
		{
			"old absolute distance", patch.Line{OldNumber: 13},
			patch.Line{OldNumber: 10}, 9, 3,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := lineDistance(test.line, test.target, test.rows)
			testutil.CheckEqual(t, got, test.want, "distance")
		})
	}
}

func TestFindCursorKeepsPreferredPaneOnTies(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	for _, pane := range []diffPane{left, right} {
		for _, exact := range []bool{false, true} {
			name := fmt.Sprintf("pane=%v/exact=%v", pane, exact)
			t.Run(name, func(t *testing.T) {
				cursor, ok := v.cursorAt(2, pane)
				testutil.RequireEqual(t, ok, true, "context cursor")
				target := identify(v, cursor)
				if !exact {
					target.line.OldNumber += 100
					target.line.NewNumber += 100
				}
				got, ok := v.findCursor(target)
				testutil.RequireEqual(t, ok, true, "matching cursor")
				testutil.CheckEqual(t, got, cursor, "preferred pane")
			})
		}
	}
}

func TestFindCursorRanksCandidatesAcrossRows(t *testing.T) {
	target := patch.Line{Kind: patch.Addition, NewNumber: 20, Text: "target"}
	for _, test := range []struct {
		name  string
		lines []patch.Line
		want  int
	}{
		{
			name: "exact numbers outrank matching text",
			lines: []patch.Line{
				{Kind: patch.Addition, NewNumber: 19, Text: "target"},
				{Kind: patch.Addition, NewNumber: 20, Text: "changed"},
			},
			want: 1,
		},
		{
			name: "matching text outranks proximity",
			lines: []patch.Line{
				{Kind: patch.Addition, NewNumber: 19, Text: "other"},
				{Kind: patch.Addition, NewNumber: 30, Text: "target"},
			},
			want: 1,
		},
		{
			name: "closer text match wins",
			lines: []patch.Line{
				{Kind: patch.Addition, NewNumber: 10, Text: "target"},
				{Kind: patch.Addition, NewNumber: 19, Text: "target"},
			},
			want: 1,
		},
		{
			name: "closer fallback wins",
			lines: []patch.Line{
				{Kind: patch.Addition, NewNumber: 10, Text: "other"},
				{Kind: patch.Addition, NewNumber: 19, Text: "changed"},
			},
			want: 1,
		},
		{
			name: "equal distance keeps first row",
			lines: []patch.Line{
				{Kind: patch.Addition, NewNumber: 19, Text: "target"},
				{Kind: patch.Addition, NewNumber: 21, Text: "target"},
			},
			want: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			v := newUnifiedView(patch.Patch{Files: []patch.File{{
				NewPath: "file.go",
				Hunks:   []patch.Hunk{{Lines: test.lines}},
			}}}, true)
			identity := identify(v, mustFirst(t, v))
			identity.line = target
			cursor, ok := v.findCursor(identity)
			testutil.RequireEqual(t, ok, true, "matching cursor")
			line, _ := v.line(cursor)
			testutil.CheckEqual(t, line, test.lines[test.want], "restored line")
		})
	}
}

func TestFindCursorRejectsUnavailableIdentity(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	target := identify(v, mustFirst(t, v))
	missingFile := target
	missingFile.oldPath, missingFile.newPath = "missing.go", "missing.go"
	for _, test := range []struct {
		name   string
		view   *diffView
		target cursorIdentity
	}{
		{"invalid cursor", v, identify(v, diffCursor{row: -1})},
		{"metadata cursor", v, identify(v, diffCursor{row: 0})},
		{"missing file", v, missingFile},
		{"empty view", newUnifiedView(patch.Patch{}, true), target},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, ok := test.view.findCursor(test.target)
			testutil.CheckEqual(t, ok, false, "unexpected matching cursor")
		})
	}
}

func TestFindCursorUsesSemanticIdentityAcrossChangedCoordinates(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("added one", mustFirst(t, oldView), Forward)
	line, _ := oldView.line(cursor)
	changed := testPatch()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newView := newUnifiedView(changed, true)
	translated, ok := newView.findCursor(identify(oldView, cursor))
	testutil.RequireEqual(t, ok, true, "%s", "semantic cursor was not found")
	testutil.RequireEqual(t, translated.row != cursor.row, true, "%s",
		"cursor coordinate was reused after rows shifted",
	)

	translatedLine, _ := newView.line(translated)
	testutil.RequireEqual(t, translatedLine, line,
		"translated line = %#v, want %#v", translatedLine, line,
	)
}

func TestFindCursorFallsBackNearRemovedLine(t *testing.T) {
	original := testPatch()
	oldView := newUnifiedView(original, true)
	cursor, _ := oldView.search("removed two", mustFirst(t, oldView), Forward)
	changed := testPatch()
	changed.Files[0].Hunks[0].Lines = changed.Files[0].Hunks[0].Lines[:2]
	newView := newUnifiedView(changed, true)
	fallback, ok := newView.findCursor(identify(oldView, cursor))
	testutil.RequireEqual(t, ok, true, "%s", "nearby cursor was not found")

	fallbackLine, _ := newView.line(fallback)
	testutil.RequireEqual(t, fallbackLine.Kind, patch.Deletion,
		"fallback kind = %v", fallbackLine.Kind,
	)
}
