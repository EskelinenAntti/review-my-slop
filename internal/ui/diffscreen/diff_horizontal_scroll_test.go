package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: "file",
				Hunks: []patch.Hunk{
					{
						Header: "@@",
						Lines: []patch.Line{
							{
								Kind:      patch.Context,
								Text:      "\t\tlong line",
								OldNumber: 1,
								NewNumber: 1,
							},
						},
					},
				},
			},
		},
	}
	v := newSideBySideView(p, true)
	cursor := mustFirst(t, v)
	rendered := ansi.Strip(renderOne(v, cursor, 120, nil))
	testutil.RequireEqual(t, !strings.ContainsRune(rendered, '\t'), true,
		fmt.Sprintf("rendered = %q width=%d", rendered,
			lipgloss.Width(rendered),
		),
	)
	testutil.RequireEqual(t, strings.Index(rendered, "│"), 59,
		fmt.Sprintf("rendered = %q width=%d", rendered,
			lipgloss.Width(rendered),
		),
	)
	testutil.RequireEqual(t, lipgloss.Width(rendered), 120,
		fmt.Sprintf("rendered = %q width=%d", rendered,
			lipgloss.Width(rendered),
		),
	)
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := longLinePatch()
	v := newUnifiedView(p, true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(37, 1)
	viewport = v.keepVisible(viewport, cursor)
	before := ansi.Strip(v.render(viewport, cursor, nil))
	viewport = v.scrollHorizontal(viewport, 4)
	after := ansi.Strip(v.render(viewport, cursor, nil))
	testutil.RequireEqual(t, before[:14], after[:14],
		fmt.Sprintf("before=%q after=%q viewport=%#v", before, after, viewport),
	)
	testutil.RequireEqual(t, strings.Contains(after[14:], "efghij"), true,
		fmt.Sprintf("before=%q after=%q viewport=%#v", before, after, viewport),
	)
	testutil.RequireEqual(t, viewport.LeftColumn, 4,
		fmt.Sprintf("before=%q after=%q viewport=%#v", before, after, viewport),
	)
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := newSideBySideView(longLinePatch(), true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(120, 1)
	viewport = v.keepVisible(viewport, cursor)
	viewport = v.scrollHorizontal(viewport, 8)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	testutil.RequireEqual(t, strings.Index(rendered, "│"), 59,
		fmt.Sprintf("gutters moved: %q", rendered),
	)
	testutil.RequireEqual(t, rendered[:6], "    1 ",
		fmt.Sprintf("gutters moved: %q", rendered),
	)
	testutil.RequireEqual(t, rendered[63:69], "    1 ",
		fmt.Sprintf("gutters moved: %q", rendered),
	)
	testutil.RequireEqual(t, strings.Contains(rendered[6:59], "ghijkl"), true,
		fmt.Sprintf("offset differs: %q", rendered),
	)
	testutil.RequireEqual(t, strings.Contains(rendered[69:], "ghijkl"), true,
		fmt.Sprintf("offset differs: %q", rendered),
	)
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := newUnifiedView(longLinePatch(), true)
	viewport := v.newViewport(37, 1)
	viewport = v.scrollHorizontal(viewport, int(^uint(0)>>1))
	testutil.RequireEqual(t, viewport.LeftColumn != 0, true,
		"end did not scroll",
	)

	viewport = v.scrollHorizontal(viewport, -int(^uint(0)>>1))
	testutil.RequireEqual(t, viewport.LeftColumn, 0,
		fmt.Sprintf("start = %d", viewport.LeftColumn),
	)
}
