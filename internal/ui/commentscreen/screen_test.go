package commentscreen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestUpdatePreservesFocusByID(t *testing.T) {
	first := comments.Comment{ID: "first", Body: "first"}
	second := comments.Comment{ID: "second", Body: "second"}
	v := New([]comments.Comment{first, second})
	v.Move(1)
	v.Update([]comments.Comment{second, first})
	selected, ok := v.Selected()
	if !ok || selected.ID != "second" {
		t.Fatalf("selected=%#v", selected)
	}
	second.Body = "edited"
	v.Update([]comments.Comment{first, second})
	selected, _ = v.Selected()
	if selected.ID != "second" || selected.Body != "edited" {
		t.Fatalf("selected=%#v", selected)
	}
	v.Update([]comments.Comment{first})
	selected, _ = v.Selected()
	if selected.ID != "first" {
		t.Fatalf("deleted selection fallback=%#v", selected)
	}
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
	if !strings.Contains(strings.Join(lines[1:5], "\n"), ">   comment 9") {
		t.Fatalf("last selection hidden: %q", lines)
	}
	if !strings.Contains(lines[5], "j/k move") {
		t.Fatalf("footer misplaced: %q", lines)
	}
	v.Resize(80, 5)
	if !strings.Contains(v.Render(nil), "comment 9") {
		t.Fatal("resize hid selection")
	}
	v.Move(-100)
	selected, _ := v.Selected()
	if selected.ID != "0" {
		t.Fatalf("first selection=%#v", selected)
	}
}

func TestEmptyListAndInputOwnership(t *testing.T) {
	v := New(nil)
	v.Move(100)
	v.Move(-100)
	if _, ok := v.Selected(); ok {
		t.Fatal("empty list has a selection")
	}
	if !strings.Contains(v.Render(nil), "No pending comments.") {
		t.Fatal("empty state missing")
	}
	items := []comments.Comment{{ID: "one", Body: "original"}}
	v.Update(items)
	items[0].Body = "changed outside view"
	selected, _ := v.Selected()
	if selected.Body != "original" {
		t.Fatal("view borrowed mutable input slice")
	}
	v.Update(nil)
	if _, ok := v.Selected(); ok {
		t.Fatal("cleared list has a selection")
	}
}

func TestRenderErrorKeepsScreenLayout(t *testing.T) {
	v := New([]comments.Comment{{ID: "one", Body: "pending"}})
	v.Resize(25, 7)
	normal := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	failed := strings.Split(ansi.Strip(v.Render(fmt.Errorf("storage failed\n%s", strings.Repeat("x", 60)))), "\n")
	if len(failed) != len(normal) || strings.Join(failed[:5], "\n") != strings.Join(normal[:5], "\n") {
		t.Fatal("error changed the screen body or height")
	}
	if !strings.HasPrefix(failed[5], "storage failed ") || ansi.StringWidth(failed[5]) != 25 || !strings.Contains(v.Render(nil), "j/k move") {
		t.Fatalf("error footer=%q", failed[5])
	}
}

func TestClickUsesVisibleRowsWithoutScrollingList(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{ID: fmt.Sprint(index)}
	}
	v := New(items)
	v.Resize(80, 7)
	v.Move(100)
	if !v.Click(20, 1) {
		t.Fatal("visible comment click rejected")
	}
	selected, _ := v.Selected()
	if selected.ID != "6" || v.top != 6 {
		t.Fatal("click did not preserve the visible list position")
	}
	if !v.Click(20, 4) {
		t.Fatal("last visible row click rejected")
	}
	selected, _ = v.Selected()
	if selected.ID != "9" {
		t.Fatal("last visible row mapped to wrong comment")
	}
	for _, point := range [][2]int{{0, 0}, {0, 5}, {0, 6}, {-1, 1}, {80, 1}} {
		if v.Click(point[0], point[1]) {
			t.Fatalf("invalid point accepted: %v", point)
		}
	}
	v.Update(items[:1])
	if v.Click(20, 2) {
		t.Fatal("blank row click accepted")
	}
	v.Update(nil)
	if v.Click(20, 1) {
		t.Fatal("empty list click accepted")
	}
}
