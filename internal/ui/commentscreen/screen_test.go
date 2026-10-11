package commentscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestUpdatePreservesFocusByID(t *testing.T) {
	first := comments.Comment{ID: fixtureFirst, Body: fixtureFirst}
	second := comments.Comment{ID: "second", Body: "second"}
	v := New([]comments.Comment{first, second})
	v.Move(one)
	v.Update([]comments.Comment{second, first})
	selected, ok := v.Selected()
	requireEqual(
		t,
		ok && selected.ID == "second", true,
		"selected=%#v",
		selected,
	)
	second.Body = "edited"
	v.Update([]comments.Comment{first, second})
	selected, _ = v.Selected()
	requireEqual(
		t,
		selected.ID == "second" && selected.Body == "edited", true,
		"selected=%#v",
		selected,
	)
	v.Update([]comments.Comment{first})
	selected, _ = v.Selected()
	requireEqual(
		t,
		selected.ID, fixtureFirst,
		"deleted selection fallback=%#v",
		selected,
	)
}

func TestMovementKeepsSelectedCommentVisible(t *testing.T) {
	items := make([]comments.Comment, ten)
	for index := range items {
		items[index] = comments.Comment{
			ID:   fmt.Sprint(index),
			Body: fmt.Sprintf("comment %d", index),
		}
	}
	v := New(items)
	v.Resize(defaultWidth, seven)
	v.Move(hundred)
	lines := strings.Split(ansi.Strip(v.Render(nil)), newline)
	requireEqual(
		t,
		strings.Contains(
			strings.Join(lines[one:five], newline),
			">   comment 9",
		),
		true,
		"last selection hidden: %q",
		lines,
	)
	requireEqual(
		t,
		strings.Contains(lines[five], "j/k move"), true,
		"footer misplaced: %q",
		lines,
	)
	v.Resize(defaultWidth, five)
	requireEqual(
		t,
		strings.Contains(v.Render(nil), "comment 9"),
		true,
		fixtureV,
		"resize hid selection",
	)
	v.Move(-hundred)
	selected, _ := v.Selected()
	requireEqual(t, selected.ID, "0", "first selection=%#v", selected)
}

func TestEmptyListAndInputOwnership(t *testing.T) {
	v := New(nil)
	v.Move(hundred)
	v.Move(-hundred)
	{
		_, ok := v.Selected()
		requireEqual(t, !ok, true, fixtureV, "empty list has a selection")
	}
	requireEqual(
		t,
		strings.Contains(v.Render(nil), "No pending comments."),
		true,
		fixtureV,
		"empty state missing",
	)
	items := []comments.Comment{{ID: "one", Body: "original"}}
	v.Update(items)
	items[zero].Body = "changed outside view"
	selected, _ := v.Selected()
	requireEqual(
		t,
		selected.Body, "original",
		fixtureV,
		"view borrowed mutable input slice",
	)
	v.Update(nil)
	{
		_, ok := v.Selected()
		requireEqual(t, !ok, true, fixtureV, "cleared list has a selection")
	}
}

func TestRenderErrorKeepsScreenLayout(t *testing.T) {
	v := New([]comments.Comment{{ID: "one", Body: "pending"}})
	v.Resize(fixtureNumber25, seven)
	normal := strings.Split(ansi.Strip(v.Render(nil)), newline)
	failed := strings.Split(
		ansi.Strip(
			v.Render(
				fmt.Errorf("storage failed\n%s", strings.Repeat("x", sixty)),
			),
		),
		newline,
	)
	requireEqual(
		t,
		len(failed) == len(normal) &&
			strings.Join(failed[:five], newline) == strings.Join(
				normal[:five],
				newline,
			),
		true,
		fixtureV,
		"error changed the screen body or height",
	)
	requireEqual(
		t,
		strings.HasPrefix(failed[five], "storage failed ") &&
			ansi.StringWidth(failed[five]) == fixtureNumber25 &&
			strings.Contains(v.Render(nil), "j/k move"),
		true,
		"error footer=%q",
		failed[five],
	)
}

func TestClickUsesVisibleRowsWithoutScrollingList(t *testing.T) {
	items := make([]comments.Comment, ten)
	for index := range items {
		items[index] = comments.Comment{ID: fmt.Sprint(index)}
	}
	v := New(items)
	v.Resize(defaultWidth, seven)
	v.Move(hundred)
	requireEqual(
		t,
		v.Click(minimumWidth, one),
		true,
		fixtureV,
		"visible comment click rejected",
	)
	selected, _ := v.Selected()
	requireEqual(
		t,
		selected.ID == "6" && v.top == six,
		true,
		fixtureV,
		"click did not preserve the visible list position",
	)
	requireEqual(
		t,
		v.Click(minimumWidth, four),
		true,
		fixtureV,
		"last visible row click rejected",
	)
	selected, _ = v.Selected()
	requireEqual(
		t,
		selected.ID, "9",
		fixtureV,
		"last visible row mapped to wrong comment",
	)
	for _, point := range [][two]int{
		{zero, zero},
		{zero, five},
		{zero, six},
		{-one, one},
		{defaultWidth, one},
	} {
		requireEqual(
			t,
			!v.Click(point[zero], point[one]), true,
			"invalid point accepted: %v",
			point,
		)
	}
	v.Update(items[:one])
	requireEqual(
		t,
		!v.Click(minimumWidth, two),
		true,
		fixtureV,
		"blank row click accepted",
	)
	v.Update(nil)
	requireEqual(
		t,
		!v.Click(minimumWidth, one),
		true,
		fixtureV,
		"empty list click accepted",
	)
}
