package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

func TestUnifiedSelectionBuildsOneSemanticAnchorTraversal(t *testing.T) {
	presentation := newTestPresentation(testChanges(), false)
	first, _ := presentation.First()
	last, _ := presentation.Move(first, Forward)
	last, _ = presentation.Move(last, Forward)
	selection := presentation.BeginSelection(first)
	selection, ok := presentation.ExtendSelection(selection, last)
	if !ok {
		t.Fatal("selection extension failed")
	}
	anchor, err := presentation.anchor(selection)
	if err != nil {
		t.Fatal(err)
	}
	if anchor.FilePath != "first.go" || strings.Join(anchor.QuotedLines, "|") != " before|-removed one|-removed two" {
		t.Fatalf("anchor = %#v", anchor)
	}
	nextFile, _ := presentation.JumpFile(first, Forward)
	if _, ok := presentation.ExtendSelection(selection, nextFile); ok {
		t.Fatal("selection crossed a file")
	}
}

func TestReversedSelectionUsesVisualBoundaries(t *testing.T) {
	presentation := newTestPresentation(testChanges(), false)
	first, _ := presentation.First()
	last, _ := presentation.Move(first, Forward)
	last, _ = presentation.Move(last, Forward)
	selection := Selection{First: last, Last: first}
	anchor, err := presentation.anchor(selection)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(anchor.QuotedLines, "|") != " before|-removed one|-removed two" {
		t.Fatalf("reversed anchor = %q", strings.Join(anchor.QuotedLines, "|"))
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	presentation := newTestPresentation(testChanges(), true)
	first, _ := presentation.First()
	removed, _ := presentation.Move(first, Forward)
	if removed.Pane != Right {
		t.Fatalf("first changed cursor = %#v", removed)
	}
	removed, ok := presentation.SwitchPane(removed, Left)
	if !ok {
		t.Fatal("could not switch to deletion pane")
	}
	line, _ := presentation.line(removed)
	if line.Text != "removed one" {
		t.Fatalf("left line = %q", line.Text)
	}
	added, ok := presentation.SwitchPane(removed, Right)
	if !ok {
		t.Fatal("paired addition missing")
	}
	line, _ = presentation.line(added)
	if line.Text != "added one" {
		t.Fatalf("right line = %q", line.Text)
	}
	rendered := ansi.Strip(presentation.Render(presentation.NewViewport(100, 20), added, nil))
	if !strings.Contains(rendered, "removed one") || !strings.Contains(rendered, "added one") {
		t.Fatalf("paired render missing lines: %q", rendered)
	}
	for _, row := range strings.Split(rendered, "\n") {
		if strings.Contains(row, " │ ") && lipgloss.Width(row) != 100 {
			t.Fatalf("rendered width=%d row=%q", lipgloss.Width(row), row)
		}
	}
}

func TestFindCursorUsesSemanticLineAfterRowsShift(t *testing.T) {
	original := testChanges()
	old := newTestPresentation(original, false)
	cursor, _ := old.Search("added one", mustFirst(t, old), Forward)
	file, _ := old.file(cursor)
	hunk, _ := old.hunk(cursor)
	line, _ := old.line(cursor)
	changed := testChanges()
	changed.Files[0].Metadata = []string{"mode changed", "more metadata"}
	newPresentation := newTestPresentation(changed, false)
	translated, ok := newPresentation.FindCursor(file, hunk, line, cursor.Coordinate, cursor.Pane)
	if !ok || translated.Coordinate == cursor.Coordinate {
		t.Fatalf("translated=%#v ok=%v", translated, ok)
	}
	got, _ := newPresentation.line(translated)
	if got != line {
		t.Fatalf("line=%#v want=%#v", got, line)
	}
}

func TestRawTextIsEscapedOnlyWhenRendered(t *testing.T) {
	changes := diff.ChangeSet{
		Files: []diff.File{{
			NewPath:   "odd\nname.go",
			NewSource: "hello\x1b[2J\n",
			Hunks: []diff.Hunk{{
				Header: "@@",
				Lines:  []diff.Line{{Kind: diff.Addition, Text: "hello\x1b[2J", NewNumber: 1}},
			}},
		}},
	}
	presentation := newTestPresentation(changes, false)
	line, _ := presentation.line(mustFirst(t, presentation))
	if line.Text != "hello\x1b[2J" {
		t.Fatalf("domain line was rewritten: %q", line.Text)
	}
	rendered := ansi.Strip(presentation.Render(presentation.NewViewport(80, 10), mustFirst(t, presentation), nil))
	if !strings.Contains(rendered, `hello\x1b[2J`) || strings.Contains(rendered, "\x1b[2J") {
		t.Fatalf("rendered control was not escaped: %q", rendered)
	}
}

func TestSyntaxHighlightingStaysInsidePresentationStyles(t *testing.T) {
	changes := diff.ChangeSet{
		Files: []diff.File{{
			NewPath:   "main.go",
			OldSource: "package main\nold()\n",
			NewSource: "package main\nnew()\n",
			Hunks: []diff.Hunk{{
				Header: "@@",
				Lines: []diff.Line{
					{Kind: diff.Deletion, Text: "old()", OldNumber: 2},
					{Kind: diff.Addition, Text: "new()", NewNumber: 2},
				},
			}},
		}},
	}
	presentation := newTestPresentation(changes, false)
	rendered := presentation.Render(presentation.NewViewport(80, 10), mustFirst(t, presentation), nil)
	highlighted := strings.Join(highlightSource("main.go", changes.Files[0].NewSource, true), "\n")
	if !strings.Contains(rendered, "[38;2;") || !strings.Contains(highlighted, "[38;2;") || strings.Contains(highlighted, "[48;2;") {
		t.Fatalf("syntax colors rendered=%q highlighted=%q", rendered, highlighted)
	}
}

func FuzzPresentationNavigation(f *testing.F) {
	f.Add([]byte("jjvjl0$"))
	f.Add([]byte("\\x00\\x1b/search"))
	f.Fuzz(func(t *testing.T, operations []byte) {
		changes := diff.ChangeSet{Files: []diff.File{{NewPath: "file.go", NewSource: "one\ntwo\n", Hunks: []diff.Hunk{{Header: "@@", Lines: []diff.Line{
			{Kind: diff.Context, Text: "one", OldNumber: 1, NewNumber: 1},
			{Kind: diff.Addition, Text: "two", NewNumber: 2},
		}}}}}}
		presentation := newTestPresentation(changes, true)
		cursor, ok := presentation.First()
		if !ok {
			return
		}
		viewport := presentation.NewViewport(80, 8)
		for _, operation := range operations {
			switch operation % 8 {
			case 0:
				cursor, _ = presentation.Move(cursor, Forward)
			case 1:
				cursor, _ = presentation.Move(cursor, Backward)
			case 2:
				cursor, _ = presentation.SwitchPane(cursor, Left)
			case 3:
				cursor, _ = presentation.SwitchPane(cursor, Right)
			case 4:
				viewport = presentation.ScrollHorizontal(viewport, int(operation)-128)
			case 5:
				viewport, cursor = presentation.ScrollHalfPage(viewport, cursor, Forward)
			case 6:
				viewport = presentation.KeepVisible(viewport, cursor)
			case 7:
				selection := presentation.BeginSelection(cursor)
				selection, _ = presentation.ExtendSelection(selection, cursor)
				_, _ = presentation.anchor(selection)
			}
			_ = presentation.Render(viewport, cursor, nil)
		}
	})
}

func newTestPresentation(changes diff.ChangeSet, split bool) *presentation {
	presentation := &presentation{}
	presentation.newView(changes, split, true)
	return presentation
}

func mustFirst(t *testing.T, presentation *presentation) Cursor {
	t.Helper()
	cursor, ok := presentation.First()
	if !ok {
		t.Fatal("no first cursor")
	}
	return cursor
}

func testChanges() diff.ChangeSet {
	return diff.ChangeSet{Repository: "/repo", Files: []diff.File{
		{OldPath: "first.go", NewPath: "first.go", OldSource: "before\nremoved one\nremoved two\nafter\n", NewSource: "before\nadded one\nafter\n", Hunks: []diff.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []diff.Line{
			{Kind: diff.Context, Text: "before", OldNumber: 1, NewNumber: 1},
			{Kind: diff.Deletion, Text: "removed one", OldNumber: 2},
			{Kind: diff.Deletion, Text: "removed two", OldNumber: 3},
			{Kind: diff.Addition, Text: "added one", NewNumber: 2},
			{Kind: diff.Context, Text: "after", OldNumber: 4, NewNumber: 3},
		}}}},
		{OldPath: "second.go", NewPath: "second.go", Hunks: []diff.Hunk{{Header: "@@ -1 +1 @@", Lines: []diff.Line{{Kind: diff.Addition, Text: "other", NewNumber: 1}}}}},
	}}
}
