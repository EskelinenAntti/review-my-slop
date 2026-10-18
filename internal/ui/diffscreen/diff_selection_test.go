package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSelectionBetweenRejectsInvalidEndpoints(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	valid, _ := v.first()
	for _, invalid := range []diffCursor{{row: -1}, {row: len(v.rows)}, {}} {
		_, ok := v.selectionBetween(invalid, valid)
		testutil.RequireEqual(t, ok, false,
			"accepted first endpoint %#v", invalid)
		_, ok = v.selectionBetween(valid, invalid)
		testutil.RequireEqual(t, ok, false,
			"accepted last endpoint %#v", invalid)
	}
}

func TestSelectionBetweenPreservesEndpointOrder(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, _ := v.first()
	last, _ := v.move(first, forward)
	selection, ok := v.selectionBetween(last, first)
	testutil.RequireEqual(t, ok, true, "reverse selection rejected")
	testutil.RequireEqual(t, selection,
		diffSelection{First: last, Last: first},
		"selection endpoints = %#v", selection)
	lines := v.lines(selection)
	testutil.RequireEqual(t, len(lines), 2, "selected lines = %#v", lines)
	line, _ := v.line(first)
	testutil.RequireEqual(t, lines[0], line,
		"selected lines not in source order: %#v", lines)
}
