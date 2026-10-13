package commentscreen

import (
	"fmt"
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
	testutil.RequireEqual(t, ok, true, fmt.Sprintf("selected=%#v", selected))
	testutil.RequireEqual(t, selected.ID, "second",
		fmt.Sprintf("selected=%#v", selected),
	)

	second.Body = "edited"
	v.Update([]comments.Comment{first, second})
	selected, _ = v.Selected()
	testutil.RequireEqual(t, selected.ID, "second",
		fmt.Sprintf("selected=%#v", selected),
	)
	testutil.RequireEqual(t, selected.Body, "edited",
		fmt.Sprintf("selected=%#v", selected),
	)

	v.Update([]comments.Comment{first})
	selected, _ = v.Selected()
	testutil.RequireEqual(t, selected.ID, "first",
		fmt.Sprintf("deleted selection fallback=%#v", selected),
	)
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
		strings.Contains(strings.Join(lines[1:5], "\n"), ">   comment 9"),
		true, fmt.Sprintf("last selection hidden: %q", lines),
	)
	testutil.RequireEqual(t, strings.Contains(lines[5], "j/k move"), true,
		fmt.Sprintf("footer misplaced: %q", lines),
	)

	v.Resize(80, 5)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "comment 9"),
		true, "resize hid selection",
	)

	v.Move(-100)
	selected, _ := v.Selected()
	testutil.RequireEqual(t, selected.ID, "0",
		fmt.Sprintf("first selection=%#v", selected),
	)
}

func TestEmptyListAndInputOwnership(t *testing.T) {
	v := New(nil)
	v.Move(100)
	v.Move(-100)
	if _, ok := v.Selected(); ok {
		t.Fatal("empty list has a selection")
	}
	testutil.RequireEqual(t,
		strings.Contains(v.Render(nil), "No pending comments."), true,
		"empty state missing",
	)

	items := []comments.Comment{{ID: "one", Body: "original"}}
	v.Update(items)
	items[0].Body = "changed outside view"
	selected, _ := v.Selected()
	testutil.RequireEqual(t, selected.Body, "original",
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
	testutil.RequireEqual(t, len(failed), len(normal),
		"error changed the screen body or height",
	)
	testutil.RequireEqual(t, strings.Join(failed[:5], "\n"),
		strings.Join(normal[:5], "\n"),
		"error changed the screen body or height",
	)
	testutil.RequireEqual(t, strings.HasPrefix(failed[5], "storage failed "),
		true, fmt.Sprintf("error footer=%q", failed[5]),
	)
	testutil.RequireEqual(t, ansi.StringWidth(failed[5]), 25,
		fmt.Sprintf("error footer=%q", failed[5]),
	)
	testutil.RequireEqual(t, strings.Contains(v.Render(nil), "j/k move"), true,
		fmt.Sprintf("error footer=%q", failed[5]),
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
	testutil.RequireEqual(t, v.Click(20, 1), true,
		"visible comment click rejected",
	)

	selected, _ := v.Selected()
	testutil.RequireEqual(t, selected.ID, "6",
		"click did not preserve the visible list position",
	)
	testutil.RequireEqual(t, v.top, 6,
		"click did not preserve the visible list position",
	)
	testutil.RequireEqual(t, v.Click(20, 4), true,
		"last visible row click rejected",
	)

	selected, _ = v.Selected()
	testutil.RequireEqual(t, selected.ID, "9",
		"last visible row mapped to wrong comment",
	)

	for _, point := range [][2]int{{0, 0}, {0, 5}, {0, 6}, {-1, 1}, {80, 1}} {
		testutil.RequireEqual(t, !v.Click(point[0], point[1]), true,
			fmt.Sprintf("invalid point accepted: %v", point),
		)
	}
	v.Update(items[:1])
	testutil.RequireEqual(t, !v.Click(20, 2), true, "blank row click accepted")

	v.Update(nil)
	testutil.RequireEqual(t, !v.Click(20, 1), true,
		"empty list click accepted",
	)
}
