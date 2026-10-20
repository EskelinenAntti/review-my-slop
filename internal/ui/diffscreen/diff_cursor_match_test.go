package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestLineDistanceFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name        string
		line        patch.Line
		target      patch.Line
		rowDistance int
		want        int
	}{
		{
			name:        "new numbers take precedence",
			line:        patch.Line{OldNumber: 10, NewNumber: 20},
			target:      patch.Line{OldNumber: 11, NewNumber: 25},
			rowDistance: 1, want: 5,
		},
		{
			name:        "missing new candidate uses old numbers",
			line:        patch.Line{OldNumber: 10},
			target:      patch.Line{OldNumber: 7, NewNumber: 25},
			rowDistance: 1, want: 3,
		},
		{
			name:        "missing new target uses old numbers",
			line:        patch.Line{OldNumber: 7, NewNumber: 20},
			target:      patch.Line{OldNumber: 10},
			rowDistance: 1, want: 3,
		},
		{
			name:        "disjoint sides use row distance",
			line:        patch.Line{OldNumber: 10},
			target:      patch.Line{NewNumber: 20},
			rowDistance: -4, want: 4,
		},
		{
			name:        "missing numbers use row distance",
			rowDistance: 4, want: 4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := lineDistance(tc.line, tc.target, tc.rowDistance)
			testutil.RequireEqual(t, got, tc.want, "line distance")
		})
	}
}

func TestFindCursorRanking(t *testing.T) {
	target := patch.Line{
		Kind: patch.Context, OldNumber: 10, NewNumber: 10, Text: "target",
	}
	for _, tc := range []struct {
		name  string
		lines []patch.Line
		want  int
	}{
		{"line numbers beat text", []patch.Line{
			{Kind: patch.Context, OldNumber: 9, NewNumber: 9, Text: "target"},
			{Kind: patch.Context, OldNumber: 10, NewNumber: 10, Text: "other"},
		}, 1},
		{"text beats proximity", []patch.Line{
			{Kind: patch.Context, OldNumber: 9, NewNumber: 9, Text: "other"},
			{Kind: patch.Context, OldNumber: 20, NewNumber: 20, Text: "target"},
		}, 1},
		{"closer text wins", []patch.Line{
			{Kind: patch.Context, OldNumber: 20, NewNumber: 20, Text: "target"},
			{Kind: patch.Context, OldNumber: 9, NewNumber: 9, Text: "target"},
		}, 1},
		{"equal matches retain first row", []patch.Line{
			{Kind: patch.Context, OldNumber: 9, NewNumber: 9, Text: "target"},
			{Kind: patch.Context, OldNumber: 11, NewNumber: 11, Text: "target"},
		}, 0},
		{"fallback uses source proximity", []patch.Line{
			{Kind: patch.Context, OldNumber: 20, NewNumber: 20, Text: "far"},
			{Kind: patch.Context, OldNumber: 9, NewNumber: 9, Text: "near"},
		}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := newDiffView(patch.Patch{Files: []patch.File{{
				OldPath: "file.go", NewPath: "file.go",
				Hunks: []patch.Hunk{{Lines: tc.lines}},
			}}}, Options{SideBySide: true})
			for _, pane := range []diffPane{left, right} {
				cursor, ok := v.findCursor(cursorIdentity{
					oldPath: "file.go", newPath: "file.go", line: target,
					cursor: diffCursor{pane: pane}, valid: true,
				})
				testutil.RequireEqual(t, ok, true, "cursor found")
				want := diffCursor{row: tc.want + 2, pane: pane}
				testutil.RequireEqual(t, cursor, want,
					"best source match retains preferred pane")
			}
		})
	}
}

func TestFindCursorFileFiltering(t *testing.T) {
	line := patch.Line{
		Kind: patch.Context, OldNumber: 10, NewNumber: 10, Text: "target",
	}
	v := newDiffView(patch.Patch{Files: []patch.File{
		{OldPath: "other.go", NewPath: "other.go",
			Hunks: []patch.Hunk{{Lines: []patch.Line{line}}}},
		{OldPath: "old.go", NewPath: "new.go", Metadata: []string{"target"},
			Hunks: []patch.Hunk{{Header: "target", Lines: []patch.Line{line}}}},
		{OldPath: "metadata.go", NewPath: "metadata.go",
			Metadata: []string{"target"}},
	}}, Options{SideBySide: true})
	for _, tc := range []struct {
		name    string
		oldPath string
		newPath string
		valid   bool
		want    bool
	}{
		{"old path matches renamed file", "old.go", "renamed.go", true, true},
		{"new path matches renamed file", "previous.go", "new.go", true, true},
		{"missing file", "missing.go", "missing.go", true, false},
		{"metadata only", "metadata.go", "metadata.go", true, false},
		{"empty paths", "", "", true, false},
		{"invalid identity", "old.go", "new.go", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cursor, ok := v.findCursor(cursorIdentity{
				oldPath: tc.oldPath, newPath: tc.newPath, line: line,
				cursor: diffCursor{pane: right}, valid: tc.valid,
			})
			testutil.RequireEqual(t, ok, tc.want, "cursor found")
			if ok {
				file, _ := v.file(cursor)
				testutil.RequireEqual(t, file.NewPath, "new.go",
					"matching file")
				got, _ := v.line(cursor)
				testutil.RequireEqual(t, got, line, "matching source line")
			}
		})
	}
}

func TestCursorIdentitySameHunk(t *testing.T) {
	target := cursorIdentity{
		oldPath: "old.go", newPath: "new.go",
		hunkHeader: "@@ -1,2 +1,3 @@", valid: true,
	}
	for _, tc := range []struct {
		name  string
		other cursorIdentity
		want  bool
	}{
		{"identical", target, true},
		{"invalid", cursorIdentity{}, false},
		{"invalid with matching source", cursorIdentity{
			oldPath: target.oldPath, newPath: target.newPath,
			hunkHeader: target.hunkHeader,
		}, false},
		{"old path matches", cursorIdentity{
			oldPath: target.oldPath, newPath: "renamed.go",
			hunkHeader: target.hunkHeader, valid: true,
		}, true},
		{"new path matches", cursorIdentity{
			oldPath: "previous.go", newPath: target.newPath,
			hunkHeader: target.hunkHeader, valid: true,
		}, true},
		{"different file", cursorIdentity{
			oldPath: "other.go", newPath: "other.go",
			hunkHeader: target.hunkHeader, valid: true,
		}, false},
		{"different hunk", cursorIdentity{
			oldPath: target.oldPath, newPath: target.newPath,
			hunkHeader: "@@ -10,2 +10,3 @@", valid: true,
		}, false},
		{"empty paths", cursorIdentity{
			hunkHeader: target.hunkHeader, valid: true,
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.RequireEqual(t, target.sameHunk(tc.other), tc.want,
				"matching hunk")
			testutil.RequireEqual(t, tc.other.sameHunk(target), tc.want,
				"matching hunk is symmetric")
		})
	}
}
