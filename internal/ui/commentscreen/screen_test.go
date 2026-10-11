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
	testutil.Requiref(t, ok && selected.ID == "second", "selected=%#v", selected)
	second.Body = "edited"
	v.Update([]comments.Comment{first, second})
	selected, _ = v.Selected()
	testutil.Requiref(t, selected.ID == "second" && selected.Body == "edited", "selected=%#v", selected)
	v.Update([]comments.Comment{first})
	selected, _ = v.Selected()
	testutil.Requiref(t, selected.ID == "first", "deleted selection fallback=%#v", selected)
}

func TestMovementKeepsSelectedCommentVisible(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{ID: fmt.Sprint(index), Body: fmt.Sprintf("comment %d", index)}
	}
	v := New(items)
	v.Resize(80, 7)
	v.Move(100)
	lines := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	testutil.Requiref(t, strings.Contains(strings.Join(lines[1:5], "\n"), ">   comment 9"), "last selection hidden: %q", lines)
	testutil.Requiref(t, strings.Contains(lines[5], "j/k move"), "footer misplaced: %q", lines)
	v.Resize(80, 5)
	testutil.Require(t, strings.Contains(v.Render(nil), "comment 9"), "resize hid selection")
	v.Move(-100)
	selected, _ := v.Selected()
	testutil.Requiref(t, selected.ID == "0", "first selection=%#v", selected)
}

func TestEmptyListAndInputOwnership(t *testing.T) {
	v := New(nil)
	v.Move(100)
	v.Move(-100)
	{
		_, ok := v.Selected()
		testutil.Require(t, !ok, "empty list has a selection")
	}
	testutil.Require(t, strings.Contains(v.Render(nil), "No pending comments."), "empty state missing")
	items := []comments.Comment{{ID: "one", Body: "original"}}
	v.Update(items)
	items[0].Body = "changed outside view"
	selected, _ := v.Selected()
	testutil.Require(t, selected.Body == "original", "view borrowed mutable input slice")
	v.Update(nil)
	{
		_, ok := v.Selected()
		testutil.Require(t, !ok, "cleared list has a selection")
	}
}

func TestRenderErrorKeepsScreenLayout(t *testing.T) {
	v := New([]comments.Comment{{ID: "one", Body: "pending"}})
	v.Resize(25, 7)
	normal := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	failed := strings.Split(ansi.Strip(v.Render(fmt.Errorf("storage failed\n%s", strings.Repeat("x", 60)))), "\n")
	testutil.Require(t, len(failed) == len(normal) && strings.Join(failed[:5], "\n") == strings.Join(normal[:5], "\n"), "error changed the screen body or height")
	testutil.Requiref(t, strings.HasPrefix(failed[5], "storage failed ") && ansi.StringWidth(failed[5]) == 25 && strings.Contains(v.Render(nil), "j/k move"), "error footer=%q", failed[5])
}

func TestClickUsesVisibleRowsWithoutScrollingList(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{ID: fmt.Sprint(index)}
	}
	v := New(items)
	v.Resize(80, 7)
	v.Move(100)
	testutil.Require(t, v.Click(20, 1), "visible comment click rejected")
	selected, _ := v.Selected()
	testutil.Require(t, selected.ID == "6" && v.top == 6, "click did not preserve the visible list position")
	testutil.Require(t, v.Click(20, 4), "last visible row click rejected")
	selected, _ = v.Selected()
	testutil.Require(t, selected.ID == "9", "last visible row mapped to wrong comment")
	for _, point := range [][2]int{{0, 0}, {0, 5}, {0, 6}, {-1, 1}, {80, 1}} {
		testutil.Requiref(t, !v.Click(point[0], point[1]), "invalid point accepted: %v", point)
	}
	v.Update(items[:1])
	testutil.Require(t, !v.Click(20, 2), "blank row click accepted")
	v.Update(nil)
	testutil.Require(t, !v.Click(20, 1), "empty list click accepted")
}
