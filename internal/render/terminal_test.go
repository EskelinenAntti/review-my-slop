package render

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/layout"
	"github.com/eskelinenantti/review-my-slop/internal/navigation"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestHighlightAdaptsToTerminalBackgroundAndKeepsSourceText(t *testing.T) {
	source := "package main\n\n// comment\nfunc answer(value int) string {\n\treturn \"ready\"\n}\n"
	dark := strings.Join(highlight("example.go", source, true), "\n")
	light := strings.Join(highlight("example.go", source, false), "\n")
	if dark == light {
		t.Fatal("light and dark terminal backgrounds use identical highlighting")
	}
	for name, rendered := range map[string]string{"dark": dark, "light": light} {
		if !strings.Contains(rendered, "[38;2;") {
			t.Errorf("%s theme does not use truecolor syntax highlighting: %q", name, rendered)
		}
		if strings.Contains(rendered, "[48;2;") {
			t.Errorf("%s theme overrides terminal background: %q", name, rendered)
		}
		plain := ansi.Strip(rendered)
		if !strings.Contains(plain, `return "ready"`) {
			t.Errorf("%s highlighting corrupted source text: %q", name, plain)
		}
	}
}

func TestRendererRefreshesHighlightCacheWhenThemeChanges(t *testing.T) {
	doc := layout.Build(patch.Patch{Files: []patch.File{{DisplayPath: "file.go", NewPath: "file.go", NewSource: "package main\nfunc main() {}\n", Hunks: []patch.Hunk{{
		Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "func main() {}", OldNumber: 1, NewNumber: 1}},
	}}}}}, layout.Unified)
	row := rowsOfKind(doc, layout.LineRow)[0]
	state := navigation.Snapshot{Viewport: navigation.Viewport{Top: row, Width: 80, Height: 1}}
	renderer := NewRenderer()
	dark := renderer.Render(doc, state, Theme{Dark: true})
	light := renderer.Render(doc, state, Theme{Dark: false})
	if dark == light {
		t.Fatal("renderer reused syntax colors after the terminal theme changed")
	}
}

func TestLicenseHighlightFixture(t *testing.T) {
	lines := highlight("LICENSE", `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`, true)
	if len(lines) != 5 {
		t.Fatalf("highlighted lines = %d, want 5", len(lines))
	}
	plain := ansi.Strip(strings.Join(lines, "\n"))
	if !strings.Contains(plain, `Copyright (c) 2026`) || !strings.Contains(plain, `"AS IS"`) {
		t.Fatalf("highlighted text was corrupted: %q", plain)
	}
}

func TestTerminalHighlightsSyntaxThroughDiffStyling(t *testing.T) {
	doc := layout.Build(patch.Patch{Files: []patch.File{{
		DisplayPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\n", NewSource: "package main\nnew()\n",
		Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
			{Kind: patch.Deletion, Text: "old()", OldNumber: 2},
			{Kind: patch.Addition, Text: "new()", NewNumber: 2},
		}}},
	}}}, layout.Unified)
	lineRows := rowsOfKind(doc, layout.LineRow)
	for _, row := range lineRows {
		output := renderAt(doc, row, layout.Right, 80, 0, Theme{Dark: true}, nil)
		if !strings.Contains(output, "[38;2;") {
			t.Fatalf("syntax highlighting missing: %q", output)
		}
	}
}

func TestTerminalKeepsSplitGuttersDividerAndTabsFixedWhenScrolled(t *testing.T) {
	doc := layout.Build(patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{
		Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "\t\tlong line abcdefghijklmnopqrstuvwxyz", OldNumber: 1, NewNumber: 1}},
	}}}}}, layout.Split)
	row := rowsOfKind(doc, layout.LineRow)[0]
	output := renderAt(doc, row, layout.Left, 120, 8, Theme{Dark: true}, nil)
	plain := ansi.Strip(output)
	if strings.ContainsRune(plain, '\t') || strings.Index(plain, "│") != 59 || lipgloss.Width(plain) != 120 {
		t.Fatalf("rendered = %q width=%d", plain, lipgloss.Width(plain))
	}
	if plain[:6] != "    1 " || plain[63:69] != "    1 " {
		t.Fatalf("gutters moved: %q", plain)
	}
}

func TestTerminalKeepsUnifiedGutterFixedDuringHorizontalScroll(t *testing.T) {
	doc := layout.Build(patch.Patch{Files: []patch.File{{DisplayPath: "long", Hunks: []patch.Hunk{{
		Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}},
	}}}}}, layout.Unified)
	row := rowsOfKind(doc, layout.LineRow)[0]
	before := ansi.Strip(renderAt(doc, row, layout.Right, 37, 0, Theme{Dark: true}, nil))
	after := ansi.Strip(renderAt(doc, row, layout.Right, 37, 4, Theme{Dark: true}, nil))
	if before[:14] != after[:14] || !strings.Contains(after[14:], "efghij") {
		t.Fatalf("before=%q after=%q", before, after)
	}
}

func TestTerminalPreservesCursorAndSelectionStyling(t *testing.T) {
	doc := layout.Build(patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{
		Header: "@@", Lines: []patch.Line{{Kind: patch.Addition, Text: "added", NewNumber: 1}},
	}}}}}, layout.Unified)
	row := rowsOfKind(doc, layout.LineRow)[0]
	cell := layout.Cell{Row: row, Pane: layout.Right}
	cursor := renderAt(doc, row, layout.Right, 80, 0, Theme{Dark: true}, &cell)
	if !strings.Contains(cursor, "\x1b[7m") {
		t.Fatalf("cursor does not use reverse styling: %q", cursor)
	}
	selection := &layout.Selection{First: cell, Last: cell}
	state := navigation.Snapshot{Selection: selection, Viewport: navigation.Viewport{Top: row, Width: 80, Height: 1}}
	selected := Terminal(doc, state, Theme{})
	if !strings.Contains(selected, "48;2;219;234;254") {
		t.Fatalf("selection background missing: %q", selected)
	}
	if strings.Contains(selected, "\x1b[1m") {
		t.Fatalf("selection adds bold styling: %q", selected)
	}
}

func TestDiffMarkersUseTerminalColorsAndCursorFillsWidth(t *testing.T) {
	doc := layout.Build(patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{
		Header: "@@", Lines: []patch.Line{
			{Kind: patch.Addition, Text: "added", NewNumber: 1},
			{Kind: patch.Deletion, Text: "removed", OldNumber: 1},
		},
	}}}}}, layout.Unified)
	rows := rowsOfKind(doc, layout.LineRow)
	added := renderAt(doc, rows[0], layout.Right, 80, 0, Theme{Dark: true}, nil)
	removed := renderAt(doc, rows[1], layout.Right, 80, 0, Theme{Dark: true}, nil)
	if !strings.Contains(added, "\x1b[32m+\x1b[m") || !strings.Contains(removed, "\x1b[31m-\x1b[m") {
		t.Fatalf("added=%q removed=%q", added, removed)
	}
	cell := layout.Cell{Row: rows[0], Pane: layout.Right}
	cursorRow := renderAt(doc, rows[0], layout.Right, 80, 0, Theme{Dark: true}, &cell)
	if width := lipgloss.Width(cursorRow); width != 80 {
		t.Fatalf("cursor row width = %d, want 80", width)
	}
}

func TestTerminalStickyHeaderDoesNotCoverRows(t *testing.T) {
	doc := layout.Build(patch.Patch{Files: []patch.File{
		{DisplayPath: "first.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
			{Kind: patch.Context, Text: "first one", OldNumber: 1, NewNumber: 1},
			{Kind: patch.Context, Text: "first two", OldNumber: 2, NewNumber: 2},
			{Kind: patch.Context, Text: "first three", OldNumber: 3, NewNumber: 3},
		}}}},
		{DisplayPath: "second.go", Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "second one", OldNumber: 1, NewNumber: 1}}}}},
	}}, layout.Unified)
	state := navigation.Snapshot{Viewport: navigation.Viewport{Top: 3, Width: 60, Height: 3}}
	rows := strings.Split(ansi.Strip(Terminal(doc, state, Theme{})), "\n")
	if len(rows) != 3 || !strings.Contains(rows[0], "first.go") || !strings.Contains(rows[1], "first two") || !strings.Contains(rows[2], "first three") {
		t.Fatalf("sticky render = %#v", rows)
	}
}

func TestTerminalCodeRowsHaveExactWidth(t *testing.T) {
	for _, test := range []struct {
		format layout.Format
		width  int
	}{{layout.Unified, 37}, {layout.Split, 120}} {
		doc := layout.Build(patch.Patch{Files: []patch.File{{DisplayPath: "file", Hunks: []patch.Hunk{{
			Header: "@@", Lines: []patch.Line{{Kind: patch.Context, Text: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", OldNumber: 1, NewNumber: 1}},
		}}}}}, test.format)
		row := rowsOfKind(doc, layout.LineRow)[0]
		plain := Terminal(doc, navigation.Snapshot{Viewport: navigation.Viewport{Top: row, Width: test.width, Height: 1}}, Theme{Dark: true})
		if width := lipgloss.Width(plain); width != test.width {
			t.Fatalf("format %d width = %d, want %d", test.format, width, test.width)
		}
	}
}

func TestRenderStyledRowStripsSyntaxBackgroundColors(t *testing.T) {
	value := strings.Join([]string{"\x1b[48;2;255;0;0;38;2;1;2;3mtruecolor", "\x1b[48;5;123;1mindexed", "\x1b[45mstandard", "\x1b[105mbright"}, " ")
	rendered := renderStyledRow(lineStyle(patch.Addition, true), value, 80, false)
	for _, forbidden := range []string{"48;2;255;0;0", "48;5;123", "[45m", "[105m"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("retains %q: %q", forbidden, rendered)
		}
	}
}

func rowsOfKind(doc *layout.Document, kind layout.RowKind) []int {
	var rows []int
	for index := 0; index < doc.RowCount(); index++ {
		if doc.Row(index).Kind == kind {
			rows = append(rows, index)
		}
	}
	return rows
}

func renderAt(doc *layout.Document, row int, pane layout.Pane, width, offset int, theme Theme, cursor *layout.Cell) string {
	return Terminal(doc, navigation.Snapshot{
		Cursor:   cursor,
		Viewport: navigation.Viewport{Top: row, LeftColumn: offset, Width: width, Height: 1},
	}, theme)
}
