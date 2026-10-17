package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"
)

type saveCommentFunc func(
	comments.Comment,
	patch.Patch,
) (comments.Comment, error)
type deleteCommentFunc func(comments.Comment, patch.Patch) error
type loadCommentsFunc func() ([]comments.Comment, error)
type refreshDiffFunc func(patch.Kind) (patch.Patch, error)
type saveSideBySideFunc func(bool) error

// commentStore supplies the persistence operations used by the terminal client.
type commentStore interface {
	Add(comments.Comment) (comments.Comment, error)
	List(string) ([]comments.Comment, error)
	Update(comments.Comment) error
	Delete(string, string) error
}

type size struct {
	Width  int
	Height int
}

type initialLayout struct {
	SideBySide     bool
	SaveSideBySide saveSideBySideFunc
	size           size
}

type refreshDiffMsg struct {
	patch patch.Patch
	kind  patch.Kind
	err   error
}

type commentEditorFinishedMsg struct {
	body string
	err  error
}
type commentsLoadedMsg struct {
	comments []comments.Comment
	revision uint64
	err      error
}
type sourceEditorFinishedMsg struct{ err error }

type mode uint8

const (
	modeBrowse mode = iota
	modeComments
	modeHelp
	modeSearch
)

const horizontalScrollStep = 4
const verticalScrollStep = 3

var defaultSize = size{Width: 80, Height: 30}

// commentState pairs stored comments with the revision used to reject stale
// loads.
type commentState struct {
	items    []comments.Comment
	revision uint64
}

type commentEdit struct {
	body   string
	index  int
	anchor comments.Anchor
}

type model struct {
	ctx context.Context

	diffView    *diffscreen.View
	commentView *commentscreen.View
	width       int
	height      int
	mode        mode
	keys        keymap.Matcher

	currentPatch patch.Patch
	kind         patch.Kind
	diffOptions  diffscreen.Options
	comments     commentState
	edit         commentEdit

	save       saveCommentFunc
	delete     deleteCommentFunc
	load       loadCommentsFunc
	refresh    refreshDiffFunc
	saveLayout saveSideBySideFunc
	err        error
	quitting   bool
}

func (model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.diffOptions.Dark {
			m.diffOptions.Dark = dark
			m.diffView.Configure(m.diffOptions)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)

	case tea.KeyPressMsg:
		m.diffView.EndDrag()
		var m model = m
		name := msg.String()
		if m.mode == modeComments {
			var m model = m
			m.err = nil
			switch name {
			case "esc", "C", "q":
				m.mode = modeBrowse
			case "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "j", "down":
				m.commentView.Move(1)
			case "k", "up":
				m.commentView.Move(-1)
			case "enter", "e":
				var m model = m
				selected, ok := m.commentView.Selected()
				if !ok {
					return m, nil
				}
				m.edit.index = func() int {
					for index, item := range m.comments.items {
						if selected.ID != "" && item.ID == selected.ID || selected.ID == "" && item.Body == selected.Body && item.Anchor.FilePath == selected.Anchor.FilePath {
							return index
						}
					}
					return -1
				}()
				if m.edit.index < 0 {
					return m, nil
				}
				m.edit.body = m.comments.items[m.edit.index].Body
				m.edit.anchor = m.comments.items[m.edit.index].Anchor
				cmd, err := editor.EditComment(m.ctx, m.edit.body, m.edit.anchor, func(body string, err error) tea.Msg { return commentEditorFinishedMsg{body: body, err: err} })
				if err != nil {
					m.err = err
					m.edit = commentEdit{index: -1}
					return m, nil
				}
				return m, cmd
			case "D":
				if selected, ok := m.commentView.Selected(); ok {
					func(m *model, index int) {
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
					}(&m, func() int {
						for index, item := range m.comments.items {
							if selected.ID != "" && item.ID == selected.ID || selected.ID == "" && item.Body == selected.Body && item.Anchor.FilePath == selected.Anchor.FilePath {
								return index
							}
						}
						return -1
					}())
				}
			default:
				return m, nil
			}
			return m, nil
		}
		if m.mode == modeHelp {
			if name == "esc" || name == "?" || name == "q" {
				m.mode = modeBrowse
			}
			return m, nil
		}
		if m.mode == modeSearch {
			var m model = m
			switch name {
			case "esc":
				m.diffView.CancelSearch()
				m.mode = modeBrowse
			case "enter":
				m.diffView.AcceptSearch()
				m.mode = modeBrowse
			case "backspace":
				m.diffView.BackspaceSearch()
			default:
				if msg.Text != "" {
					m.diffView.InsertSearch(msg.Text)
				}
			}
			return m, nil
		}
		{
			var (
				m    model  = m
				name string = name
			)
			m.err = nil
			name = m.keys.Feed(name)
			if func() bool {
				var m *model = &m
				if motion, ok := navigationMotions[name]; ok {
					m.diffView.Move(motion)
					return true
				}
				if alignment, ok := navigationAlignments[name]; ok {
					m.diffView.Align(alignment)
					return true
				}
				switch name {
				case "n":
					m.diffView.Find(diffscreen.Forward)
				case "N":
					m.diffView.Find(diffscreen.Backward)
				case "h", "left":
					m.diffView.ScrollHorizontal(-horizontalScrollStep)
				case "l", "right":
					m.diffView.ScrollHorizontal(horizontalScrollStep)
				case "0":
					m.diffView.ScrollHorizontal(-int(^uint(0) >> 1))
				case "$":
					m.diffView.ScrollHorizontal(int(^uint(0) >> 1))
				default:
					return false
				}
				return true
			}() {
				return m, nil
			}
			switch name {
			case "ctrl+c", "q":
				m.quitting = true
				return m, tea.Quit
			case "c":
				var m model = m
				cmd, err := func() (tea.Cmd, error) {
					file, lines, ok := m.diffView.Selected()
					if !ok {
						return nil, errors.New("select code lines before commenting")
					}
					anchor := func() comments.Anchor {
						path := file.NewPath
						if path == "" {
							path = file.OldPath
						}
						anchor := comments.Anchor{FilePath: path}
						for _, line := range lines {
							prefix := " "
							if line.Kind == patch.Addition {
								prefix = "+"
							}
							if line.Kind == patch.Deletion {
								prefix = "-"
							}
							anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
							func() {
								var inlineLocalStart1, end *int = &anchor.OldStart, &anchor.OldEnd
								if int(line.OldNumber) == 0 {
									return
								}
								if *inlineLocalStart1 == 0 || int(line.OldNumber) < *inlineLocalStart1 {
									*inlineLocalStart1 = int(line.OldNumber)
								}
								if int(line.OldNumber) > *end {
									*end = int(line.OldNumber)
								}
							}()
							func() {
								var inlineLocalStart2, end *int = &anchor.NewStart, &anchor.NewEnd
								if int(line.NewNumber) == 0 {
									return
								}
								if *inlineLocalStart2 == 0 || int(line.NewNumber) < *inlineLocalStart2 {
									*inlineLocalStart2 = int(line.NewNumber)
								}
								if int(line.NewNumber) > *end {
									*end = int(line.NewNumber)
								}
							}()
						}
						return anchor
					}()
					m.edit.body, m.edit.index, m.edit.anchor = "", -1, anchor
					cmd, err := editor.EditComment(m.ctx, m.edit.body, m.edit.anchor, func(body string, err error) tea.Msg { return commentEditorFinishedMsg{body: body, err: err} })
					if err != nil {
						m.edit = commentEdit{index: -1}
						return nil, err
					}
					return cmd, nil
				}()
				if err != nil {
					m.err = err
					return m, nil
				}
				return m, cmd
			case "e":
				var m model = m
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
				if err != nil {
					m.err = err
					return m, nil
				}
				return m, cmd
			case "R":
				return m, func() tea.Cmd {
					if m.refresh == nil {
						return nil
					}
					kind := m.kind
					return func() tea.Msg { p, err := m.refresh(kind); return refreshDiffMsg{patch: p, kind: kind, err: err} }
				}()
			case "tab":
				var m model = m
				if m.currentPatch.Branch == "" {
					return m, nil
				}
				if m.kind == patch.Unstaged {
					m.kind = patch.Branch
				} else {
					m.kind = patch.Unstaged
				}
				m.diffView.ClearSelection()
				return m, func() tea.Cmd {
					if m.refresh == nil {
						return nil
					}
					kind := m.kind
					return func() tea.Msg { p, err := m.refresh(kind); return refreshDiffMsg{patch: p, kind: kind, err: err} }
				}()
			default:
				var m model = m
				switch name {
				case "?":
					m.mode = modeHelp
				case "/":
					m.mode = modeSearch
					m.diffView.BeginSearch()
				case "v":
					m.diffView.ToggleSelection()
				case "esc":
					m.diffView.ClearSelection()
				case "C":
					m.mode = modeComments
					m.commentView.Update(m.comments.items)
					return m, func() tea.Cmd {
						if m.load == nil {
							return nil
						}
						revision := m.comments.revision
						return func() tea.Msg {
							inlineLocalComments1, err := m.load()
							return commentsLoadedMsg{comments: inlineLocalComments1, revision: revision, err: err}
						}
					}()
				case "t":
					enabled := !m.diffOptions.SideBySide
					m.diffOptions.SideBySide = enabled
					m.diffView.Configure(m.diffOptions)
					if m.saveLayout != nil {
						if err := m.saveLayout(m.diffOptions.SideBySide); err != nil {
							m.err = fmt.Errorf("save side-by-side preference: %w", err)
						}
					}
				default:
					return m, nil
				}
				return m, nil
			}
		}
	case tea.MouseClickMsg, tea.MouseMotionMsg,
		tea.MouseReleaseMsg, tea.MouseWheelMsg:
		func() {
			var m *model = &m
			switch msg := msg.(type) {
			case tea.MouseClickMsg:
				func() {
					if msg.Button != tea.MouseLeft {
						return
					}
					switch m.mode {
					case modeBrowse:
						m.diffView.BeginDrag(msg.X, msg.Y)
					case modeComments:
						m.commentView.Click(msg.X, msg.Y)
					default:
						return
					}
				}()
			case tea.MouseMotionMsg:
				if msg.Button == tea.MouseLeft && m.mode == modeBrowse {
					m.diffView.DragTo(msg.X, msg.Y)
				}
			case tea.MouseReleaseMsg:
				func() {
					if msg.Button != tea.MouseLeft && msg.Button != tea.MouseNone {
						return
					}
					if m.mode == modeBrowse {
						m.diffView.DragTo(msg.X, msg.Y)
					}
					m.diffView.EndDrag()
				}()
			case tea.MouseWheelMsg:
				func() {
					m.diffView.EndDrag()
					if m.mode != modeBrowse && m.mode != modeSearch {
						return
					}
					switch msg.Button {
					case tea.MouseWheelUp:
						m.diffView.ScrollVertical(-verticalScrollStep)
					case tea.MouseWheelDown:
						m.diffView.ScrollVertical(verticalScrollStep)
					case tea.MouseWheelLeft:
						m.diffView.ScrollHorizontal(-horizontalScrollStep)
					case tea.MouseWheelRight:
						m.diffView.ScrollHorizontal(horizontalScrollStep)
					default:
						return
					}
				}()
			default:
				return
			}
		}()
	default:
		var m model = m
		switch msg := msg.(type) {
		case commentEditorFinishedMsg:
			func() {
				var m *model = &m
				if msg.err != nil {
					m.err = msg.err
					m.edit = commentEdit{index: -1}
					return
				}
				m.edit.body = msg.body
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
			}()
		case commentsLoadedMsg:
			func() {
				if msg.revision != m.comments.revision {
					return
				}
				if msg.err != nil {
					m.err = fmt.Errorf("refresh comments: %w", msg.err)
					return
				}
				m.comments.items = msg.comments
				m.commentView.Update(m.comments.items)
				m.err = nil
			}()
		case sourceEditorFinishedMsg:
			if msg.err != nil {
				m.err = fmt.Errorf("editor: %w", msg.err)
				break
			}
			return m, func() tea.Cmd {
				if m.refresh == nil {
					return nil
				}
				kind := m.kind
				return func() tea.Msg { p, err := m.refresh(kind); return refreshDiffMsg{patch: p, kind: kind, err: err} }
			}()
		case tea.FocusMsg:
			return m, func() tea.Cmd {
				if m.refresh == nil {
					return nil
				}
				kind := m.kind
				return func() tea.Msg { p, err := m.refresh(kind); return refreshDiffMsg{patch: p, kind: kind, err: err} }
			}()
		case refreshDiffMsg:
			func() {
				if msg.kind != m.kind {
					return
				}
				if msg.err != nil {
					m.err = fmt.Errorf("refresh diff: %w", msg.err)
					return
				}
				m.currentPatch = msg.patch
				m.diffView.Update(msg.patch)
				m.err = nil
			}()
		default:
			return m, nil
		}
		return m, nil
	}
	return m, nil
}
