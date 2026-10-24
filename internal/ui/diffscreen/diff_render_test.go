package diffscreen

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestRenderCodeLinesScrollOnlyContent(t *testing.T) {
	const text = "abcdefghijklmnopqrstuvwxyz0123456789"
	colored := "\x1b[35m" + text + "\x1b[m"
	p := patch.Patch{Files: []patch.File{{
		DisplayPath: "file",
		Hunks: []patch.Hunk{{
			Header: "@@",
			Lines: []patch.Line{
				{Kind: patch.Deletion, Text: colored, OldNumber: 1},
				{Kind: patch.Addition, Text: colored, NewNumber: 1},
			},
		}},
	}}}
	for _, test := range []struct {
		name   string
		split  bool
		offset int
		want   string
	}{
		{"unified", false, 0, "    1       - " + text[:31]},
		{"unified scrolled", false, 4, "    1       - " + text[4:35]},
		{"split", true, 0, "    1 - " + text[:13] + " │     1 + " + text[:13]},
		{
			"split scrolled", true, 4,
			"    1 " + text[2:17] + " │     1 " + text[2:17],
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: test.split})
			cursor, ok := v.cursorAt(2, left)
			if !ok {
				t.Fatal("missing deletion cursor")
			}
			viewport := v.keepVisible(v.newViewport(45, 1), cursor)
			viewport = v.scrollHorizontal(viewport, test.offset)
			got := ansi.Strip(v.renderRows(viewport, cursor, nil)[0])
			if got != test.want {
				t.Fatalf("rendered row = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRenderRowsPreservesLayout(t *testing.T) {
	for _, test := range []struct {
		name     string
		patch    patch.Patch
		split    bool
		top      int
		expected []string
	}{
		{
			name: "unified", patch: stickyHeaderPatch(),
			expected: []string{"first.go", "@@", "first one"},
		},
		{
			name: "split", patch: stickyHeaderPatch(), split: true,
			expected: []string{"first.go", "@@", "first one"},
		},
		{
			name: "sticky", patch: stickyHeaderPatch(), top: 3,
			expected: []string{"first.go", "first two", "first three"},
		},
		{
			name: "padded", patch: stickyHeaderPatch(),
			expected: []string{
				"first.go", "@@", "first one", "first two", "first three",
				"second.go", "@@", "second one", "", "",
			},
		},
		{name: "empty", expected: []string{"", "", ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			v := newDiffView(test.patch, Options{SideBySide: test.split})
			viewport := v.newViewport(120, len(test.expected))
			viewport.top = test.top
			rows := v.renderRows(viewport, diffCursor{}, nil)
			if len(rows) != len(test.expected) {
				t.Fatalf("got %d rows, want %d", len(rows), len(test.expected))
			}
			for index, fragment := range test.expected {
				row := rows[index]
				if strings.Contains(row, "\n") ||
					!strings.Contains(row, fragment) ||
					fragment == "" && row != "" {
					t.Errorf("row %d = %q, want a single row containing %q",
						index, row, fragment,
					)
				}
			}
			if test.split && !strings.Contains(rows[2], " │ ") {
				t.Errorf("split row has no divider: %q", rows[2])
			}
		})
	}
}

func TestRenderKeepsPresentationStateLocal(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "unified"
		if split {
			name = "split"
		}
		t.Run(name, func(t *testing.T) {
			v := newDiffView(testPatch(), Options{
				SideBySide: split,
				Dark:       true,
			})
			first := mustFirst(t, v)
			last, _ := v.last()
			selection, ok := v.selectionBetween(first, first)
			if !ok {
				t.Fatal("could not select first line")
			}
			for _, state := range []struct {
				name      string
				viewport  diffViewport
				cursor    diffCursor
				selection *diffSelection
			}{
				{"full", v.newViewport(121, 20), first, nil},
				{"selected", v.newViewport(81, 20), last, &selection},
				{
					name:     "scrolled",
					viewport: v.keepVisible(v.newViewport(101, 3), last),
					cursor:   last,
				},
				{
					name: "clamped",
					viewport: diffViewport{
						width: 101, height: 4, top: -1, leftColumn: -1,
					},
					cursor: first,
				},
			} {
				want := v.renderRows(
					state.viewport, state.cursor, state.selection,
				)
				t.Run(state.name, func(t *testing.T) {
					t.Parallel()
					for range 3 {
						got := v.renderRows(
							state.viewport, state.cursor, state.selection,
						)
						if !slices.Equal(got, want) {
							t.Fatalf("render changed:\ngot  %q\nwant %q",
								got, want,
							)
						}
					}
				})
			}
		})
	}
}

func BenchmarkRenderScreen(b *testing.B) {
	for _, split := range []bool{false, true} {
		name := "unified"
		if split {
			name = "split"
		}
		b.Run(name, func(b *testing.B) {
			v := New(longPatch(), Options{SideBySide: split, Dark: true})
			v.Resize(120, 30)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				v.Render(nil)
			}
		})
	}
}
