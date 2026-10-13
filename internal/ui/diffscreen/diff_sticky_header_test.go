package diffscreen

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestFileHeaderSticksWithoutCoveringDiffRows(t *testing.T) {
	p := stickyHeaderPatch()
	v := newUnifiedView(p, true)
	viewport := v.newViewport(60, 3)
	viewport.top = 3

	rendered := strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, len(rendered), viewport.Height,
		fmt.Sprintf("sticky render = %#v", rendered),
	)
	testutil.RequireEqual(t, strings.Contains(rendered[0], "first.go"), true,
		fmt.Sprintf("sticky render = %#v", rendered),
	)
	testutil.RequireEqual(t, strings.Contains(rendered[1], "first two"), true,
		fmt.Sprintf("sticky header covered content rows: %#v", rendered),
	)
	testutil.RequireEqual(t, strings.Contains(rendered[2], "first three"),
		true, fmt.Sprintf("sticky header covered content rows: %#v", rendered),
	)

	secondFileRow := 5
	viewport.top = secondFileRow
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, strings.Count(
		strings.Join(rendered, "\n"),
		"second.go",
	), 1,
		fmt.Sprintf("file header was duplicated at its natural position: %#v",
			rendered,
		),
	)

	viewport.top = secondFileRow + 1
	rendered = strings.Split(
		ansi.Strip(v.render(viewport, diffCursor{}, nil)),
		"\n",
	)
	testutil.RequireEqual(t, strings.Contains(rendered[0], "second.go"), true,
		fmt.Sprintf("sticky header did not change with the file: %#v",
			rendered,
		),
	)
}

func stickyHeaderPatch() patch.Patch {
	p := patch.Patch{Files: []patch.File{
		{
			DisplayPath: "first.go",
			Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "first one",
					OldNumber: 1,
					NewNumber: 1,
				},
				{
					Kind:      patch.Context,
					Text:      "first two",
					OldNumber: 2,
					NewNumber: 2,
				},
				{
					Kind:      patch.Context,
					Text:      "first three",
					OldNumber: 3,
					NewNumber: 3,
				},
			}}},
		},
		{
			DisplayPath: "second.go",
			Hunks: []patch.Hunk{{Header: "@@", Lines: []patch.Line{
				{
					Kind:      patch.Context,
					Text:      "second one",
					OldNumber: 1,
					NewNumber: 1,
				},
			}}},
		},
	}}
	return p
}

func TestKeepVisibleAccountsForStickyFileHeader(t *testing.T) {
	v := newUnifiedView(longPatch(), true)
	cursor, _ := v.last()
	viewport := v.keepVisible(v.newViewport(50, 4), cursor)
	rendered := ansi.Strip(v.render(viewport, cursor, nil))
	line, _ := v.line(cursor)
	testutil.RequireEqual(t,
		cursor.row < viewport.top+v.contentHeight(viewport), true,
		fmt.Sprintf("cursor hidden by sticky header: viewport=%#v render=%q",
			viewport, rendered,
		),
	)
	testutil.RequireEqual(t,
		strings.Contains(rendered, strconv.Itoa(int(line.NewNumber))), true,
		fmt.Sprintf("cursor hidden by sticky header: viewport=%#v render=%q",
			viewport, rendered,
		),
	)
}
