package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/helpscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

func TestVisualSelectionCreatesMappedAnchorAndSubmits(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved []comments.Comment
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize30 := layout.size
		if inlineLocalSize30.Width <= 0 || inlineLocalSize30.Height <= 0 {
			inlineLocalSize30 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize30.Width, height: inlineLocalSize30.Height, save: func(stored comments.Comment, _ patch.Patch) (comments.Comment, error) {
			saved = append(saved, stored)
			stored.ID = "new"
			return stored, nil
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "j"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "v"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "j"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "c"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	_ = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(commentEditorFinishedMsg{body: "fix both lines"})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, len(saved), 1, "saved comments = %d", len(saved))

	anchor := saved[0].Anchor
	testutil.RequireEqual(t, anchor.FilePath, "main.go", "anchor = %#v", anchor)
	testutil.RequireEqual(t, anchor.OldStart, 2, "anchor = %#v", anchor)
	testutil.RequireEqual(t, anchor.NewStart, 2, "anchor = %#v", anchor)
	testutil.RequireEqual(t,
		slices.Equal(anchor.QuotedLines, []string{"-old()", "+new()"}), true,
		"anchor = %#v", anchor,
	)
}

func TestCommentSaveFailureClearsPendingEdit(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize12 := layout.size
		if inlineLocalSize12.Width <= 0 || inlineLocalSize12.Height <= 0 {
			inlineLocalSize12 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize12.Width, height: inlineLocalSize12.Height, save: func(comments.Comment, patch.Patch) (comments.Comment, error) {
			return comments.Comment{}, errors.New("storage unavailable")
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "c"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(commentEditorFinishedMsg{body: "keep this"})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.edit.body, "", "body=%q err=%v", m.edit.body,
		m.err,
	)
	testutil.RequireEqual(t, m.err != nil, true, "body=%q err=%v", m.edit.body,
		m.err,
	)
	testutil.RequireEqual(t, m.err.Error(), "storage unavailable",
		"body=%q err=%v", m.edit.body, m.err,
	)
}

func TestCommentRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := func() model {
		var (
			m model = func() model {
				var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
				var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
				inlineLocalSize29 := layout.size
				if inlineLocalSize29.Width <= 0 || inlineLocalSize29.Height <= 0 {
					inlineLocalSize29 = defaultSize
				}
				m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize29.Width, height: inlineLocalSize29.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
				m.diffView = diffscreen.New(p, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "c"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.err != nil, true, "error = %v", m.err)
	testutil.RequireEqual(t, m.err.Error(), "$EDITOR is not set", "error = %v",
		m.err,
	)
}

func TestEmptyNewCommentIsDiscarded(t *testing.T) {
	t.Setenv("EDITOR", "true")
	called := false
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize11 := layout.size
		if inlineLocalSize11.Width <= 0 || inlineLocalSize11.Height <= 0 {
			inlineLocalSize11 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize11.Width, height: inlineLocalSize11.Height, save: func(stored comments.Comment, _ patch.Patch) (comments.Comment, error) {
			called = true
			return stored, nil
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "c"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(commentEditorFinishedMsg{body: " \n"})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, !called, true, "called=%v comments=%d", called,
		len(m.comments.items),
	)
	testutil.RequireEqual(t, len(m.comments.items), 0, "called=%v comments=%d",
		called, len(m.comments.items),
	)
}

func TestOpenCurrentLineUsesEditorWithWorkingTreeLocation(t *testing.T) {
	t.Setenv("EDITOR", "printf")
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize5 := layout.size
		if inlineLocalSize5.Width <= 0 || inlineLocalSize5.Height <= 0 {
			inlineLocalSize5 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize5.Width, height: inlineLocalSize5.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.currentPatch.Root = "/tmp/repo with spaces"
	func() {
		var (
			m    *model = &m
			text string = "new()"
		)
		t.Helper()
		m.diffView.Move(diffscreen.FirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreen.NextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	cmd, err := func() (tea.Cmd, error) {
		path, number, err := func() (string, int, error) {
			file, line, ok := m.diffView.Current()
			if !ok {
				return "", 0, errors.New("select a code line to open in $EDITOR")
			}
			path, number := file.NewPath, line.NewNumber
			if path == "" || path == "/dev/null" {
				path = file.OldPath
			}
			if number == 0 {
				number = line.OldNumber
			}
			if path == "" || path == "/dev/null" || number < 1 {
				return "", 0, errors.New("current line has no editable working-tree location")
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
			}
			return path, int(number), nil
		}()
		if err != nil {
			return nil, err
		}
		return editor.OpenSource(m.ctx, path, number, func(err error) tea.Msg { return sourceEditorFinishedMsg{err: err} })
	}()
	testutil.RequireEqual(t, err, nil, "command=%v err=%v", cmd, err)
	testutil.RequireEqual(t, cmd != nil, true, "command=%v err=%v", cmd, err)

	path, line, err := func() (string, int, error) {
		file, line, ok := m.diffView.Current()
		if !ok {
			return "", 0, errors.New("select a code line to open in $EDITOR")
		}
		path, number := file.NewPath, line.NewNumber
		if path == "" || path == "/dev/null" {
			path = file.OldPath
		}
		if number == 0 {
			number = line.OldNumber
		}
		if path == "" || path == "/dev/null" || number < 1 {
			return "", 0, errors.New("current line has no editable working-tree location")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
		}
		return path, int(number), nil
	}()
	testutil.RequireEqual(t, err, nil, "location=%q:%d err=%v", path, line, err)
	testutil.RequireEqual(t, path, "/tmp/repo with spaces/main.go",
		"location=%q:%d err=%v", path, line, err,
	)
	testutil.RequireEqual(t, line, 2, "location=%q:%d err=%v", path, line, err)
}

func TestOpenCurrentLineRequiresEditor(t *testing.T) {
	t.Setenv("EDITOR", "")
	m := func() model {
		var (
			m model = func() model {
				var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
				var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
				inlineLocalSize18 := layout.size
				if inlineLocalSize18.Width <= 0 || inlineLocalSize18.Height <= 0 {
					inlineLocalSize18 = defaultSize
				}
				m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize18.Width, height: inlineLocalSize18.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
				m.diffView = diffscreen.New(p, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "e"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.err != nil, true, "error = %v", m.err)
	testutil.RequireEqual(t, m.err.Error(), "$EDITOR is not set", "error = %v",
		m.err,
	)
}

func TestCommentsCanBeViewedEditedAndDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	items := []comments.Comment{
		{ID: "one", Body: "old body"},
		{ID: "two", Body: "second"},
	}
	var persisted, deleted comments.Comment
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize33 := layout.size
		if inlineLocalSize33.Width <= 0 || inlineLocalSize33.Height <= 0 {
			inlineLocalSize33 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: items}, commentView: commentscreen.New(items), edit: commentEdit{index: -1}, width: inlineLocalSize33.Width, height: inlineLocalSize33.Height, save: func(stored comments.Comment, _ patch.Patch) (comments.Comment, error) {
			persisted = stored
			return stored, nil
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.delete = func(stored comments.Comment, _ patch.Patch) error { deleted = stored; return nil }
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "C"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.mode, modeComments, "%s",
		"comments did not open",
	)
	testutil.RequireEqual(t, strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "old body"), true,
		"%s", "comments did not open",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(commentEditorFinishedMsg{body: "edited body"})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, persisted.ID, "one", "persisted = %#v", persisted)
	testutil.RequireEqual(t, persisted.Body, "edited body", "persisted = %#v",
		persisted,
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "D"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, deleted.ID, "one", "deleted=%#v comments=%#v",
		deleted, m.comments.items,
	)
	testutil.RequireEqual(t, len(m.comments.items), 1,
		"deleted=%#v comments=%#v", deleted, m.comments.items,
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "q"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.mode, modeBrowse, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
	testutil.RequireEqual(t, !m.quitting, true, "mode=%v quitting=%v", m.mode,
		m.quitting,
	)
}

func TestOpeningCommentsReloadsPendingComments(t *testing.T) {
	m := func() model {
		var (
			p     patch.Patch        = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
			items []comments.Comment = []comments.Comment{{ID: "read", Body: "already read"}}
		)
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize7 := layout.size
		if inlineLocalSize7.Width <= 0 || inlineLocalSize7.Height <= 0 {
			inlineLocalSize7 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: items}, commentView: commentscreen.New(items), edit: commentEdit{index: -1}, width: inlineLocalSize7.Width, height: inlineLocalSize7.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.load = func() ([]comments.Comment, error) { return nil, nil }

	next, cmd := m.Update(func() tea.KeyPressMsg {
		var text string = "C"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() model {
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, cmd != nil, true, "%s",
		"opening comments did not request a refresh",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = cmd()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.mode, modeComments, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
	testutil.RequireEqual(t, len(m.comments.items), 0, "mode=%v comments=%#v",
		m.mode, m.comments.items,
	)
}

func TestCommentReloadFailurePreservesCurrentComments(t *testing.T) {
	m := func() model {
		var (
			p     patch.Patch        = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
			items []comments.Comment = []comments.Comment{{ID: "keep", Body: "keep"}}
		)
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize14 := layout.size
		if inlineLocalSize14.Width <= 0 || inlineLocalSize14.Height <= 0 {
			inlineLocalSize14 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: items}, commentView: commentscreen.New(items), edit: commentEdit{index: -1}, width: inlineLocalSize14.Width, height: inlineLocalSize14.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.load = func() ([]comments.Comment, error) { return nil, errors.New("storage unavailable") }

	next, cmd := m.Update(func() tea.KeyPressMsg {
		var text string = "C"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() model {
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = cmd()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, len(m.comments.items), 1, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	testutil.RequireEqual(t, m.err != nil, true, "comments=%#v error=%v",
		m.comments.items, m.err,
	)
	testutil.RequireEqual(t, m.err.Error(),
		"refresh comments: storage unavailable", "comments=%#v error=%v",
		m.comments.items, m.err,
	)
}

func TestEmptyEditedCommentIsDeleted(t *testing.T) {
	t.Setenv("EDITOR", "true")
	m := func() model {
		var (
			p     patch.Patch        = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
			items []comments.Comment = []comments.Comment{{ID: "one", Body: "old"}}
		)
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize16 := layout.size
		if inlineLocalSize16.Width <= 0 || inlineLocalSize16.Height <= 0 {
			inlineLocalSize16 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: items}, commentView: commentscreen.New(items), edit: commentEdit{index: -1}, width: inlineLocalSize16.Width, height: inlineLocalSize16.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	deleted := false
	m.delete = func(comments.Comment, patch.Patch) error { deleted = true; return nil }
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "C"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(commentEditorFinishedMsg{body: "\n"})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, deleted, true, "deleted=%v comments=%d", deleted,
		len(m.comments.items),
	)
	testutil.RequireEqual(t, len(m.comments.items), 0, "deleted=%v comments=%d",
		deleted, len(m.comments.items),
	)
}

func TestCommentDeleteFailureKeepsCommentAndShowsError(t *testing.T) {
	m := func() model {
		var (
			p     patch.Patch        = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
			items []comments.Comment = []comments.Comment{{ID: "one", Body: "keep"}}
		)
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize21 := layout.size
		if inlineLocalSize21.Width <= 0 || inlineLocalSize21.Height <= 0 {
			inlineLocalSize21 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: items}, commentView: commentscreen.New(items), edit: commentEdit{index: -1}, width: inlineLocalSize21.Width, height: inlineLocalSize21.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.delete = func(comments.Comment, patch.Patch) error { return errors.New("delete failed") }
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "C"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "D"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, len(m.comments.items), 1, "comments=%#v render=%q",
		m.comments.items, func() string {
			switch m.mode {
			case modeHelp:
				bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
				return helpscreen.Render(bindings, m.width, m.height)
			case modeComments:
				return m.commentView.Render(m.err)
			default:
				return m.diffView.Render(m.err)
			}
		}(),
	)
	testutil.RequireEqual(t,
		strings.Contains(ansi.Strip(func() string {
			switch m.mode {
			case modeHelp:
				bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
				return helpscreen.Render(bindings, m.width, m.height)
			case modeComments:
				return m.commentView.Render(m.err)
			default:
				return m.diffView.Render(m.err)
			}
		}()), "delete failed"), true,
		"comments=%#v render=%q", m.comments.items, func() string {
			switch m.mode {
			case modeHelp:
				bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
				return helpscreen.Render(bindings, m.width, m.height)
			case modeComments:
				return m.commentView.Render(m.err)
			default:
				return m.diffView.Render(m.err)
			}
		}(),
	)
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize31 := layout.size
		if inlineLocalSize31.Width <= 0 || inlineLocalSize31.Height <= 0 {
			inlineLocalSize31 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize31.Width, height: inlineLocalSize31.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "v"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	for range 10 {
		m = func() model {
			var (
				m   model   = m
				msg tea.Msg = func() tea.KeyPressMsg {
					var text string = "j"
					runes := []rune(text)
					return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
				}()
			)
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
	}
	_, line, _ := m.diffView.Current()
	testutil.RequireEqual(t, line.Text != "more()", true, "%s",
		"diffSelection crossed hunk",
	)
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize26 := layout.size
		if inlineLocalSize26.Width <= 0 || inlineLocalSize26.Height <= 0 {
			inlineLocalSize26 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize26.Width, height: inlineLocalSize26.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	var saved []bool
	m.saveLayout = func(enabled bool) error { saved = append(saved, enabled); return nil }
	m.diffOptions.SideBySide = false
	m.diffView.Configure(m.diffOptions)
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "G"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "more()", "G line=%q", func() string { _, line, _ := m.diffView.Current(); return line.Text }())

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "g"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "g"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "gg line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "t"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true, "%s",
		"split view not enabled",
	)
	testutil.RequireEqual(t, strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "│"), true, "%s",
		"split view not enabled",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "t"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, !m.diffOptions.SideBySide, true, "saved=%v", saved)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize2 := layout.size
		if inlineLocalSize2.Width <= 0 || inlineLocalSize2.Height <= 0 {
			inlineLocalSize2 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize2.Width, height: inlineLocalSize2.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	var saved []bool
	m.saveLayout = func(enabled bool) error { saved = append(saved, enabled); return nil }
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "t"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, !m.diffOptions.SideBySide, true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{false}), true,
		"sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved,
	)
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize24 := layout.size
		if inlineLocalSize24.Width <= 0 || inlineLocalSize24.Height <= 0 {
			inlineLocalSize24 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize24.Width, height: inlineLocalSize24.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.saveLayout = nil
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	func() {
		var (
			m    *model = &m
			text string = "keep()"
		)
		t.Helper()
		m.diffView.Move(diffscreen.FirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreen.NextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	_, before, _ := m.diffView.Current()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got,
		before,
	)
	testutil.RequireEqual(t, got, before, "focused line=%#v, want %#v", got,
		before,
	)
}

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreen.Alignment
	}{
		{
			"z",
			diffscreen.Center,
		}, {"t", diffscreen.Top}, {"b", diffscreen.Bottom},
	} {
		m := func() model {
			var p patch.Patch = func() patch.Patch {
				lines := make([]patch.Line, 30)
				for index := range lines {
					lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
				}
				return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
			}()
			var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
			inlineLocalSize4 := layout.size
			if inlineLocalSize4.Width <= 0 || inlineLocalSize4.Height <= 0 {
				inlineLocalSize4 = defaultSize
			}
			m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize4.Width, height: inlineLocalSize4.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
			m.diffView = diffscreen.New(p, m.diffOptions)
			m.diffView.Resize(m.width, m.height)
			m.commentView.Resize(m.width, m.height)
			return m
		}()
		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 9})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		want := diffscreen.New(func() patch.Patch {
			lines := make([]patch.Line, 30)
			for index := range lines {
				lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
			}
			return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
		}(), diffscreen.Options{Dark: true})
		want.Resize(100, 9)
		for range 10 {
			m.diffView.Move(diffscreen.NextLine)
			want.Move(diffscreen.NextLine)
		}
		want.Align(test.alignment)
		m = func() model {
			var (
				m   model   = m
				msg tea.Msg = func() tea.KeyPressMsg {
					var text string = "z"
					runes := []rune(text)
					return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
				}()
			)
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		m = func() model {
			var (
				m   model   = m
				msg tea.Msg = func() tea.KeyPressMsg {
					runes := []rune(test.key)
					return tea.KeyPressMsg(tea.Key{Text: test.key, Code: runes[0]})
				}()
			)
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		testutil.RequireEqual(t, m.diffView.Render(nil), want.Render(nil),
			"z%s did not align view", test.key,
		)
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize34 := layout.size
		if inlineLocalSize34.Width <= 0 || inlineLocalSize34.Height <= 0 {
			inlineLocalSize34 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize34.Width, height: inlineLocalSize34.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "G"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	last := func() string { _, line, _ := m.diffView.Current(); return line.Text }()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "g"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "h"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "g"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), last, "%s",
		"pending prefix moved focus",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "g"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "gg line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)
}

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize32 := layout.size
		if inlineLocalSize32.Width <= 0 || inlineLocalSize32.Height <= 0 {
			inlineLocalSize32 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize32.Width, height: inlineLocalSize32.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	status := ansi.Strip(strings.Split(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "\n")[m.height-2])
	testutil.RequireEqual(t, strings.HasPrefix(status, "j/k/h/l move"), true,
		"status=%q", status,
	)
	testutil.RequireEqual(t, strings.HasSuffix(status, "local changes"), true,
		"status=%q", status,
	)

	help := ansi.Strip(func() string {
		bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
		return helpscreen.Render(bindings, m.width, m.height)
	}())
	for _, binding := range []string{
		"Ctrl-w h/l/w",
		"zz/zt/zb",
		"n/N",
		"]f/[f",
		"Tab",
		"t",
	} {
		testutil.RequireEqual(t, strings.Contains(help, binding), true,
			"help missing %q", binding,
		)
	}
}

func TestStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := func() model {
		var p patch.Patch = func() patch.Patch {
			lines := make([]patch.Line, 30)
			for index := range lines {
				lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
			}
			return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
		}()
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize19 := layout.size
		if inlineLocalSize19.Width <= 0 || inlineLocalSize19.Height <= 0 {
			inlineLocalSize19 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize19.Width, height: inlineLocalSize19.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 9})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, !strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "%"), true, "%s",
		"initial view shows progress",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "l"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, !strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "%"), true, "%s",
		"horizontal scroll shows progress",
	)

	for range 10 {
		m = func() model {
			var (
				m   model   = m
				msg tea.Msg = func() tea.KeyPressMsg {
					var text string = "j"
					runes := []rune(text)
					return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
				}()
			)
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
	}
	testutil.RequireEqual(t, strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "%)"), true, "%s",
		"scrolled view hides progress",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "G"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t,
		strings.Contains(func() string {
			switch m.mode {
			case modeHelp:
				bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
				return helpscreen.Render(bindings, m.width, m.height)
			case modeComments:
				return m.commentView.Render(m.err)
			default:
				return m.diffView.Render(m.err)
			}
		}(), "local changes (100%)"), true, "%s",
		"final view hides progress",
	)
}

func TestStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize15 := layout.size
		if inlineLocalSize15.Width <= 0 || inlineLocalSize15.Height <= 0 {
			inlineLocalSize15 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize15.Width, height: inlineLocalSize15.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 100})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, !strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "%)"), true, "%s",
		"fitting diff shows progress",
	)
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize3 := layout.size
		if inlineLocalSize3.Width <= 0 || inlineLocalSize3.Height <= 0 {
			inlineLocalSize3 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize3.Width, height: inlineLocalSize3.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	func() {
		var (
			m    *model = &m
			text string = "new()"
		)
		t.Helper()
		m.diffView.Move(diffscreen.FirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreen.NextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "h"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "old pane line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: 'w', Mod: tea.ModCtrl})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "new()", "new pane line=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := func() model {
		var p patch.Patch = func() patch.Patch {
			lines := make([]patch.Line, 30)
			for index := range lines {
				lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
			}
			return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
		}()
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize13 := layout.size
		if inlineLocalSize13.Width <= 0 || inlineLocalSize13.Height <= 0 {
			inlineLocalSize13 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize13.Width, height: inlineLocalSize13.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 37, Height: 30})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	initial := m.diffView.Render(nil)
	want := diffscreen.New(func() patch.Patch {
		lines := make([]patch.Line, 30)
		for index := range lines {
			lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
		}
		return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
	}(), diffscreen.Options{Dark: true})
	want.Resize(37, 30)
	want.ScrollHorizontal(2 * horizontalScrollStep)
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "l"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.diffView.Render(nil), want.Render(nil), "%s",
		"right keys did not scroll by two steps",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "h"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.diffView.Render(nil), initial, "%s",
		"left keys did not restore initial offset",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "$"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.diffView.Render(nil) != initial, true, "%s",
		"$ did not scroll",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "0"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.diffView.Render(nil), initial, "%s",
		"0 did not restore initial offset",
	)
}

func TestFocusAndManualRefreshLoadCurrentView(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize9 := layout.size
		if inlineLocalSize9.Width <= 0 || inlineLocalSize9.Height <= 0 {
			inlineLocalSize9 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize9.Width, height: inlineLocalSize9.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.currentPatch.Branch = "main"
	m.kind = patch.Branch
	var requested []patch.Kind
	m.refresh = func(kind patch.Kind) (patch.Patch, error) {
		requested = append(requested, kind)
		p := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		p.Kind, p.Branch = kind, "main"
		p.Files[0].Metadata = []string{fmt.Sprintf("refresh-%d", len(requested))}
		return p, nil
	}
	next, cmd := m.Update(tea.FocusMsg{})
	m = func() model {
		var next tea.Model = next
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, cmd != nil, true, "%s", "focus did not refresh")

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = cmd()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	next, cmd = m.Update(func() tea.KeyPressMsg {
		var text string = "R"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() model {
		var next tea.Model = next
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, cmd != nil, true, "%s", "R did not refresh")

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = cmd()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t,
		slices.Equal(requested, []patch.Kind{patch.Branch, patch.Branch}), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
	testutil.RequireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"refresh-2"},
		), true,
		"requested=%v metadata=%q", requested, m.currentPatch.Files[0].Metadata,
	)
}

func TestSourceEditorCompletionRefreshesDiff(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize10 := layout.size
		if inlineLocalSize10.Width <= 0 || inlineLocalSize10.Height <= 0 {
			inlineLocalSize10 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize10.Width, height: inlineLocalSize10.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	refreshed := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
	refreshed.Files[0].Metadata = []string{"after-editor"}
	m.refresh = func(patch.Kind) (patch.Patch, error) { return refreshed, nil }

	next, cmd := m.Update(sourceEditorFinishedMsg{})
	m = func() model {
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, cmd != nil, true, "%s",
		"editor completion did not refresh",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = cmd()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t,
		slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"after-editor"},
		),
		true, "metadata=%q", m.currentPatch.Files[0].Metadata,
	)
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(
		ansi.Strip(func() string {
			var m model = func() model {
				var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
				var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
				inlineLocalSize8 := layout.size
				if inlineLocalSize8.Width <= 0 || inlineLocalSize8.Height <= 0 {
					inlineLocalSize8 = defaultSize
				}
				m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize8.Width, height: inlineLocalSize8.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
				m.diffView = diffscreen.New(p, m.diffOptions)
				m.diffView.Resize(m.width, m.height)
				m.commentView.Resize(m.width, m.height)
				return m
			}()
			switch m.mode {
			case modeHelp:
				bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
				return helpscreen.Render(bindings, m.width, m.height)
			case modeComments:
				return m.commentView.Render(m.err)
			default:
				return m.diffView.Render(m.err)
			}
		}()),
		"\n",
		2,
	)[0]
	testutil.RequireEqual(t, header, "review-my-slop  +2-1", "header = %q",
		header,
	)
}

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize20 := layout.size
		if inlineLocalSize20.Width <= 0 || inlineLocalSize20.Height <= 0 {
			inlineLocalSize20 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize20.Width, height: inlineLocalSize20.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	origin := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "/"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "keep"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	first := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	testutil.RequireEqual(t, m.mode, modeSearch, "mode=%v line=%q", m.mode,
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()", "mode=%v line=%q", m.mode,
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "n"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}() != first, true,
		"next=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()", "next=%#v",
		func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "N"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), first,
		"previous=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "/"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "missing"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "no matches"), true,
		"%s", "missing search did not miss",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), first,
		"cancel=%#v origin=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(), origin,
	)
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize1 := layout.size
		if inlineLocalSize1.Width <= 0 || inlineLocalSize1.Height <= 0 {
			inlineLocalSize1 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize1.Width, height: inlineLocalSize1.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	origin := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "/"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "main.go"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	file, _, _ := m.diffView.Current()
	testutil.RequireEqual(t, file.DisplayPath, "main.go", "file=%q",
		file.DisplayPath,
	)

	for range len("main.go") {
		m = func() model {
			var (
				m   model   = m
				msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace})
			)
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
	}
	testutil.RequireEqual(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), origin,
		"diffCursor=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(),
	)
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize28 := layout.size
		if inlineLocalSize28.Width <= 0 || inlineLocalSize28.Height <= 0 {
			inlineLocalSize28 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize28.Width, height: inlineLocalSize28.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	origin := func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "/"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "old()"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "diffCursor=%#v line=%q",
		func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(), func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc})
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string {
		lines := strings.Split(m.diffView.Render(nil), "\n")
		return strings.Join(lines[1:len(lines)-2], "\n")
	}(), origin,
		"cancel=%#v want=%#v", func() string {
			lines := strings.Split(m.diffView.Render(nil), "\n")
			return strings.Join(lines[1:len(lines)-2], "\n")
		}(), origin,
	)
}

func TestTabTogglesDefaultBranchAndIgnoresStaleRefresh(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize27 := layout.size
		if inlineLocalSize27.Width <= 0 || inlineLocalSize27.Height <= 0 {
			inlineLocalSize27 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize27.Width, height: inlineLocalSize27.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.currentPatch.Branch = "main"
	m.refresh = func(kind patch.Kind) (patch.Patch, error) {
		p := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		p.Kind, p.Branch = kind, "main"
		return p, nil
	}
	next, _ := m.Update(func() tea.KeyPressMsg {
		var text string = "tab"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() model {
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.kind, patch.Branch, "kind=%v", m.kind)

	stale := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
	stale.Files[0].Metadata = []string{"stale"}
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(refreshDiffMsg{patch: stale})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t,
		!slices.Equal(
			m.currentPatch.Files[0].Metadata,
			[]string{"stale"},
		), true,
		"%s", "stale refresh applied",
	)

	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "tab"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		"kind=%v after toggling back to local", m.kind,
	)
}

func TestTabDoesNothingWithoutDefaultBranch(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize17 := layout.size
		if inlineLocalSize17.Width <= 0 || inlineLocalSize17.Height <= 0 {
			inlineLocalSize17 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize17.Width, height: inlineLocalSize17.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	refreshed := false
	m.refresh = func(patch.Kind) (patch.Patch, error) {
		refreshed = true
		return patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}, nil
	}
	next, cmd := m.Update(func() tea.KeyPressMsg {
		var text string = "tab"
		runes := []rune(text)
		return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
	}())
	m = func() model {
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, cmd == nil, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	testutil.RequireEqual(t, !refreshed, true,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
	testutil.RequireEqual(t, m.kind, patch.Unstaged,
		"tab changed model without default branch: cmd=%v refreshed=%v kind=%v",
		cmd != nil, refreshed, m.kind,
	)
}

func TestDiffRefreshFallbackAndEmptyDiff(t *testing.T) {
	m := func() model {
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize23 := layout.size
		if inlineLocalSize23.Width <= 0 || inlineLocalSize23.Height <= 0 {
			inlineLocalSize23 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: patch.Patch{}, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize23.Width, height: inlineLocalSize23.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: patch.Patch{}.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(patch.Patch{}, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = refreshDiffMsg{patch: patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}}
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "initial focus=%q",
		func() string { _, line, _ := m.diffView.Current(); return line.Text }(),
	)

	func() {
		var (
			m    *model = &m
			text string = "new()"
		)
		t.Helper()
		m.diffView.Move(diffscreen.FirstLine)
		count := 0
		for _, file := range m.currentPatch.Files {
			for _, hunk := range file.Hunks {
				count += len(hunk.Lines)
			}
		}
		for range count {
			if func() string { _, line, _ := (*m).diffView.Current(); return line.Text }() == text {
				return
			}
			m.diffView.Move(diffscreen.NextLine)
		}
		t.Fatalf("line %q not found", text)
	}()
	changed := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
	changed.Files[0].Hunks[0].Lines[2].Text = "different()"
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(refreshDiffMsg{patch: changed})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	if _, _, ok := m.diffView.Current(); !ok {
		t.Fatal("refresh lost focus")
	}
}

func TestCommentAfterRefreshUsesCurrentPatch(t *testing.T) {
	t.Setenv("EDITOR", "true")
	var saved patch.Patch
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize25 := layout.size
		if inlineLocalSize25.Width <= 0 || inlineLocalSize25.Height <= 0 {
			inlineLocalSize25 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize25.Width, height: inlineLocalSize25.Height, save: func(stored comments.Comment, p patch.Patch) (comments.Comment, error) { saved = p; return stored, nil }, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	refreshed := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
	refreshed.Files[0].Metadata = []string{"refreshed"}
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(refreshDiffMsg{patch: refreshed})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "c"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	_ = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(commentEditorFinishedMsg{body: "comment"})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t,
		slices.Equal(saved.Files[0].Metadata, []string{"refreshed"}), true,
		"metadata=%q", saved.Files[0].Metadata,
	)
}

func TestViewPreservesTerminalColors(t *testing.T) {
	result := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize6 := layout.size
		if inlineLocalSize6.Width <= 0 || inlineLocalSize6.Height <= 0 {
			inlineLocalSize6 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize6.Width, height: inlineLocalSize6.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}().View()
	testutil.RequireEqual(t, result.BackgroundColor, nil, "view=%#v", result)
	testutil.RequireEqual(t, result.ForegroundColor, nil, "view=%#v", result)
	testutil.RequireEqual(t, result.AltScreen, true, "view=%#v", result)
	testutil.RequireEqual(t, result.ReportFocus, true, "view=%#v", result)
	testutil.RequireEqual(t, result.MouseMode, tea.MouseModeCellMotion, "%s",
		"view does not capture terminal wheel events",
	)
}

func TestMouseWheelScrollsContentWithoutMovingFocus(t *testing.T) {
	m := func() model {
		var p patch.Patch = func() patch.Patch {
			lines := make([]patch.Line, 30)
			for index := range lines {
				lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
			}
			return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
		}()
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize22 := layout.size
		if inlineLocalSize22.Width <= 0 || inlineLocalSize22.Height <= 0 {
			inlineLocalSize22 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize22.Width, height: inlineLocalSize22.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 37, Height: 9})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "v"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = func() tea.KeyPressMsg {
				var text string = "j"
				runes := []rune(text)
				return tea.KeyPressMsg(tea.Key{Text: text, Code: runes[0]})
			}()
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	_, focus, _ := m.diffView.Current()
	_, selected, _ := m.diffView.Selected()
	initial := func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}()
	for _, button := range []tea.MouseButton{
		tea.MouseWheelDown,
		tea.MouseWheelRight,
	} {
		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.MouseWheelMsg{Button: button})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
	}
	testutil.RequireEqual(t, func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}() != initial, true, "%s",
		"wheel did not scroll content",
	)
	testutil.RequireEqual(t, strings.Contains(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "%)"), true, "%s",
		"wheel did not scroll content",
	)

	_, got, _ := m.diffView.Current()
	_, gotSelected, _ := m.diffView.Selected()
	testutil.RequireEqual(t, got, focus, "%s",
		"wheel changed focus or selection",
	)
	testutil.RequireEqual(t, slices.Equal(selected, gotSelected), true, "%s",
		"wheel changed focus or selection",
	)

	for _, button := range []tea.MouseButton{
		tea.MouseWheelUp,
		tea.MouseWheelLeft,
	} {
		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.MouseWheelMsg{Button: button})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
	}
	testutil.RequireEqual(t, func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), initial, "%s",
		"reverse wheel did not restore content",
	)

	for _, inlineLocalMode1 := range []mode{modeComments, modeHelp} {
		m.mode = inlineLocalMode1
		before := m.diffView.Render(nil)
		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		testutil.RequireEqual(t, m.diffView.Render(nil), before, "%s",
			"wheel scrolled diff behind another screen",
		)
	}
}
