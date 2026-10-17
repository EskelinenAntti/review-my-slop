package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestCommentscreenUpdatePreservesFocusByID(t *testing.T) {
	first := feedback.Comment{ID: "first", Body: "first"}
	second := feedback.Comment{ID: "second", Body: "second"}
	v := newTestCommentScreen([]feedback.Comment{first, second})
	v.Move(1)
	v.Update([]feedback.Comment{second, first})
	selected, ok := v.Selected()
	requireEqual(t, ok, true, "selected=%#v", selected)
	requireEqual(t, selected.ID, "second", "selected=%#v", selected)

	second.Body = "edited"
	v.Update([]feedback.Comment{first, second})
	selected, _ = v.Selected()
	requireEqual(t, selected.ID, "second", "selected=%#v", selected)
	requireEqual(t, selected.Body, "edited", "selected=%#v", selected)

	v.Update([]feedback.Comment{first})
	selected, _ = v.Selected()
	requireEqual(t, selected.ID, "first",
		"deleted selection fallback=%#v", selected,
	)
}

func TestCommentscreenMovementKeepsSelectedCommentVisible(t *testing.T) {
	items := make([]feedback.Comment, 10)
	for index := range items {
		items[index] = feedback.Comment{
			ID:   fmt.Sprint(index),
			Body: fmt.Sprintf("comment %d", index),
		}
	}
	v := newTestCommentScreen(items)
	v.Resize(80, 7)
	v.Move(100)
	lines := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	requireEqual(t,
		strings.Contains(strings.Join(lines[1:5], "\n"), ">   comment 9"), true,
		"last selection hidden: %q", lines,
	)
	requireEqual(t, strings.Contains(lines[5], "j/k move"), true,
		"footer misplaced: %q", lines,
	)

	v.Resize(80, 5)
	requireEqual(t, strings.Contains(v.Render(nil), "comment 9"), true,
		"%s", "resize hid selection",
	)

	v.Move(-100)
	selected, _ := v.Selected()
	requireEqual(t, selected.ID, "0", "first selection=%#v", selected)
}

func TestCommentscreenEmptyListAndInputOwnership(t *testing.T) {
	v := newTestCommentScreen(nil)
	v.Move(100)
	v.Move(-100)
	if _, ok := v.Selected(); ok {
		t.Fatal("empty list has a selection")
	}
	requireEqual(t,
		strings.Contains(v.Render(nil), "No pending comments."), true, "%s",
		"empty state missing",
	)

	items := []feedback.Comment{{ID: "one", Body: "original"}}
	v.Update(items)
	items[0].Body = "changed outside view"
	selected, _ := v.Selected()
	requireEqual(t, selected.Body, "original", "%s",
		"view borrowed mutable input slice",
	)

	v.Update(nil)
	if _, ok := v.Selected(); ok {
		t.Fatal("cleared list has a selection")
	}
}

func TestCommentscreenRenderErrorKeepsScreenLayout(t *testing.T) {
	v := newTestCommentScreen([]feedback.Comment{{ID: "one", Body: "pending"}})
	v.Resize(25, 7)
	normal := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	failed := strings.Split(
		ansi.Strip(
			v.Render(fmt.Errorf("storage failed\n%s", strings.Repeat("x", 60))),
		),
		"\n",
	)
	requireEqual(t, len(failed), len(normal), "%s",
		"error changed the screen body or height",
	)
	requireEqual(t, strings.Join(failed[:5], "\n"),
		strings.Join(normal[:5], "\n"), "%s",
		"error changed the screen body or height",
	)
	requireEqual(t, strings.HasPrefix(failed[5], "storage failed "),
		true, "error footer=%q", failed[5],
	)
	requireEqual(t, ansi.StringWidth(failed[5]), 25, "error footer=%q",
		failed[5],
	)
	requireEqual(t, strings.Contains(v.Render(nil), "j/k move"), true,
		"error footer=%q", failed[5],
	)
}

func TestCommentscreenClickUsesVisibleRowsWithoutScrollingList(t *testing.T) {
	items := make([]feedback.Comment, 10)
	for index := range items {
		items[index] = feedback.Comment{ID: fmt.Sprint(index)}
	}
	v := newTestCommentScreen(items)
	v.Resize(80, 7)
	v.Move(100)
	requireEqual(t, v.Click(20, 1), true, "%s",
		"visible comment click rejected",
	)

	selected, _ := v.Selected()
	requireEqual(t, selected.ID, "6", "%s",
		"click did not preserve the visible list position",
	)
	requireEqual(t, v.top, 6, "%s",
		"click did not preserve the visible list position",
	)
	requireEqual(t, v.Click(20, 4), true, "%s",
		"last visible row click rejected",
	)

	selected, _ = v.Selected()
	requireEqual(t, selected.ID, "9", "%s",
		"last visible row mapped to wrong comment",
	)

	for _, point := range [][2]int{{0, 0}, {0, 5}, {0, 6}, {-1, 1}, {80, 1}} {
		requireEqual(t, !v.Click(point[0], point[1]), true,
			"invalid point accepted: %v", point,
		)
	}
	v.Update(items[:1])
	requireEqual(t, !v.Click(20, 2), true, "%s", "blank row click accepted")

	v.Update(nil)
	requireEqual(t, !v.Click(20, 1), true, "%s", "empty list click accepted")
}
