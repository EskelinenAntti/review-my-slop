package app

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/helpscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

func TestMouseDragCreatesCommentSelection(t *testing.T) {
	t.Setenv("EDITOR", "true")
	for _, release := range []tea.MouseButton{tea.MouseLeft, tea.MouseNone} {
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
			next, _ := m.Update(tea.MouseClickMsg{X: 20, Y: 4, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "%s",
			"click did not focus deletion",
		)

		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.MouseMotionMsg{X: 20, Y: 5, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.MouseReleaseMsg{X: 20, Y: 6, Button: release})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		_, lines, ok := m.diffView.Selected()
		testutil.RequireEqual(t, ok, true, "released selection = %#v", lines)
		testutil.RequireEqual(t, len(lines), 3, "released selection = %#v",
			lines,
		)
		testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()",
			"released selection = %#v", lines,
		)

		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.MouseMotionMsg{X: 20, Y: 3, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "keep()", "%s",
			"motion after release changed focus",
		)

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
		testutil.RequireEqual(t, len(m.edit.anchor.QuotedLines), 3,
			"comment anchor = %#v", m.edit.anchor,
		)
		testutil.RequireEqual(t, m.edit.anchor.QuotedLines[0], "-old()",
			"comment anchor = %#v", m.edit.anchor,
		)
	}
}

func TestMouseClickCommentsAndIgnoreInformationalUI(t *testing.T) {
	m := func() model {
		var (
			p     patch.Patch        = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", OldSource: "package main\nold()\nkeep()\n", NewSource: "package main\nnew()\nkeep()\nmore()\n", Hunks: []patch.Hunk{{Header: "@@ -1,3 +1,3 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "package main", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}, {Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}}}, {Header: "@@ -3,1 +3,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 3, NewNumber: 3}, {Kind: patch.Addition, Text: "more()", NewNumber: 4}}}}}}}
			items []comments.Comment = []comments.Comment{{ID: "one"}, {ID: "two"}}
		)
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize2 := layout.size
		if inlineLocalSize2.Width <= 0 || inlineLocalSize2.Height <= 0 {
			inlineLocalSize2 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: items}, commentView: commentscreen.New(items), edit: commentEdit{index: -1}, width: inlineLocalSize2.Width, height: inlineLocalSize2.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
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
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.MouseClickMsg{X: 10, Y: 2, Button: tea.MouseLeft})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	selected, _ := m.commentView.Selected()
	testutil.RequireEqual(t, selected.ID, "two", "%s",
		"click did not focus comment",
	)

	for _, inlineLocalMode1 := range []mode{
		modeBrowse,
		modeComments,
		modeHelp,
		modeSearch,
	} {
		m.mode = inlineLocalMode1
		before := func() string {
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
		m = func() model {
			var m model = m
			t.Helper()
			next, _ := m.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}()
		m = func(m model, msg tea.Msg) model {
			t.Helper()
			next, _ := m.Update(msg)
			t.Helper()
			result, ok := next.(model)
			testutil.RequireEqual(t, ok, true, "model=%T", next)
			return result
		}(m,
			tea.MouseClickMsg{X: 0, Y: m.height - 2, Button: tea.MouseLeft})
		testutil.RequireEqual(t, m.mode, inlineLocalMode1, "%s",
			"header/footer click triggered an action",
		)
		testutil.RequireEqual(t, !m.quitting, true, "%s",
			"header/footer click triggered an action",
		)
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
		}(), before, "%s",
			"header/footer click triggered an action",
		)
	}
}

func TestMouseIgnoresOtherButtonsAndKeyboardCancelsDrag(t *testing.T) {
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
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.MouseClickMsg{X: 10, Y: 5, Button: tea.MouseRight})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "%s",
		"right click moved focus",
	)

	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "package main", "%s",
		"motion without a press moved focus",
	)

	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.MouseClickMsg{X: 10, Y: 3, Button: tea.MouseLeft})
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
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.MouseMotionMsg{X: 10, Y: 5, Button: tea.MouseLeft})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, func() string { _, line, _ := m.diffView.Current(); return line.Text }(), "old()", "%s",
		"keyboard action did not cancel drag",
	)
}
