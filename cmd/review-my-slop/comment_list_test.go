package main

import (
	fmt "fmt"
	strings "strings"
	testing "testing"

	ansi "github.com/charmbracelet/x/ansi"
)

func TestCommentscreenUpdatePreservesFocusByID(t *testing.T) {
	first := commentsComment{ID: "first", Body: "first"}
	second := commentsComment{ID: "second", Body: "second"}
	v := (func(items []commentsComment) *commentscreenView {
		v := &commentscreenView{width: 80, height: 30}
		v.Update(items)
		return v
	})([]commentsComment{first, second})
	v.Move(1)
	v.Update([]commentsComment{second, first})
	selected, ok := v.Selected()
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ok, true, "selected=%#v", selected)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.ID, "second", "selected=%#v", selected)

	second.Body = "edited"
	v.Update([]commentsComment{first, second})
	selected, _ = v.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.ID, "second", "selected=%#v", selected)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.Body, "edited", "selected=%#v", selected)

	v.Update([]commentsComment{first})
	selected, _ = v.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.ID, "first",
		"deleted selection fallback=%#v", selected,
	)
}

func TestCommentscreenMovementKeepsSelectedCommentVisible(t *testing.T) {
	items := make([]commentsComment, 10)
	for index := range items {
		items[index] = commentsComment{
			ID:   fmt.Sprint(index),
			Body: fmt.Sprintf("comment %d", index),
		}
	}
	v := (func(items []commentsComment) *commentscreenView {
		v := &commentscreenView{width: 80, height: 30}
		v.Update(items)
		return v
	})(items)
	v.Resize(80, 7)
	v.Move(100)
	lines := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(strings.Join(lines[1:5], "\n"), ">   comment 9"), true,
		"last selection hidden: %q", lines,
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(lines[5], "j/k move"), true,
		"footer misplaced: %q", lines,
	)

	v.Resize(80, 5)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(v.Render(nil), "comment 9"), true,
		"%s", "resize hid selection",
	)

	v.Move(-100)
	selected, _ := v.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.ID, "0", "first selection=%#v", selected)
}

func TestCommentscreenEmptyListAndInputOwnership(t *testing.T) {
	v := (func(items []commentsComment) *commentscreenView {
		v := &commentscreenView{width: 80, height: 30}
		v.Update(items)
		return v
	})(nil)
	v.Move(100)
	v.Move(-100)
	if _, ok := v.Selected(); ok {
		t.Fatal("empty list has a selection")
	}
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t,
		strings.Contains(v.Render(nil), "No pending comments."), true, "%s",
		"empty state missing",
	)

	items := []commentsComment{{ID: "one", Body: "original"}}
	v.Update(items)
	items[0].Body = "changed outside view"
	selected, _ := v.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.Body, "original", "%s",
		"view borrowed mutable input slice",
	)

	v.Update(nil)
	if _, ok := v.Selected(); ok {
		t.Fatal("cleared list has a selection")
	}
}

func TestCommentscreenRenderErrorKeepsScreenLayout(t *testing.T) {
	v := (func(items []commentsComment) *commentscreenView {
		v := &commentscreenView{width: 80, height: 30}
		v.Update(items)
		return v
	})([]commentsComment{{ID: "one", Body: "pending"}})
	v.Resize(25, 7)
	normal := strings.Split(ansi.Strip(v.Render(nil)), "\n")
	failed := strings.Split(
		ansi.Strip(
			v.Render(fmt.Errorf("storage failed\n%s", strings.Repeat("x", 60))),
		),
		"\n",
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(failed), len(normal), "%s",
		"error changed the screen body or height",
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Join(failed[:5], "\n"),
		strings.Join(normal[:5], "\n"), "%s",
		"error changed the screen body or height",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.HasPrefix(failed[5], "storage failed "),
		true, "error footer=%q", failed[5],
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, ansi.StringWidth(failed[5]), 25, "error footer=%q",
		failed[5],
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, strings.Contains(v.Render(nil), "j/k move"), true,
		"error footer=%q", failed[5],
	)
}

func TestCommentscreenClickUsesVisibleRowsWithoutScrollingList(t *testing.T) {
	items := make([]commentsComment, 10)
	for index := range items {
		items[index] = commentsComment{ID: fmt.Sprint(index)}
	}
	v := (func(items []commentsComment) *commentscreenView {
		v := &commentscreenView{width: 80, height: 30}
		v.Update(items)
		return v
	})(items)
	v.Resize(80, 7)
	v.Move(100)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.Click(20, 1), true, "%s",
		"visible comment click rejected",
	)

	selected, _ := v.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.ID, "6", "%s",
		"click did not preserve the visible list position",
	)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.top, 6, "%s",
		"click did not preserve the visible list position",
	)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, v.Click(20, 4), true, "%s",
		"last visible row click rejected",
	)

	selected, _ = v.Selected()
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, selected.ID, "9", "%s",
		"last visible row mapped to wrong comment",
	)

	for _, point := range [][2]int{{0, 0}, {0, 5}, {0, 6}, {-1, 1}, {80, 1}} {
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, !v.Click(point[0], point[1]), true,
			"invalid point accepted: %v", point,
		)
	}
	v.Update(items[:1])
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !v.Click(20, 2), true, "%s",
		"blank row click accepted",
	)

	v.Update(nil)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, !v.Click(20, 1), true, "%s",
		"empty list click accepted",
	)
}
