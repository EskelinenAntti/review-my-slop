package frame

import (
	"strings"
	"testing"
)

func TestBodyRowExcludesFrameAndTerminalEdges(t *testing.T) {
	for _, tc := range []struct {
		x, y, width, height, row int
		valid                    bool
	}{
		{0, 1, 80, 10, 0, true}, {79, 7, 80, 10, 6, true},
		{-1, 1, 80, 10, 0, false}, {80, 1, 80, 10, 0, false},
		{0, 0, 80, 10, -1, false}, {0, 8, 80, 10, 7, false},
		{0, 9, 80, 10, 8, false}, {0, 1, 80, 2, 0, false},
	} {
		row, valid := BodyRow(tc.x, tc.y, tc.width, tc.height)
		if row != tc.row || valid != tc.valid {
			t.Fatalf("%+v: got (%d, %v)", tc, row, valid)
		}
	}
}

func TestRenderPadsAndTruncatesBody(t *testing.T) {
	for _, tc := range []struct {
		body   []string
		height int
		want   string
	}{
		{[]string{"one"}, 6, "header\none\n\n\nfooter\n"},
		{[]string{"one", "two", "three"}, 4, "header\none\nfooter\n"},
		{nil, 1, "header\n\nfooter\n"},
	} {
		if got := Render("header", tc.body, "footer", tc.height); got != tc.want {
			t.Fatalf("got %q, want %q", got, tc.want)
		}
	}
	if got := len(strings.Split(Render("", nil, "", 8), "\n")); got != 8 {
		t.Fatalf("rows=%d", got)
	}
}
