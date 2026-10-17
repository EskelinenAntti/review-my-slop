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
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/helpscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

func TestStoreBackedCommentLifecycle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	store := comments.Store{Path: filepath.Join(t.TempDir(), "comments.db")}
	m, err := func() (model, error) {
		var (
			p                patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			inlineLocalSize4 size        = defaultSize
		)
		preferences, err := settings.Load()
		if err != nil {
			return model{}, err
		}
		m := func() model {
			var layout initialLayout = initialLayout{SideBySide: preferences.SideBySide, SaveSideBySide: func(enabled bool) error { return settings.Save(settings.Preferences{SideBySide: enabled}) }, size: inlineLocalSize4}
			inlineLocalSize11 := layout.size
			if inlineLocalSize11.Width <= 0 || inlineLocalSize11.Height <= 0 {
				inlineLocalSize11 = defaultSize
			}
			m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize11.Width, height: inlineLocalSize11.Height, save: func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
				comment.Repository = current.Root
				if comment.ID != "" {
					if err := commentStore(store).Update(comment); err != nil {
						return comments.Comment{}, err
					}
					return comment, nil
				}
				return commentStore(store).Add(comment)
			}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
			m.diffView = diffscreen.New(p, m.diffOptions)
			m.diffView.Resize(m.width, m.height)
			m.commentView.Resize(m.width, m.height)
			return m
		}()
		m.delete = func(comment comments.Comment, current patch.Patch) error {
			return commentStore(store).Delete(current.Root, comment.ID)
		}
		m.load = func() ([]comments.Comment, error) { return commentStore(store).List(p.Root) }
		return m, nil
	}()
	testutil.NoError(t, err)

	anchor := comments.Anchor{
		FilePath:    "main.go",
		NewStart:    2,
		QuotedLines: []string{"+new()"},
	}
	m.edit.body = "check this"
	m.edit.anchor = anchor
	func() {
		body := strings.TrimSpace(m.edit.body)
		if body == "" {
			if m.edit.index >= 0 {
				func() {
					var index int = m.edit.index
					if index < 0 || index >= len(m.comments.items) {
						return
					}
					if m.delete == nil {
						m.err = errors.New("comment storage is unavailable")
						return
					}
					if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
						m.err = err
						return
					}
					m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
					m.commentView.Update(m.comments.items)
					m.comments.revision++
					m.err = nil
				}()
			}
			m.edit = commentEdit{index: -1}
			m.diffView.ClearSelection()
			return
		}
		if m.save == nil {
			m.err = errors.New("comment storage is unavailable")
			m.edit = commentEdit{index: -1}
			return
		}
		var comment comments.Comment
		if m.edit.index >= 0 {
			comment = m.comments.items[m.edit.index]
			comment.Body = body
		} else {
			comment = comments.Comment{Anchor: m.edit.anchor, Body: body}
		}
		saved, err := m.save(comment, m.currentPatch)
		if err != nil {
			m.err = err
			m.edit = commentEdit{index: -1}
			return
		}
		if m.edit.index >= 0 {
			m.comments.items[m.edit.index] = saved
		} else {
			m.comments.items = append(m.comments.items, saved)
			m.commentView.Update(m.comments.items)
			m.commentView.Move(len(m.comments.items))
		}
		m.comments.revision++
		m.commentView.Update(m.comments.items)
		m.edit = commentEdit{index: -1}
		m.err = nil
		m.diffView.ClearSelection()
	}()
	testutil.RequireEqual(t, m.err, nil, "%s", m.err)

	loaded, err := m.load()
	testutil.RequireEqual(t, err, nil, "loaded = %#v, error = %v", loaded, err)
	testutil.RequireEqual(t, len(loaded), 1, "loaded = %#v, error = %v", loaded,
		err,
	)
	testutil.RequireEqual(t, loaded[0].Repository, "/repo",
		"loaded = %#v, error = %v", loaded, err,
	)
	testutil.RequireEqual(t, loaded[0].ID != "", true,
		"loaded = %#v, error = %v", loaded, err,
	)
	testutil.RequireEqual(t,
		slices.Equal(loaded[0].Anchor.QuotedLines, anchor.QuotedLines), true,
		"loaded = %#v, error = %v", loaded, err,
	)

	id := loaded[0].ID
	m.edit.index = 0
	m.edit.body = "edited"
	func() {
		body := strings.TrimSpace(m.edit.body)
		if body == "" {
			if m.edit.index >= 0 {
				func() {
					var index int = m.edit.index
					if index < 0 || index >= len(m.comments.items) {
						return
					}
					if m.delete == nil {
						m.err = errors.New("comment storage is unavailable")
						return
					}
					if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
						m.err = err
						return
					}
					m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
					m.commentView.Update(m.comments.items)
					m.comments.revision++
					m.err = nil
				}()
			}
			m.edit = commentEdit{index: -1}
			m.diffView.ClearSelection()
			return
		}
		if m.save == nil {
			m.err = errors.New("comment storage is unavailable")
			m.edit = commentEdit{index: -1}
			return
		}
		var comment comments.Comment
		if m.edit.index >= 0 {
			comment = m.comments.items[m.edit.index]
			comment.Body = body
		} else {
			comment = comments.Comment{Anchor: m.edit.anchor, Body: body}
		}
		saved, err := m.save(comment, m.currentPatch)
		if err != nil {
			m.err = err
			m.edit = commentEdit{index: -1}
			return
		}
		if m.edit.index >= 0 {
			m.comments.items[m.edit.index] = saved
		} else {
			m.comments.items = append(m.comments.items, saved)
			m.commentView.Update(m.comments.items)
			m.commentView.Move(len(m.comments.items))
		}
		m.comments.revision++
		m.commentView.Update(m.comments.items)
		m.edit = commentEdit{index: -1}
		m.err = nil
		m.diffView.ClearSelection()
	}()
	testutil.RequireEqual(t, m.err, nil, "%s", m.err)

	loaded, err = m.load()
	testutil.RequireEqual(t, err, nil, "edited = %#v, error = %v", loaded, err)
	testutil.RequireEqual(t, len(loaded), 1, "edited = %#v, error = %v", loaded,
		err,
	)
	testutil.RequireEqual(t, loaded[0].ID, id, "edited = %#v, error = %v",
		loaded, err,
	)
	testutil.RequireEqual(t, loaded[0].Body, "edited",
		"edited = %#v, error = %v", loaded, err,
	)

	func() {
		if 0 < 0 || 0 >= len(m.comments.items) {
			return
		}
		if m.delete == nil {
			m.err = errors.New("comment storage is unavailable")
			return
		}
		if err := m.delete(m.comments.items[0], m.currentPatch); err != nil {
			m.err = err
			return
		}
		m.comments.items = append(m.comments.items[:0], m.comments.items[0+1:]...)
		m.commentView.Update(m.comments.items)
		m.comments.revision++
		m.err = nil
	}()
	testutil.RequireEqual(t, m.err, nil, "%s", m.err)

	loaded, err = m.load()
	testutil.RequireEqual(t, err, nil, "remaining = %#v, error = %v", loaded,
		err,
	)
	testutil.RequireEqual(t, len(loaded), 0, "remaining = %#v, error = %v",
		loaded, err,
	)
}

func TestNewUsesSavedSideBySideForWideInitialSize(t *testing.T) {
	m := func() model {
		var (
			p      patch.Patch   = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			layout initialLayout = initialLayout{
				SideBySide: true,
				size:       size{Width: 120, Height: 30},
			}
		)
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
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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
	}(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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

func TestNewKeepsSavedSideBySideInactiveForNarrowInitialSize(t *testing.T) {
	m := func() model {
		var (
			p      patch.Patch   = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			layout initialLayout = initialLayout{
				SideBySide: true,
				size:       size{Width: 80, Height: 30},
			}
		)
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
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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
	}(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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

	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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
	}(), "│"), true,
		"sideBySide=%v render=%q", m.diffOptions.SideBySide, func() string {
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

func TestSideBySideToggleStillSavesPreference(t *testing.T) {
	var saved []bool
	m := func() model {
		var (
			p      patch.Patch   = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			layout initialLayout = initialLayout{
				SaveSideBySide: func(enabled bool) error {
					saved = append(saved, enabled)
					return nil
				},
				size: size{Width: 120, Height: 30},
			}
		)
		inlineLocalSize7 := layout.size
		if inlineLocalSize7.Width <= 0 || inlineLocalSize7.Height <= 0 {
			inlineLocalSize7 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize7.Width, height: inlineLocalSize7.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
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
	func() model {
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
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		"saved=%v", saved,
	)
}

func TestRefreshTranslatesCursorAndSelection(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
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
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	refreshed := patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
	refreshed.Files[0].Metadata = []string{"new metadata"}
	m.currentPatch = refreshed
	m.diffView.Update(refreshed)
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	testutil.RequireEqual(t, got, want, "focused line=%#v, want %#v", got, want)

	_, lines, ok := m.diffView.Selected()
	testutil.RequireEqual(t, ok, true, "selection=%#v", lines)
	testutil.RequireEqual(t, len(lines), 2, "selection=%#v", lines)
}

func TestUnchangedRefreshPreservesStateAndClearsError(t *testing.T) {
	p := func() patch.Patch {
		lines := make([]patch.Line, 30)
		for index := range lines {
			lines[index] = patch.Line{Kind: patch.Context, Text: fmt.Sprintf("line %d %s", index, strings.Repeat("x", 80)), OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)}
		}
		return patch.Patch{Files: []patch.File{{DisplayPath: "long.go", Hunks: []patch.Hunk{{Header: "@@", Lines: lines}}}}}
	}()
	p.Files[0].OldPath, p.Files[0].NewPath = "long.go", "long.go"
	m := func() model {
		var layout initialLayout = initialLayout{size: size{Width: 40, Height: 8}}
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
	for range 8 {
		m.diffView.Move(diffscreen.NextLine)
	}
	m.diffView.ToggleSelection()
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.ScrollHorizontal(12)
	before := m.diffView.Render(nil)
	m.err = errors.New("previous refresh failed")
	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(refreshDiffMsg{patch: p})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	if got := m.diffView.Render(nil); got != before {
		t.Fatal("refresh changed presentation state")
	}
	testutil.RequireEqual(t, m.err, nil, "refresh did not clear error: %v",
		m.err,
	)
}

func TestRefreshFailureRetainsView(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
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
	before := m.diffView.Render(nil)
	m = func() model {
		var (
			m   model   = m
			msg tea.Msg = refreshDiffMsg{err: errors.New("git failed")}
		)
		t.Helper()
		next, _ := m.Update(msg)
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	testutil.RequireEqual(t, m.err != nil, true, "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
	testutil.RequireEqual(t, m.diffView.Render(nil), before,
		"patch=%#v error=%v", m.currentPatch, m.err,
	)
	testutil.RequireEqual(t, m.currentPatch.Root, "/repo", "patch=%#v error=%v",
		m.currentPatch, m.err,
	)
}

func TestViewSwitchPreservesSemanticCursor(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
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
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
		t.Helper()
		result, ok := next.(model)
		testutil.RequireEqual(t, ok, true, "model=%T", next)
		return result
	}()
	m.diffView.Move(diffscreen.NextLine)
	m.diffView.Move(diffscreen.NextLine)
	_, want, _ := m.diffView.Current()
	m.diffOptions.SideBySide = true
	m.diffView.Configure(m.diffOptions)
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true, "focused line=%#v, want %#v", got, want)
	testutil.RequireEqual(t, got, want, "focused line=%#v, want %#v", got, want)
}

func TestCommentSaveUsesPatchAndPreservesAnchor(t *testing.T) {
	var savedPatch patch.Patch
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize15 := layout.size
		if inlineLocalSize15.Width <= 0 || inlineLocalSize15.Height <= 0 {
			inlineLocalSize15 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize15.Width, height: inlineLocalSize15.Height, save: func(stored comments.Comment, p patch.Patch) (comments.Comment, error) {
			savedPatch = p
			stored.ID = "1"
			return stored, nil
		}, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.edit.body = "comment"
	m.edit.anchor = comments.Anchor{FilePath: "main.go"}
	func() {
		body := strings.TrimSpace(m.edit.body)
		if body == "" {
			if m.edit.index >= 0 {
				func() {
					var index int = m.edit.index
					if index < 0 || index >= len(m.comments.items) {
						return
					}
					if m.delete == nil {
						m.err = errors.New("comment storage is unavailable")
						return
					}
					if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
						m.err = err
						return
					}
					m.comments.items = append(m.comments.items[:index], m.comments.items[index+1:]...)
					m.commentView.Update(m.comments.items)
					m.comments.revision++
					m.err = nil
				}()
			}
			m.edit = commentEdit{index: -1}
			m.diffView.ClearSelection()
			return
		}
		if m.save == nil {
			m.err = errors.New("comment storage is unavailable")
			m.edit = commentEdit{index: -1}
			return
		}
		var comment comments.Comment
		if m.edit.index >= 0 {
			comment = m.comments.items[m.edit.index]
			comment.Body = body
		} else {
			comment = comments.Comment{Anchor: m.edit.anchor, Body: body}
		}
		saved, err := m.save(comment, m.currentPatch)
		if err != nil {
			m.err = err
			m.edit = commentEdit{index: -1}
			return
		}
		if m.edit.index >= 0 {
			m.comments.items[m.edit.index] = saved
		} else {
			m.comments.items = append(m.comments.items, saved)
			m.commentView.Update(m.comments.items)
			m.commentView.Move(len(m.comments.items))
		}
		m.comments.revision++
		m.commentView.Update(m.comments.items)
		m.edit = commentEdit{index: -1}
		m.err = nil
		m.diffView.ClearSelection()
	}()
	testutil.RequireEqual(t, savedPatch.Root, "/repo",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	testutil.RequireEqual(t, len(m.comments.items), 1,
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
	testutil.RequireEqual(t, m.comments.items[0].Anchor.FilePath, "main.go",
		"saved patch/comments = %#v %#v", savedPatch, m.comments.items,
	)
}

func TestRenderingAndKeyBindingsRemainAvailable(t *testing.T) {
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize12 := layout.size
		if inlineLocalSize12.Width <= 0 || inlineLocalSize12.Height <= 0 {
			inlineLocalSize12 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize12.Width, height: inlineLocalSize12.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	rendered := func() string {
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
	for _, value := range []string{
		"review-my-slop",
		"+1-1",
		"old()",
		"new()",
		"local changes",
	} {
		testutil.RequireEqual(t, strings.Contains(rendered, value), true,
			"render missing %q: %q", value, rendered,
		)
	}
	m.mode = modeHelp
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
	}(), "Ctrl-w h/l/w"), true,
		"%s", "help lost diffPane binding",
	)
}

func TestEmptyViewKeepsKeyboardHintAtBottom(t *testing.T) {
	m := func() model {
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize14 := layout.size
		if inlineLocalSize14.Width <= 0 || inlineLocalSize14.Height <= 0 {
			inlineLocalSize14 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: patch.Patch{}, comments: commentState{items: nil}, commentView: commentscreen.New(nil), edit: commentEdit{index: -1}, width: inlineLocalSize14.Width, height: inlineLocalSize14.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: patch.Patch{}.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(patch.Patch{}, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)

	lines := strings.Split(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}(), "\n")
	if got, want := lines[m.height-2], "j/k/h/l move"; !strings.Contains(
		got,
		want,
	) {
		t.Fatalf("line %d = %q, want it to contain %q", m.height-1, got, want)
	}
	if got := lines[2]; !strings.Contains(
		got,
		"No unstaged or untracked changes.",
	) {
		t.Fatalf("empty-viewState line = %q", got)
	}
}

func TestMenuKeyboardHintsStayAtBottom(t *testing.T) {
	m := func() model {
		var (
			p     patch.Patch        = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			items []comments.Comment = []comments.Comment{
				{
					Body:   "first",
					Anchor: comments.Anchor{FilePath: "main.go", NewStart: 2},
				},
			}
		)
		var layout initialLayout = initialLayout{size: size{Width: 100, Height: 30}}
		inlineLocalSize6 := layout.size
		if inlineLocalSize6.Width <= 0 || inlineLocalSize6.Height <= 0 {
			inlineLocalSize6 = defaultSize
		}
		m := model{ctx: context.Background(), currentPatch: p, comments: commentState{items: items}, commentView: commentscreen.New(items), edit: commentEdit{index: -1}, width: inlineLocalSize6.Width, height: inlineLocalSize6.Height, save: nil, saveLayout: layout.SaveSideBySide, diffOptions: diffscreen.Options{SideBySide: layout.SideBySide, Dark: true}, kind: p.Kind, keys: keymap.New(keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true}, keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}}, keymap.Sequence{Prefix: "[", Keys: []string{"f"}}, keymap.Sequence{Prefix: "]", Keys: []string{"f"}}, keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}})}
		m.diffView = diffscreen.New(p, m.diffOptions)
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)
		return m
	}()
	m.width, m.height = 80, 10
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)

	tests := []struct {
		name             string
		inlineLocalMode1 mode
		hint             string
	}{
		{name: "comments", inlineLocalMode1: modeComments, hint: "j/k move"},
		{name: "help", inlineLocalMode1: modeHelp, hint: "? or Esc closes help"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m.mode = test.inlineLocalMode1
			lines := strings.Split(func() string {
				switch m.mode {
				case modeHelp:
					bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
					return helpscreen.Render(bindings, m.width, m.height)
				case modeComments:
					return m.commentView.Render(m.err)
				default:
					return m.diffView.Render(m.err)
				}
			}(), "\n")
			if got := lines[m.height-2]; !strings.Contains(got, test.hint) {
				t.Fatalf(
					"line %d = %q, want it to contain %q",
					m.height-1,
					got,
					test.hint,
				)
			}
		})
	}
}

func TestCommentsMenuScrollsWithinScreenBody(t *testing.T) {
	items := make([]comments.Comment, 10)
	for index := range items {
		items[index] = comments.Comment{
			Body:   fmt.Sprintf("comment %d", index),
			Anchor: comments.Anchor{FilePath: "main.go"},
		}
	}
	m := func() model {
		var p patch.Patch = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
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
	m.width, m.height = 80, 7
	m.mode = modeComments
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	m.commentView.Move(len(items))

	rendered := strings.Split(ansi.Strip(func() string {
		switch m.mode {
		case modeHelp:
			bindings := []helpscreen.Binding{{Keys: "j/k, arrows", Description: "move"}, {Keys: "h/l, left/right", Description: "scroll horizontally"}, {Keys: "Mouse wheel", Description: "scroll content"}, {Keys: "Click/drag", Description: "focus line/select range"}, {Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"}, {Keys: "0/$", Description: "start/end of lines"}, {Keys: "gg/G", Description: "first/last changed line"}, {Keys: "zz/zt/zb", Description: "center/top/bottom current line"}, {Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"}, {Keys: "/", Description: "search diff text"}, {Keys: "n/N", Description: "next/previous search match"}, {Keys: "]f/[f", Description: "next/previous file"}, {Keys: "v", Description: "select a line range"}, {Keys: "c", Description: "comment on selection/current line"}, {Keys: "e", Description: "open current line in $EDITOR"}, {Keys: "C", Description: "view comments"}, {Keys: "R", Description: "refresh diff"}, {Keys: "Tab", Description: "toggle local/branch changes"}, {Keys: "t", Description: "toggle unified/side-by-side"}, {Keys: "q", Description: "quit"}}
			return helpscreen.Render(bindings, m.width, m.height)
		case modeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}()), "\n")
	testutil.RequireEqual(t,
		strings.Contains(
			strings.Join(rendered[1:m.height-2], "\n"),
			"comment 9",
		),
		true, "selected comment is outside the screen body: %q", rendered,
	)
	testutil.RequireEqual(t, strings.Contains(rendered[m.height-2], "j/k move"),
		true, "footer line = %q", rendered[m.height-2],
	)
}

func TestNarrowLayoutToggleSavesPreferenceWithoutApplicationGeometryRules(
	t *testing.T,
) {
	var saved []bool
	m := func() model {
		var (
			p      patch.Patch   = patch.Patch{Root: "/repo", Files: []patch.File{{DisplayPath: "main.go", OldPath: "main.go", NewPath: "main.go", Hunks: []patch.Hunk{{Header: "@@ -1,2 +1,2 @@", Lines: []patch.Line{{Kind: patch.Context, Text: "keep()", OldNumber: 1, NewNumber: 1}, {Kind: patch.Deletion, Text: "old()", OldNumber: 2}, {Kind: patch.Addition, Text: "new()", NewNumber: 2}}}}}}}
			layout initialLayout = initialLayout{
				size: size{Width: 80, Height: 30},
				SaveSideBySide: func(enabled bool) error {
					saved = append(saved, enabled)
					return nil
				},
			}
		)
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
	testutil.RequireEqual(t, m.err, nil,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, func() string {
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
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true}), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, func() string {
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
	}(), " │ "), true,
		"narrow toggle: saved=%v err=%v render=%q", saved, m.err, func() string {
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

	m = func() model {
		var m model = m
		t.Helper()
		next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
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
	}(), " │ "), true, "%s",
		"widening did not activate preferred layout",
	)
}
