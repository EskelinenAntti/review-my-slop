package ui

import (
	"context"
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type saveCommentFunc func(comments.Comment, patch.Patch) (comments.Comment, error)
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

const (
	horizontalScrollStep   = 4
	minimumSideBySideWidth = 100
)

var defaultSize = size{Width: 80, Height: 30}

type reviewState struct {
	patch      patch.Patch
	view       reviewView
	cursor     diffCursor
	viewport   diffViewport
	selection  *diffSelection
	sideBySide bool
}

type commentState struct {
	items      []comments.Comment
	row        int
	body       string
	editIndex  int
	editAnchor comments.Anchor
	revision   uint64
}

type searchState struct {
	query []rune
	term  string
	from  diffCursor
	miss  bool
}

type model struct {
	ctx        context.Context
	review     reviewState
	comments   commentState
	search     searchState
	width      int
	height     int
	mode       mode
	save       saveCommentFunc
	delete     deleteCommentFunc
	load       loadCommentsFunc
	refresh    refreshDiffFunc
	err        error
	quitting   bool
	keys       keymap.Matcher
	saveLayout saveSideBySideFunc
	kind       patch.Kind
	dark       bool
}

func newModel(p patch.Patch, comments []comments.Comment, save saveCommentFunc, layout initialLayout) model {
	size := layout.size
	if size.Width <= 0 || size.Height <= 0 {
		size = defaultSize
	}
	m := model{
		ctx:        context.Background(),
		review:     reviewState{patch: p},
		comments:   commentState{items: comments, editIndex: -1},
		width:      size.Width,
		height:     size.Height,
		save:       save,
		saveLayout: layout.SaveSideBySide,
		dark:       true,
		kind:       p.Kind,
		keys: keymap.New(
			keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true},
			keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
			keymap.Sequence{Prefix: "[", Keys: []string{"f"}},
			keymap.Sequence{Prefix: "]", Keys: []string{"f"}},
			keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}},
		),
	}
	m.review.sideBySide = layout.SideBySide
	m.review.view = m.newReviewView(p)
	m.review.viewport = m.review.view.newViewport(m.width, m.screenBodyHeight())
	m.review.cursor, _ = m.review.view.first()
	return m
}

func newWithStore(store commentStore, p patch.Patch, items []comments.Comment, size size) (model, error) {
	preferences, err := settings.Load()
	if err != nil {
		return model{}, err
	}
	m := newModel(p, items, func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
		comment.Repository = current.Root
		if comment.ID != "" {
			if err := store.Update(comment); err != nil {
				return comments.Comment{}, err
			}
			return comment, nil
		}
		return store.Add(comment)
	}, initialLayout{
		SideBySide:     preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error { return settings.Save(settings.Preferences{SideBySide: enabled}) },
		size:           size,
	})
	m.setDelete(func(comment comments.Comment, current patch.Patch) error {
		return store.Delete(current.Root, comment.ID)
	})
	m.setLoadComments(func() ([]comments.Comment, error) {
		return store.List(p.Root)
	})
	return m, nil
}

func (m *model) setRefresh(refresh refreshDiffFunc)    { m.refresh = refresh }
func (m *model) setDelete(delete deleteCommentFunc)    { m.delete = delete }
func (m *model) setLoadComments(load loadCommentsFunc) { m.load = load }
func (m *model) configureSideBySide(enabled bool, save saveSideBySideFunc) {
	m.saveLayout = save
	m.setSideBySide(enabled)
}

func (m model) Init() tea.Cmd { return func() tea.Msg { return tea.RequestBackgroundColor() } }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.dark {
			m.dark = dark
			m.rebuildView(m.review.patch)
		}
	case tea.WindowSizeMsg:
		activeBefore := m.sideBySideActive()
		m.width, m.height = msg.Width, msg.Height
		m.review.viewport = m.review.view.resize(m.review.viewport, m.width, m.screenBodyHeight())
		if activeBefore != m.sideBySideActive() {
			m.rebuildView(m.review.patch)
		} else {
			m.review.viewport = m.review.view.keepVisible(m.review.viewport, m.review.cursor)
		}
	case commentEditorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.clearCommentEdit()
		} else {
			m.comments.body = msg.body
			m.finishCommentEdit()
		}
	case commentsLoadedMsg:
		if msg.revision != m.comments.revision {
			break
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh comments: %w", msg.err)
		} else {
			m.comments.items = msg.comments
			m.comments.row = min(m.comments.row, max(0, len(m.comments.items)-1))
			m.err = nil
		}
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			break
		}
		return m, m.loadRefresh()
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		if msg.kind != m.kind {
			return m, nil
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", msg.err)
		} else {
			m.rebuildView(msg.patch)
			m.err = nil
		}
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m model) loadRefresh() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg { p, err := m.refresh(kind); return refreshDiffMsg{patch: p, kind: kind, err: err} }
}

func (m model) loadComments() tea.Cmd {
	if m.load == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		comments, err := m.load()
		return commentsLoadedMsg{comments: comments, revision: revision, err: err}
	}
}

func (m *model) rebuildView(p patch.Patch) {
	oldView := m.review.view
	oldState := viewState{
		cursor:    &m.review.cursor,
		selection: m.review.selection,
		viewport:  m.review.viewport,
	}
	m.review.patch = p
	m.review.view = m.newReviewView(p)
	state := preserve(oldView, oldState, m.review.view)
	m.review.viewport = state.viewport
	m.review.selection = state.selection
	if state.cursor != nil {
		m.review.cursor = *state.cursor
	}
}

func (m model) newReviewView(p patch.Patch) reviewView {
	if m.sideBySideActive() {
		return newSideBySideView(p, m.dark)
	}
	return newUnifiedView(p, m.dark)
}

func (m model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	if m.mode == modeComments {
		return m.updateComments(name)
	}
	if m.mode == modeHelp {
		if name == "esc" || name == "?" || name == "q" {
			m.mode = modeBrowse
		}
		return m, nil
	}
	if m.mode == modeSearch {
		return m.updateSearch(name, key)
	}
	m.err = nil
	name = m.keys.Feed(name)
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		m.cancelSelection()
		m.mode = modeSearch
		m.search.query = nil
		m.search.from = m.review.cursor
		m.search.miss = false
	case "n":
		m.repeatSearch(forward)
	case "N":
		m.repeatSearch(backward)
	case "j", "down":
		m.move(forward)
	case "k", "up":
		m.move(backward)
	case "h", "left":
		m.review.viewport = m.review.view.scrollHorizontal(m.review.viewport, -horizontalScrollStep)
	case "l", "right":
		m.review.viewport = m.review.view.scrollHorizontal(m.review.viewport, horizontalScrollStep)
	case "0":
		m.review.viewport.LeftColumn = 0
	case "$":
		m.review.viewport = m.review.view.scrollHorizontal(m.review.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(forward)
	case "ctrl+u":
		m.halfPage(backward)
	case "g g":
		if cursor, ok := m.review.view.first(); ok {
			m.setCursor(cursor)
		}
	case "G":
		if cursor, ok := m.review.view.last(); ok {
			m.setCursor(cursor)
		}
	case "] f":
		m.jumpFile(forward)
	case "[ f":
		m.jumpFile(backward)
	case "z z":
		m.review.viewport = m.review.view.align(m.review.viewport, m.review.cursor, middle)
	case "z t":
		m.review.viewport = m.review.view.align(m.review.viewport, m.review.cursor, top)
	case "z b":
		m.review.viewport = m.review.view.align(m.review.viewport, m.review.cursor, bottom)
	case "ctrl+w h":
		m.switchPane(left)
	case "ctrl+w l":
		m.switchPane(right)
	case "ctrl+w ctrl+w":
		m.switchPane(m.review.cursor.pane.other())
	case "v":
		if m.review.selection == nil {
			selection := m.review.view.beginSelection(m.review.cursor)
			m.review.selection = &selection
		} else {
			m.cancelSelection()
		}
	case "esc":
		m.cancelSelection()
	case "c":
		cmd, err := m.beginComment()
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, cmd
	case "e":
		cmd, err := m.openCurrentLine()
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, cmd
	case "C":
		m.mode = modeComments
		m.comments.row = min(m.comments.row, max(0, len(m.comments.items)-1))
		return m, m.loadComments()
	case "R":
		return m, m.loadRefresh()
	case "tab":
		if m.review.patch.Branch == "" {
			return m, nil
		}
		if m.kind == patch.Unstaged {
			m.kind = patch.Branch
		} else {
			m.kind = patch.Unstaged
		}
		m.cancelSelection()
		return m, m.loadRefresh()
	case "t":
		m.toggleSideBySide()
	}
	return m, nil
}

func (m model) screenBodyHeight() int { return max(1, m.height-3) }
