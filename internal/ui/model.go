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
	review := &m.review
	review.sideBySide = layout.SideBySide
	review.view = m.newReviewView(p)
	review.viewport = review.view.NewViewport(m.width, m.screenBodyHeight())
	review.cursor, _ = review.view.First()
	return m
}

func NewWithReview(actions review.Review, p patch.Patch, items []comments.Comment, size Size, defaultBranch string) (Model, error) {
	sideBySide, err := loadLayoutSettings()
	if err != nil {
		return Model{}, err
	}
	m := New(p, items, actions.SaveComment, InitialLayout{
		SideBySide:     sideBySide,
		SaveSideBySide: saveLayoutSettings,
		Size:           size,
	})
	m.delete = actions.DeleteComment
	m.load = func() ([]comments.Comment, error) {
		return actions.Comments(m.review.patch)
	}
	m.refresh = func(branch string) (patch.Patch, error) {
		return actions.Load(branch)
	}
	m.defaultBranch = defaultBranch
	return m, nil
}

func (m Model) Init() tea.Cmd { return func() tea.Msg { return tea.RequestBackgroundColor() } }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	review := &m.review
	comments := &m.comments
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.dark {
			m.dark = dark
			m.rebuildView(review.patch)
		}
	case tea.WindowSizeMsg:
		activeBefore := m.sideBySideActive()
		m.width, m.height = msg.Width, msg.Height
		review.viewport = review.view.Resize(review.viewport, m.width, m.screenBodyHeight())
		if activeBefore != m.sideBySideActive() {
			m.rebuildView(review.patch)
		} else {
			review.viewport = review.view.KeepVisible(review.viewport, review.cursor)
		}
	case commentEditorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.clearCommentEdit()
		} else {
			comments.body = msg.body
			m.finishCommentEdit()
		}
	case commentsLoadedMsg:
		if msg.revision != comments.revision {
			break
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh comments: %w", msg.err)
		} else {
			comments.items = msg.comments
			comments.row = min(comments.row, max(0, len(comments.items)-1))
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
		} else if msg.patch.Fingerprint != review.patch.Fingerprint {
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
	review := &m.review
	oldView := review.view
	oldState := State{
		Cursor:    &review.cursor,
		Selection: review.selection,
		Viewport:  review.viewport,
	}
	review.patch = p
	review.view = m.newReviewView(p)
	state := Preserve(oldView, oldState, review.view)
	review.viewport = state.Viewport
	review.selection = state.Selection
	if state.Cursor != nil {
		review.cursor = *state.Cursor
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
	review := &m.review
	comments := &m.comments
	search := &m.search
	switch m.mode {
	case modeComments:
		return m.updateComments(name)
	case modeHelp:
		if name == "esc" || name == "?" || name == "q" {
			m.mode = modeBrowse
		}
		return m, nil
	case modeSearch:
		return m.updateSearch(name, key)
	}
	m.err = nil
	pending := m.pendingKey
	m.pendingKey = ""
	if pending == "[" || pending == "]" {
		if name == "f" {
			direction := Backward
			if pending == "]" {
				direction = Forward
			}
			m.jumpFile(direction)
		}
		return m, nil
	}
	if pending == "z" {
		switch name {
		case "z":
			review.viewport = review.view.Align(review.viewport, review.cursor, Middle)
		case "t":
			review.viewport = review.view.Align(review.viewport, review.cursor, Top)
		case "b":
			review.viewport = review.view.Align(review.viewport, review.cursor, Bottom)
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
			m.switchPane(review.cursor.Pane.Other())
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
		search.query = nil
		search.from = review.cursor
		search.miss = false
	case "n":
		m.repeatSearch(Forward)
	case "N":
		m.repeatSearch(Backward)
	case "j", "down":
		m.move(Forward)
	case "k", "up":
		m.move(Backward)
	case "h", "left":
		review.viewport = review.view.ScrollHorizontal(review.viewport, -horizontalScrollStep)
	case "l", "right":
		review.viewport = review.view.ScrollHorizontal(review.viewport, horizontalScrollStep)
	case "0":
		review.viewport.LeftColumn = 0
	case "$":
		review.viewport = review.view.ScrollHorizontal(review.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(Forward)
	case "ctrl+u":
		m.halfPage(Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			if cursor, ok := review.view.First(); ok {
				m.setCursor(cursor)
			}
		} else {
			m.pendingKey = "g"
		}
	case "G":
		if cursor, ok := review.view.Last(); ok {
			m.setCursor(cursor)
		}
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if review.selection == nil {
			selection := review.view.BeginSelection(review.cursor)
			review.selection = &selection
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
		comments.row = min(comments.row, max(0, len(comments.items)-1))
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
