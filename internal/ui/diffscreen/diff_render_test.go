package diffscreen

import (
	"slices"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

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
