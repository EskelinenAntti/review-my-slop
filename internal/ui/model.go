package ui

import (
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comments"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

type SaveCommentFunc func(comments.Comment, patch.Patch) (comments.Comment, error)
type DeleteCommentFunc func(comments.Comment, patch.Patch) error
type LoadCommentsFunc func() ([]comments.Comment, error)
type RefreshDiffFunc func(parent string) (patch.Patch, error)
type SaveSideBySideFunc func(bool) error

type Size struct {
	Width  int
	Height int
}

type InitialLayout struct {
	SideBySide     bool
	SaveSideBySide SaveSideBySideFunc
	Size           Size
}

type refreshDiffMsg struct {
	patch  patch.Patch
	branch string
	err    error
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

var DefaultSize = Size{Width: 80, Height: 30}

type reviewState struct {
	patch      patch.Patch
	view       View
	cursor     Cursor
	viewport   Viewport
	selection  *Selection
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
	from  Cursor
	miss  bool
}

type Model struct {
	review        reviewState
	comments      commentState
	search        searchState
	width         int
	height        int
	mode          mode
	save          SaveCommentFunc
	delete        DeleteCommentFunc
	load          LoadCommentsFunc
	refresh       RefreshDiffFunc
	err           error
	quitting      bool
	pendingKey    string
	saveLayout    SaveSideBySideFunc
	defaultBranch string
	showDefault   bool
	dark          bool
}

func New(p patch.Patch, comments []comments.Comment, save SaveCommentFunc, layout InitialLayout) Model {
	size := layout.Size
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	m := Model{
		review:     reviewState{patch: p},
		comments:   commentState{items: comments, editIndex: -1},
		width:      size.Width,
		height:     size.Height,
		save:       save,
		saveLayout: layout.SaveSideBySide,
		dark:       true,
	}
	m.review.sideBySide = layout.SideBySide
	m.review.view = m.newReviewView(p)
	m.review.viewport = m.review.view.NewViewport(m.width, m.screenBodyHeight())
	m.review.cursor, _ = m.review.view.First()
	return m
}

func NewWithReview(actions review.Actions, p patch.Patch, items []comments.Comment, size Size) (Model, error) {
	sideBySide, err := loadLayoutSettings()
	if err != nil {
		return Model{}, err
	}
	m := New(p, items, actions.SaveComment, InitialLayout{
		SideBySide:     sideBySide,
		SaveSideBySide: saveLayoutSettings,
		Size:           size,
	})
	m.SetDelete(actions.DeleteComment)
	m.SetLoadComments(func() ([]comments.Comment, error) {
		return actions.Comments(m.review.patch)
	})
	m.SetRefresh(func(branch string) (patch.Patch, error) {
		return actions.Load(branch)
	})
	return m, nil
}

func (m *Model) SetRefresh(refresh RefreshDiffFunc)    { m.refresh = refresh }
func (m *Model) SetDelete(delete DeleteCommentFunc)    { m.delete = delete }
func (m *Model) SetLoadComments(load LoadCommentsFunc) { m.load = load }
func (m *Model) SetDefaultBranch(branch string) {
	m.defaultBranch = branch
	if branch == "" {
		m.showDefault = false
	}
}
func (m *Model) SetSideBySide(enabled bool, save SaveSideBySideFunc) {
	m.saveLayout = save
	m.setSideBySide(enabled)
}

func (m Model) Init() tea.Cmd { return func() tea.Msg { return tea.RequestBackgroundColor() } }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.dark {
			m.dark = dark
			m.rebuildView(m.review.patch)
		}
	case tea.WindowSizeMsg:
		activeBefore := m.sideBySideActive()
		m.width, m.height = msg.Width, msg.Height
		m.review.viewport = m.review.view.Resize(m.review.viewport, m.width, m.screenBodyHeight())
		if activeBefore != m.sideBySideActive() {
			m.rebuildView(m.review.patch)
		} else {
			m.review.viewport = m.review.view.KeepVisible(m.review.viewport, m.review.cursor)
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
		if msg.branch != m.currentBranch() {
			return m, nil
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", msg.err)
		} else if msg.patch.Fingerprint != m.review.patch.Fingerprint {
			m.rebuildView(msg.patch)
			m.err = nil
		}
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) loadRefresh() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	branch := m.currentBranch()
	return func() tea.Msg { p, err := m.refresh(branch); return refreshDiffMsg{patch: p, branch: branch, err: err} }
}

func (m Model) loadComments() tea.Cmd {
	if m.load == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		comments, err := m.load()
		return commentsLoadedMsg{comments: comments, revision: revision, err: err}
	}
}

func (m *Model) rebuildView(p patch.Patch) {
	oldView := m.review.view
	oldState := State{
		Cursor:    &m.review.cursor,
		Selection: m.review.selection,
		Viewport:  m.review.viewport,
	}
	m.review.patch = p
	m.review.view = m.newReviewView(p)
	state := Preserve(oldView, oldState, m.review.view)
	m.review.viewport = state.Viewport
	m.review.selection = state.Selection
	if state.Cursor != nil {
		m.review.cursor = *state.Cursor
	}
}

func (m Model) newReviewView(p patch.Patch) View {
	if m.sideBySideActive() {
		return NewSideBySideView(p, m.dark)
	}
	return NewUnifiedView(p, m.dark)
}

func (m Model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	pending := m.pendingKey
	m.pendingKey = ""
	if pending == "[" || pending == "]" {
		if pending+name == "]f" {
			m.jumpFile(Forward)
		}
		if pending+name == "[f" {
			m.jumpFile(Backward)
		}
		return m, nil
	}
	if pending == "z" {
		switch name {
		case "z":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, Middle)
		case "t":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, Top)
		case "b":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, Bottom)
		}
		return m, nil
	}
	if pending == "ctrl+w" {
		switch name {
		case "h":
			m.switchPane(Left)
		case "l":
			m.switchPane(Right)
		case "ctrl+w":
			m.switchPane(m.review.cursor.Pane.Other())
		}
		return m, nil
	}
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
		m.repeatSearch(Forward)
	case "N":
		m.repeatSearch(Backward)
	case "j", "down":
		m.move(Forward)
	case "k", "up":
		m.move(Backward)
	case "h", "left":
		m.review.viewport = m.review.view.ScrollHorizontal(m.review.viewport, -horizontalScrollStep)
	case "l", "right":
		m.review.viewport = m.review.view.ScrollHorizontal(m.review.viewport, horizontalScrollStep)
	case "0":
		m.review.viewport.LeftColumn = 0
	case "$":
		m.review.viewport = m.review.view.ScrollHorizontal(m.review.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(Forward)
	case "ctrl+u":
		m.halfPage(Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			if cursor, ok := m.review.view.First(); ok {
				m.setCursor(cursor)
			}
		} else {
			m.pendingKey = "g"
		}
	case "G":
		if cursor, ok := m.review.view.Last(); ok {
			m.setCursor(cursor)
		}
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if m.review.selection == nil {
			selection := m.review.view.BeginSelection(m.review.cursor)
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
		if m.defaultBranch == "" {
			return m, nil
		}
		m.showDefault = !m.showDefault
		m.cancelSelection()
		return m, m.loadRefresh()
	case "t":
		m.toggleSideBySide()
	}
	return m, nil
}

func (m Model) currentBranch() string {
	if !m.showDefault {
		return ""
	}
	return m.defaultBranch
}
func (m Model) screenBodyHeight() int { return max(1, m.height-3) }
