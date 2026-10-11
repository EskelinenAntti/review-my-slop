package diffscreen

import (
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := stickyHeaderPatch()
	v := newUnifiedView(p, true)
	viewport := v.newViewport(60, 3)
	viewport.top = 3

	rendered := strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		newline,
	)
	requireEqual(
		t,
		len(rendered) == viewport.Height &&
			strings.Contains(rendered[0], "first.go"),
		true,
		"sticky render = %#v",
		rendered,
	)
	requireEqual(
		t,
		strings.Contains(rendered[1], "first two") &&
			strings.Contains(rendered[2], "first three"),
		true,
		"sticky header covered content rows: %#v",
		rendered,
	)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		newline,
	)
	requireEqual(
		t,
		strings.Count(strings.Join(rendered, newline), fixtureSecondGo), 1,
		"file header was duplicated at its natural position: %#v",
		rendered,
	)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		newline,
	)
	requireEqual(
		t,
		strings.Contains(rendered[0], fixtureSecondGo), true,
		"sticky header did not change with the file: %#v",
		rendered,
	)
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	cursor, _ := v.last()
	viewport := v.keepVisible(v.newViewport(50, 4), cursor)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	line, _ := v.line(cursor)
	requireEqual(
		t,
		cursor.row < viewport.top+v.contentHeight(viewport) &&
			strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))),
		true,
		"last cursor row is hidden by sticky head"+
			"er: viewport=%#v render=%q",
		viewport,
		rendered,
	)
}

func TestSplitTabsDoNotShiftLineNumbersOrDivider(t *testing.T) {
	p := patch.Patch{
		Files: []patch.File{
			{
				DisplayPath: fixtureFile,
				Hunks: []patch.Hunk{
					{
						Header: hunkMarker,
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
	rendered := ansi.Strip(renderOne(v, cursor, wideTestWidth, nil))
	requireEqual(
		t,
		!strings.ContainsRune(rendered, '\t') &&
			strings.Index(rendered, "│") == 59 &&
			lipgloss.Width(rendered) == wideTestWidth,
		true,
		"rendered = %q width=%d",
		rendered,
		lipgloss.Width(rendered),
	)
}

func TestHorizontalScrollKeepsUnifiedGutterFixed(t *testing.T) {
	p := longLinePatch()
	v := newUnifiedView(p, true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(narrowTestWidth, 1)
	viewport = v.keepVisible(viewport, cursor)
	before := ansi.Strip(v.render(viewport, cursor, nil))
	viewport = v.scrollHorizontal(viewport, 4)
	after := ansi.Strip(v.render(viewport, cursor, nil))
	requireEqual(
		t,
		viewport.LeftColumn == 4 &&
			(before[:14] == after[:14] &&
				strings.Contains(after[14:], "efghij")),
		true,
		"before=%q after=%q viewport=%#v",
		before,
		after,
		viewport,
	)
}

func TestHorizontalScrollKeepsSplitGuttersAndDividerFixed(t *testing.T) {
	v := newSideBySideView(longLinePatch(), true)
	cursor := mustFirst(t, v)
	viewport := v.newViewport(wideTestWidth, 1)
	viewport = v.keepVisible(viewport, cursor)
	viewport = v.scrollHorizontal(viewport, 8)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	requireEqual(
		t,
		rendered[63:69] == "    1 " &&
			(rendered[:6] == "    1 " &&
				strings.Index(rendered, "│") == 59),
		true,
		"gutters moved: %q",
		rendered,
	)
	requireEqual(
		t,
		strings.Contains(rendered[6:59], "ghijkl") &&
			strings.Contains(rendered[69:], "ghijkl"),
		true,
		"offset differs: %q",
		rendered,
	)
}

func TestHorizontalScrollStartAndEndClamp(t *testing.T) {
	v := newUnifiedView(longLinePatch(), true)
	viewport := v.newViewport(narrowTestWidth, 1)
	viewport = v.scrollHorizontal(viewport, int(^uint(0)>>1))
	requireEqual(
		t,
		viewport.LeftColumn != 0,
		true,
		fixtureV,
		"end did not scroll",
	)
	viewport = v.scrollHorizontal(viewport, -int(^uint(0)>>1))
	requireEqual(
		t,
		viewport.LeftColumn, 0,
		"start = %d",
		viewport.LeftColumn,
	)
}
