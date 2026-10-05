package diffscreen

import (
	"slices"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestSplitRowsPairOnlyAdjacentChangeBlocks(t *testing.T) {
	code := func(kind patch.LineKind, text string) patch.Line {
		return patch.Line{Kind: kind, Text: text}
	}
	d1, d2 := code(patch.Deletion, "old-a"), code(patch.Deletion, "old-b")
	a1, a2 := code(patch.Addition, "new-a"), code(patch.Addition, "new-b")
	context := code(patch.Context, "unchanged")
	for _, tc := range []struct {
		name  string
		lines []patch.Line
		want  [][2]string
	}{
		{"additions", []patch.Line{a1, a2}, [][2]string{{"", "new-a"}, {"", "new-b"}}},
		{"deletions", []patch.Line{d1, d2}, [][2]string{{"old-a", ""}, {"old-b", ""}}},
		{"more deletions", []patch.Line{d1, d2, a1}, [][2]string{{"old-a", "new-a"}, {"old-b", ""}}},
		{"more additions", []patch.Line{d1, a1, a2}, [][2]string{{"old-a", "new-a"}, {"", "new-b"}}},
		{"context separates blocks", []patch.Line{d1, context, a1}, [][2]string{{"old-a", ""}, {"unchanged", "unchanged"}, {"", "new-a"}}},
		{"addition before deletion", []patch.Line{a1, d1, a2}, [][2]string{{"", "new-a"}, {"old-a", "new-b"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := patch.Patch{Files: []patch.File{{DisplayPath: "file.go", Hunks: []patch.Hunk{{Header: "@@", Lines: tc.lines}}}}}
			v := newSideBySideView(p, true)
			var got [][2]string
			for row, entry := range v.rows {
				if entry.kind != lineRow {
					continue
				}
				var texts [2]string
				for _, pane := range []diffPane{left, right} {
					if cursor, ok := v.cursorAt(row, pane); ok {
						line, _ := v.line(cursor)
						texts[pane] = line.Text
					}
				}
				got = append(got, texts)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("paired source lines=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestCursorRestorationPrioritizesIdentityBeforeDistance(t *testing.T) {
	code := func(kind patch.LineKind, text string, number patch.LineNumber) patch.Line {
		return patch.Line{Kind: kind, Text: text, NewNumber: number}
	}
	context := code(patch.Context, "unchanged", 1)
	target := code(patch.Addition, "target", 10)
	makePatch := func(lines []patch.Line) patch.Patch {
		return patch.Patch{Files: []patch.File{{OldPath: "file.go", NewPath: "file.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
	}
	old := newUnifiedView(makePatch([]patch.Line{context, target}), true)
	cursor, _ := old.last()
	identity := identify(old, cursor)
	for _, tc := range []struct {
		name  string
		lines []patch.Line
		want  patch.Line
	}{
		{"numbers before text", []patch.Line{code(patch.Addition, "target", 20), code(patch.Addition, "changed text", 10)}, code(patch.Addition, "changed text", 10)},
		{"text before distance", []patch.Line{code(patch.Addition, "target", 20), code(patch.Addition, "different", 21)}, code(patch.Addition, "target", 20)},
		{"source distance before kind", []patch.Line{code(patch.Addition, "different", 20), context}, context},
		{"nearest matching text by source position", []patch.Line{code(patch.Addition, "target", 20), code(patch.Addition, "target", 21)}, code(patch.Addition, "target", 20)},
		{"equidistant matching text", []patch.Line{code(patch.Addition, "target", 8), context, code(patch.Addition, "target", 12)}, code(patch.Addition, "target", 8)},
		{"nearest remaining line", []patch.Line{context, code(patch.Context, "nearby", 2)}, code(patch.Context, "nearby", 2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, header := range []string{"@@", "@@ -10,2 +10,3 @@"} {
				t.Run(header, func(t *testing.T) {
					p := makePatch(tc.lines)
					p.Files[0].Hunks[0].Header = header
					next := newUnifiedView(p, true)
					restored, ok := next.findCursor(identity)
					line, _ := next.line(restored)
					if !ok || line != tc.want {
						t.Fatalf("restored=%#v, want %#v", line, tc.want)
					}
				})
			}
		})
	}
}
