package commentscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestReplaceCommentsPreservesFocusByID(t *testing.T) {
	first := comments.Comment{ID: "first", Body: "first"}
	second := comments.Comment{ID: "second", Body: "second"}
	v := NewCommentList([]comments.Comment{first, second})
	v.MoveCommentSelection(1)
	v.ReplaceComments([]comments.Comment{second, first})
	selected, ok := v.SelectedComment()
	testutil.RequireValuesEqual(t, ok, true, "selected=%#v", selected)
	testutil.RequireValuesEqual(
		t,
		selected.ID,
		"second",
		"selected=%#v",
		selected,
	)

	second.Body = "edited"
	v.ReplaceComments([]comments.Comment{first, second})
	selected, _ = v.SelectedComment()
	testutil.RequireValuesEqual(
		t,
		selected.ID,
		"second",
		"selected=%#v",
		selected,
	)
	testutil.RequireValuesEqual(
		t,
		selected.Body,
		"edited",
		"selected=%#v",
		selected,
	)

	v.ReplaceComments([]comments.Comment{first})
	selected, _ = v.SelectedComment()
	testutil.RequireValuesEqual(t, selected.ID, "first",
		"deleted selection fallback=%#v", selected,
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
	v := NewCommentList(items)
	v.Resize(80, 7)
	v.MoveCommentSelection(100)
	lines := strings.Split(ansi.Strip(v.FormatCommentsScreen(nil)), "\n")
	testutil.RequireValuesEqual(t,
		strings.Contains(strings.Join(lines[1:5], "\n"), ">   comment 9"), true,
		"last selection hidden: %q", lines,
	)
	testutil.RequireValuesEqual(t, strings.Contains(lines[5], "j/k move"), true,
		"footer misplaced: %q", lines,
	)

	v.Resize(80, 5)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(v.FormatCommentsScreen(nil), "comment 9"),
		true,
		"%s",
		"resize hid selection",
	)

	v.MoveCommentSelection(-100)
	selected, _ := v.SelectedComment()
	testutil.RequireValuesEqual(
		t,
		selected.ID,
		"0",
		"first selection=%#v",
		selected,
	)
}

func TestEmptyListAndInputOwnership(t *testing.T) {
	v := NewCommentList(nil)
	v.MoveCommentSelection(100)
	v.MoveCommentSelection(-100)
	if _, ok := v.SelectedComment(); ok {
		t.Fatal("empty list has a selection")
	}
	testutil.RequireValuesEqual(
		t,
		strings.Contains(
			v.FormatCommentsScreen(nil),
			"No pending comments.",
		),
		true,
		"%s",
		"empty state missing",
	)

	items := []comments.Comment{{ID: "one", Body: "original"}}
	v.ReplaceComments(items)
	items[0].Body = "changed outside view"
	selected, _ := v.SelectedComment()
	testutil.RequireValuesEqual(t, selected.Body, "original", "%s",
		"view borrowed mutable input slice",
	)

	v.ReplaceComments(nil)
	if _, ok := v.SelectedComment(); ok {
		t.Fatal("cleared list has a selection")
	}
}

func TestFormatCommentsScreenKeepsLayoutOnError(t *testing.T) {
	v := NewCommentList([]comments.Comment{{ID: "one", Body: "pending"}})
	v.Resize(25, 7)
	normal := strings.Split(ansi.Strip(v.FormatCommentsScreen(nil)), "\n")
	failed := strings.Split(
		ansi.Strip(
			v.FormatCommentsScreen(
				fmt.Errorf("storage failed\n%s", strings.Repeat("x", 60)),
			),
		),
		"\n",
	)
	testutil.RequireValuesEqual(t, len(failed), len(normal), "%s",
		"error changed the screen body or height",
	)
	testutil.RequireValuesEqual(t, strings.Join(failed[:5], "\n"),
		strings.Join(normal[:5], "\n"), "%s",
		"error changed the screen body or height",
	)
	testutil.RequireValuesEqual(
		t,
		strings.HasPrefix(failed[5], "storage failed "),
		true,
		"error footer=%q",
		failed[5],
	)
	testutil.RequireValuesEqual(
		t,
		ansi.StringWidth(failed[5]),
		25,
		"error footer=%q",
		failed[5],
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(v.FormatCommentsScreen(nil), "j/k move"),
		true,
		"error footer=%q",
		failed[5],
	)
}

func TestClickUsesVisibleRowsWithoutScrollingList(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{ID: fmt.Sprint(index)}
	}
	v := NewCommentList(items)
	v.Resize(80, 7)
	v.MoveCommentSelection(100)
	testutil.RequireValuesEqual(t, v.Click(20, 1), true, "%s",
		"visible comment click rejected",
	)

	selected, _ := v.SelectedComment()
	testutil.RequireValuesEqual(t, selected.ID, "6", "%s",
		"click did not preserve the visible list position",
	)
	testutil.RequireValuesEqual(t, v.top, 6, "%s",
		"click did not preserve the visible list position",
	)
	testutil.RequireValuesEqual(t, v.Click(20, 4), true, "%s",
		"last visible row click rejected",
	)

	selected, _ = v.SelectedComment()
	testutil.RequireValuesEqual(t, selected.ID, "9", "%s",
		"last visible row mapped to wrong comment",
	)

	for _, point := range [][2]int{{0, 0}, {0, 5}, {0, 6}, {-1, 1}, {80, 1}} {
		testutil.RequireValuesEqual(t, !v.Click(point[0], point[1]), true,
			"invalid point accepted: %v", point,
		)
	}
	v.ReplaceComments(items[:1])
	testutil.RequireValuesEqual(t, !v.Click(20, 2), true, "%s",
		"blank row click accepted",
	)

	v.ReplaceComments(nil)
	testutil.RequireValuesEqual(t, !v.Click(20, 1), true, "%s",
		"empty list click accepted",
	)
}
