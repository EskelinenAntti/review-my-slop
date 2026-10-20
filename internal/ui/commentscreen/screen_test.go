package commentscreen

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestUpdatePreservesFocusByID(t *testing.T) {
	first := comments.Comment{ID: "first", Body: "first"}
	second := comments.Comment{ID: "second", Body: "second"}
	v := New([]comments.Comment{first, second})
	v.Move(1)
	v.Update([]comments.Comment{second, first})
	selected, ok := v.Selected()
	testutil.RequireEqual(t, ok, true, "selected=%#v", selected)
	testutil.RequireEqual(t, selected.ID, "second", "selected=%#v", selected)

	second.Body = "edited"
	v.Update([]comments.Comment{first, second})
	selected, _ = v.Selected()
	testutil.RequireEqual(t, selected.ID, "second", "selected=%#v", selected)
	testutil.RequireEqual(t, selected.Body, "edited", "selected=%#v", selected)

	v.Update([]comments.Comment{first})
	selected, _ = v.Selected()
	testutil.RequireEqual(t, selected.ID, "first",
		"deleted selection fallback=%#v", selected,
	)
}

func TestUpdateFocusLookupFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name       string
		selectedID string
		items      []comments.Comment
		wantIndex  int
	}{
		{
			name:       "missing ID keeps index when still in bounds",
			selectedID: "removed",
			items: []comments.Comment{
				{ID: "new-first"}, {ID: "new-second"},
			},
			wantIndex: 1,
		},
		{
			name:      "empty ID keeps index",
			items:     []comments.Comment{{}, {ID: "new-second"}},
			wantIndex: 1,
		},
		{
			name:       "duplicate ID selects first match",
			selectedID: "duplicate",
			items: []comments.Comment{
				{ID: "duplicate"}, {ID: "duplicate"},
			},
			wantIndex: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := New([]comments.Comment{{ID: "first"}, {ID: tc.selectedID}})
			v.Move(1)
			v.Update(tc.items)
			index, ok := v.SelectedIndex()
			testutil.RequireEqual(t, ok, true, "%s",
				"updated list has no selection")
			testutil.RequireEqual(t, index, tc.wantIndex,
				"selected index=%d, want %d", index, tc.wantIndex)
		})
	}
}

func TestMovementKeepsSelectedCommentVisible(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{
			ID:   fmt.Sprint(index),
			Body: fmt.Sprintf("comment %d", index),
		}
	}
	v := New(items)
	v.Resize(80, 7)
	v.Move(100)
	lines := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	testutil.RequireEqual(t,
		strings.Contains(strings.Join(lines[1:5], "\n"), ">   comment 9"), true,
		"last selection hidden: %q", lines,
	)
	testutil.RequireEqual(t, strings.Contains(lines[5], "j/k move"), true,
		"footer misplaced: %q", lines,
	)

	v.Resize(80, 5)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "comment 9"), true,
		"%s", "resize hid selection",
	)

	v.Move(-100)
	selected, _ := v.Selected()
	testutil.RequireEqual(t, selected.ID, "0", "first selection=%#v", selected)
}

func TestMovementBoundsExtremeDistances(t *testing.T) {
	v := New([]comments.Comment{{ID: "first"}, {ID: "last"}})
	for range 2 {
		v.Move(math.MaxInt)
		index, ok := v.SelectedIndex()
		testutil.RequireEqual(t, ok, true, "selected index=%d", index)
		testutil.RequireEqual(t, index, 1, "selected index=%d", index)
	}
	v.Move(math.MinInt)
	index, _ := v.SelectedIndex()
	testutil.RequireEqual(t, index, 0, "selected index=%d", index)

	v.Update(nil)
	v.Move(math.MaxInt)
	v.Move(math.MinInt)
	_, ok := v.SelectedIndex()
	testutil.RequireEqual(t, ok, false, "%s", "empty view has a selection")
}

func TestSelectedIndexTracksFocusedComment(t *testing.T) {
	items := []comments.Comment{
		{ID: "first"},
		{ID: "second"},
	}
	v := New(items)
	index, ok := v.SelectedIndex()
	testutil.RequireEqual(t, ok, true, "selected index=%d", index)
	testutil.RequireEqual(t, index, 0, "selected index=%d", index)

	v.Move(1)
	index, ok = v.SelectedIndex()
	testutil.RequireEqual(t, ok, true, "selected index=%d", index)
	testutil.RequireEqual(t, index, 1, "selected index=%d", index)

	v.Update(nil)
	_, ok = v.SelectedIndex()
	testutil.RequireEqual(t, ok, false, "%s", "empty view has a selected index")
}

func TestEmptyListAndInputOwnership(t *testing.T) {
	v := New(nil)
	v.Move(100)
	v.Move(-100)
	if _, ok := v.Selected(); ok {
		t.Fatal("empty list has a selection")
	}
	testutil.RequireEqual(t,
		strings.Contains(v.Render(nil), "No pending comments."), true, "%s",
		"empty state missing",
	)

	items := []comments.Comment{{ID: "one", Body: "original"}}
	v.Update(items)
	items[0].Body = "changed outside view"
	selected, _ := v.Selected()
	testutil.RequireEqual(t, selected.Body, "original", "%s",
		"view borrowed mutable input slice",
	)

	v.Update(nil)
	if _, ok := v.Selected(); ok {
		t.Fatal("cleared list has a selection")
	}
}

func TestRenderErrorKeepsScreenLayout(t *testing.T) {
	v := New([]comments.Comment{{ID: "one", Body: "pending"}})
	v.Resize(25, 7)
	normal := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	failed := strings.Split(
		ansi.Strip(
			v.Render(fmt.Errorf("storage failed\n%s", strings.Repeat("x", 60))),
		),
		"\n",
	)
	testutil.RequireEqual(t, len(failed), len(normal), "%s",
		"error changed the screen body or height",
	)
	testutil.RequireEqual(t, strings.Join(failed[:5], "\n"),
		strings.Join(normal[:5], "\n"), "%s",
		"error changed the screen body or height",
	)
	testutil.RequireEqual(t, strings.HasPrefix(failed[5], "storage failed "),
		true, "error footer=%q", failed[5],
	)
	testutil.RequireEqual(t, ansi.StringWidth(failed[5]), 25, "error footer=%q",
		failed[5],
	)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "j/k move"), true,
		"error footer=%q", failed[5],
	)
}

func TestClickUsesVisibleRowsWithoutScrollingList(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{ID: fmt.Sprint(index)}
	}
	v := New(items)
	v.Resize(80, 7)
	v.Move(100)
	testutil.RequireEqual(t, v.Click(20, 1), true, "%s",
		"visible comment click rejected",
	)

	selected, _ := v.Selected()
	testutil.RequireEqual(t, selected.ID, "6", "%s",
		"click did not preserve the visible list position",
	)
	testutil.RequireEqual(t, v.top, 6, "%s",
		"click did not preserve the visible list position",
	)
	testutil.RequireEqual(t, v.Click(20, 4), true, "%s",
		"last visible row click rejected",
	)

	selected, _ = v.Selected()
	testutil.RequireEqual(t, selected.ID, "9", "%s",
		"last visible row mapped to wrong comment",
	)

	for _, point := range [][2]int{{0, 0}, {0, 5}, {0, 6}, {-1, 1}, {80, 1}} {
		testutil.RequireEqual(t, !v.Click(point[0], point[1]), true,
			"invalid point accepted: %v", point,
		)
	}
	v.Update(items[:1])
	testutil.RequireEqual(t, !v.Click(20, 2), true, "%s",
		"blank row click accepted",
	)

	v.Update(nil)
	testutil.RequireEqual(t, !v.Click(20, 1), true, "%s",
		"empty list click accepted",
	)
}
