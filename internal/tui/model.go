package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/view"
)

type SaveCommentFunc func(review.Comment, patch.Patch) (review.Comment, error)
type DeleteCommentFunc func(review.Comment, patch.Patch) error
type LoadCommentsFunc func() ([]review.Comment, error)
type RefreshDiffFunc func(parent string) (patch.Patch, error)
type SaveSideBySideFunc func(bool) error

type Size struct {
	Width  int
	Height int
}

type Config struct {
	Patch          patch.Patch
	Comments       []review.Comment
	Size           Size
	SideBySide     bool
	DefaultBranch  string
	SaveComment    SaveCommentFunc
	DeleteComment  DeleteCommentFunc
	LoadComments   LoadCommentsFunc
	RefreshDiff    RefreshDiffFunc
	SaveSideBySide SaveSideBySideFunc
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
	comments []review.Comment
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
	screenChromeHeight     = 3
)

var DefaultSize = Size{Width: 80, Height: 30}

type Model struct {
	review   reviewState
	comments commentState
	search   searchState
	layoutState
	callbackState
	interactionState
}

type layoutState struct {
	width          int
	height         int
	sideBySide     bool
	defaultBranch  string
	showDefault    bool
	darkBackground bool
}

type callbackState struct {
	saveComment    SaveCommentFunc
	deleteComment  DeleteCommentFunc
	loadComments   LoadCommentsFunc
	refreshDiff    RefreshDiffFunc
	saveSideBySide SaveSideBySideFunc
}

type interactionState struct {
	mode       mode
	err        error
	quitting   bool
	pendingKey string
}

func New(config Config) Model {
	size := config.Size
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	m := Model{
		review:   reviewState{patch: config.Patch},
		comments: commentState{items: config.Comments, editIndex: -1},
		layoutState: layoutState{
			width:          size.Width,
			height:         size.Height,
			sideBySide:     config.SideBySide,
			defaultBranch:  config.DefaultBranch,
			darkBackground: true,
		},
		callbackState: callbackState{
			saveComment:    config.SaveComment,
			deleteComment:  config.DeleteComment,
			loadComments:   config.LoadComments,
			refreshDiff:    config.RefreshDiff,
			saveSideBySide: config.SaveSideBySide,
		},
	}
	m.review.view = m.newReviewView(config.Patch)
	m.review.viewport = m.review.view.NewViewport(m.width, m.bodyHeight())
	if cursor, ok := m.review.view.First(); ok {
		m.review.cursor = cursor
	}
	return m
}

func (m Model) Init() tea.Cmd { return func() tea.Msg { return tea.RequestBackgroundColor() } }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.updateBackground(msg)
	case tea.WindowSizeMsg:
		m.updateWindowSize(msg)
	case commentEditorFinishedMsg:
		m.finishEditorComment(msg)
	case commentsLoadedMsg:
		m.applyLoadedComments(msg)
	case sourceEditorFinishedMsg:
		return m, m.finishSourceEditor(msg)
	case tea.FocusMsg:
		return m, m.refreshCommand()
	case refreshDiffMsg:
		m.applyRefreshedPatch(msg)
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m *Model) updateBackground(msg tea.BackgroundColorMsg) {
	if dark := msg.IsDark(); dark != m.darkBackground {
		m.darkBackground = dark
		m.rebuildView(m.review.patch)
	}
}

func (m *Model) updateWindowSize(msg tea.WindowSizeMsg) {
	activeBefore := m.sideBySideActive()
	m.width, m.height = msg.Width, msg.Height
	m.review.viewport = m.review.view.Resize(m.review.viewport, m.width, m.bodyHeight())
	if activeBefore != m.sideBySideActive() {
		m.rebuildView(m.review.patch)
		return
	}
	m.review.viewport = m.review.view.KeepVisible(m.review.viewport, m.review.cursor)
}

func (m *Model) finishEditorComment(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.comments.clearEdit()
		return
	}
	m.comments.body = msg.body
	m.finishCommentEdit()
}

func (m *Model) applyLoadedComments(msg commentsLoadedMsg) {
	if msg.revision != m.comments.revision {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh comments: %w", msg.err)
		return
	}
	m.comments.setItems(msg.comments)
	m.err = nil
}

func (m *Model) finishSourceEditor(msg sourceEditorFinishedMsg) tea.Cmd {
	if msg.err != nil {
		m.err = fmt.Errorf("editor: %w", msg.err)
		return nil
	}
	return m.refreshCommand()
}

func (m *Model) applyRefreshedPatch(msg refreshDiffMsg) {
	if msg.branch != m.comparisonBranch() {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh diff: %w", msg.err)
		return
	}
	if msg.patch.Fingerprint == m.review.patch.Fingerprint {
		return
	}
	m.rebuildView(msg.patch)
	m.err = nil
}

func (m Model) refreshCommand() tea.Cmd {
	if m.refreshDiff == nil {
		return nil
	}
	branch := m.comparisonBranch()
	return func() tea.Msg {
		loaded, err := m.refreshDiff(branch)
		return refreshDiffMsg{patch: loaded, branch: branch, err: err}
	}
}

func (m Model) loadCommentsCommand() tea.Cmd {
	if m.loadComments == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		comments, err := m.loadComments()
		return commentsLoadedMsg{comments: comments, revision: revision, err: err}
	}
}

func (m Model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
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
	return m.updateBrowseKey(name)
}

func (m Model) updateBrowseKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	pending := m.pendingKey
	m.pendingKey = ""
	if m.handlePendingKey(pending, name) {
		return m, nil
	}
	if m.handleBrowseNavigation(name, pending) {
		return m, nil
	}
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		m.startSearch()
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
		return m, m.loadCommentsCommand()
	case "R":
		return m, m.refreshCommand()
	case "tab":
		cmd := m.toggleComparisonBranch()
		return m, cmd
	case "t":
		m.toggleSideBySide()
	}
	return m, nil
}

func (m *Model) handleBrowseNavigation(name, pending string) bool {
	switch name {
	case "n":
		m.repeatSearch(view.Forward)
	case "N":
		m.repeatSearch(view.Backward)
	case "j", "down":
		m.move(view.Forward)
	case "k", "up":
		m.move(view.Backward)
	case "h", "left":
		m.review.viewport = m.review.view.ScrollHorizontal(m.review.viewport, -horizontalScrollStep)
	case "l", "right":
		m.review.viewport = m.review.view.ScrollHorizontal(m.review.viewport, horizontalScrollStep)
	case "0":
		m.review.viewport.LeftColumn = 0
	case "$":
		m.review.viewport = m.review.view.ScrollHorizontal(m.review.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(view.Forward)
	case "ctrl+u":
		m.halfPage(view.Backward)
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
	default:
		return false
	}
	return true
}

func (m *Model) startSearch() {
	m.cancelSelection()
	m.mode = modeSearch
	m.search.query = nil
	m.search.from = m.review.cursor
	m.search.miss = false
}

func (m *Model) toggleComparisonBranch() tea.Cmd {
	if m.defaultBranch == "" {
		return nil
	}
	m.showDefault = !m.showDefault
	m.cancelSelection()
	return m.refreshCommand()
}

func (m *Model) handlePendingKey(pending, name string) bool {
	switch pending {
	case "[", "]":
		switch pending + name {
		case "]f":
			m.jumpFile(view.Forward)
		case "[f":
			m.jumpFile(view.Backward)
		}
		return true
	case "z":
		switch name {
		case "z":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, view.Middle)
		case "t":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, view.Top)
		case "b":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, view.Bottom)
		}
		return true
	case "ctrl+w":
		switch name {
		case "h":
			m.switchPane(view.Left)
		case "l":
			m.switchPane(view.Right)
		case "ctrl+w":
			m.switchPane(m.review.cursor.Pane.Other())
		}
		return true
	default:
		return false
	}
}

func (m Model) comparisonBranch() string {
	if !m.showDefault {
		return ""
	}
	return m.defaultBranch
}
func (m Model) bodyHeight() int { return max(1, m.height-screenChromeHeight) }
