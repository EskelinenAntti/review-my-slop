package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderUsesSharedHeadersAndLayoutSpecificCodeRows(t *testing.T) {
	p := testPatch()
	p.Files[0].Metadata = []string{"index 123..456"}
	for _, split := range []bool{false, true} {
		name := "unified"
		if split {
			name = "split"
		}
		t.Run(name, func(t *testing.T) {
			v := newDiffView(p, Options{Dark: true, SideBySide: split})
			viewport := v.newViewport(100, len(v.rows)+2)
			cursor, _ := v.first()
			lines := strings.Split(v.render(viewport, cursor, nil), "\n")
			testutil.RequireEqual(t, len(lines), viewport.height,
				"rendered height = %d, want %d", len(lines), viewport.height)
			for y, row := range v.rows {
				switch row.kind {
				case fileRow:
					expected := renderFileRow(row.text, viewport.width)
					testutil.RequireEqual(t, lines[y], expected,
						"file row %d = %q", y, lines[y])
				case metadataRow:
					expected := metadataStyle.Render("  " + row.text)
					testutil.RequireEqual(t, lines[y], expected,
						"metadata row %d = %q", y, lines[y])
				case hunkRow:
					expected := hunkStyle.Render(row.text)
					testutil.RequireEqual(t, lines[y], expected,
						"hunk row %d = %q", y, lines[y])
				case lineRow:
					hasDivider := strings.Contains(lines[y], splitPaneDivider)
					testutil.RequireEqual(t, hasDivider, split,
						"code row %d has wrong layout: %q", y, lines[y])
				default:
					t.Fatalf("unexpected row kind %v", row.kind)
				}
			}
			for _, line := range lines[len(v.rows):] {
				testutil.RequireEqual(t, line, "", "padding row = %q", line)
			}
		})
	}
}
