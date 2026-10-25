package diffscreen

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSourceLineResolutionForRenderingAndIdentity(t *testing.T) {
	context := patch.Line{
		Kind: patch.Context, OldNumber: 1, NewNumber: 1, Text: "context",
	}
	removed := patch.Line{Kind: patch.Deletion, OldNumber: 2, Text: "removed"}
	unpaired := patch.Line{Kind: patch.Deletion, OldNumber: 3, Text: "unpaired"}
	added := patch.Line{Kind: patch.Addition, NewNumber: 2, Text: "added"}
	later := patch.Line{Kind: patch.Addition, NewNumber: 20, Text: "later hunk"}
	p := patch.Patch{Files: []patch.File{{
		OldPath: "file.go", NewPath: "file.go", DisplayPath: "file.go",
		Hunks: []patch.Hunk{
			{
				Header: "first",
				Lines:  []patch.Line{context, removed, unpaired, added},
			},
			{Header: "second", Lines: []patch.Line{later}},
		},
	}}}
	tests := []struct {
		name   string
		split  bool
		row    int
		pane   diffPane
		want   patch.Line
		header string
	}{
		{
			name: "unified left", row: 3, pane: left,
			want: removed, header: "first",
		},
		{
			name: "unified right", row: 5, pane: right,
			want: added, header: "first",
		},
		{
			name: "unified later hunk", row: 7, pane: left,
			want: later, header: "second",
		},
		{
			name: "split context left", split: true, row: 2, pane: left,
			want: context, header: "first",
		},
		{
			name: "split context right", split: true, row: 2, pane: right,
			want: context, header: "first",
		},
		{
			name: "split deletion", split: true, row: 3, pane: left,
			want: removed, header: "first",
		},
		{
			name: "split addition", split: true, row: 3, pane: right,
			want: added, header: "first",
		},
		{
			name: "split unpaired", split: true, row: 4, pane: left,
			want: unpaired, header: "first",
		},
		{name: "split empty pane", split: true, row: 4, pane: right},
		{
			name: "split later hunk", split: true, row: 6, pane: right,
			want: later, header: "second",
		},
		{name: "file header", row: 0, pane: right},
		{name: "hunk header", row: 1, pane: right},
		{name: "negative row", row: -1, pane: right},
		{name: "past end", row: 99, pane: right},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newDiffView(p, Options{SideBySide: tt.split})
			cursor := diffCursor{row: tt.row, pane: tt.pane}
			assertSourceLineConsumers(t, v, cursor, tt.want, tt.header)
		})
	}
}

func assertSourceLineConsumers(
	t *testing.T,
	v *diffView,
	cursor diffCursor,
	want patch.Line,
	header string,
) {
	t.Helper()
	line, ok := v.line(cursor)
	wantValid := header != ""
	testutil.RequireEqual(t, ok, wantValid, "line validity")
	testutil.RequireEqual(t, line, want, "resolved source line")
	identity := identify(v, cursor)
	testutil.RequireEqual(t, identity.valid, wantValid, "identity validity")
	testutil.RequireEqual(t, identity.line, want, "identity source line")
	testutil.RequireEqual(t, identity.hunkHeader, header, "identity hunk")
	if cursor.row < 0 || cursor.row >= len(v.rows) ||
		v.rows[cursor.row].kind != lineRow {
		return
	}
	const width = 40
	renderer := diffRenderer{
		diffView: v,
		viewport: v.newViewport(width, 10),
		cursor:   cursor,
	}
	var rendered string
	if v.split {
		rendered = renderer.renderPane(
			v.rows[cursor.row], cursor.row, cursor.pane, width,
		)
	} else {
		rendered = renderer.renderUnifiedLine(
			v.rows[cursor.row], cursor.row, width,
		)
	}
	if !wantValid {
		testutil.RequireEqual(t, rendered, strings.Repeat(" ", width),
			"empty pane")
		return
	}
	testutil.RequireEqual(t,
		strings.Contains(ansi.Strip(rendered), want.Text), true,
		"rendered source line: %q", rendered,
	)
}
