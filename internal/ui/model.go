package ui

import (
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comments"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

type teaCmd = tea.Cmd
type teaModel = tea.Model
type teaMsg = tea.Msg
type teaKeyPress = tea.KeyPressMsg

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
var formatError = fmt.Errorf
var formatString = fmt.Sprintf

type reviewState struct {
	patch      patch.Patch
	view       View
	cursor     Cursor
	viewport   Viewport
	selection  *Selection
	sideBySide bool
}

type commentState struct {
	items      []reviewComment
	row        int
	body       string
	editIndex  int
	editAnchor commentAnchor
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
	width, height := size.Width, size.Height
	m := Model{
		review:     reviewState{patch: p},
		comments:   commentState{items: comments, editIndex: -1},
		width:      width,
		height:     height,
		save:       save,
		saveLayout: layout.SaveSideBySide,
		dark:       true,
	}
	review := &m.review
	review.sideBySide = layout.SideBySide
	view := m.newReviewView(p)
	review.view = view
	review.viewport = view.NewViewport(m.width, m.screenBodyHeight())
	review.cursor, _ = view.First()
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

func (m Model) Init() tea.Cmd { return func() teaMsg { return tea.RequestBackgroundColor() } }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	review, comment := &m.review, &m.comments
	currentPatch := review.patch
	rebuild, refresh := m.rebuildView, m.loadRefresh
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.dark {
			m.dark = dark
			rebuild(currentPatch)
		}
	case tea.WindowSizeMsg:
		activeBefore := m.sideBySideActive()
		m.width, m.height = msg.Width, msg.Height
		viewport := review.view.Resize(review.viewport, m.width, m.screenBodyHeight())
		review.viewport = viewport
		if activeBefore != m.sideBySideActive() {
			rebuild(currentPatch)
		} else {
			review.viewport = review.view.KeepVisible(viewport, review.cursor)
		}
	case commentEditorFinishedMsg:
		err := msg.err
		if err != nil {
			m.err = err
			m.clearCommentEdit()
		} else {
			comment.body = msg.body
			m.finishCommentEdit()
		}
	case commentsLoadedMsg:
		if msg.revision != comment.revision {
			break
		}
		err := msg.err
		if err != nil {
			m.err = formatError("refresh comments: %w", err)
		} else {
			items := msg.comments
			comment.items = items
			comment.row = min(comment.row, max(0, len(items)-1))
			m.err = nil
		}
	case sourceEditorFinishedMsg:
		err := msg.err
		if err != nil {
			m.err = formatError("editor: %w", err)
			break
		}
		return m, refresh()
	case tea.FocusMsg:
		return m, refresh()
	case refreshDiffMsg:
		if msg.branch != m.currentBranch() {
			return m, nil
		}
		err := msg.err
		if err != nil {
			m.err = formatError("refresh diff: %w", err)
		} else {
			newPatch := msg.patch
			if newPatch.Fingerprint != currentPatch.Fingerprint {
				rebuild(newPatch)
				m.err = nil
			}
		}
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) loadRefresh() teaCmd {
	refresh := m.refresh
	if refresh == nil {
		return nil
	}
	branch := m.currentBranch()
	return func() teaMsg { p, err := refresh(branch); return refreshDiffMsg{patch: p, branch: branch, err: err} }
}

func (m Model) loadComments() teaCmd {
	load := m.load
	if load == nil {
		return nil
	}
	revision := m.comments.revision
	return func() teaMsg {
		comments, err := load()
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
	view := m.newReviewView(p)
	review.view = view
	state := Preserve(oldView, oldState, view)
	review.viewport = state.Viewport
	review.selection = state.Selection
	if state.Cursor != nil {
		review.cursor = *state.Cursor
	}
}

func (m Model) newReviewView(p patch.Patch) View {
	dark := m.dark
	if m.sideBySideActive() {
		return NewSideBySideView(p, dark)
	}
	return NewUnifiedView(p, dark)
}

func (m Model) updateKey(key teaKeyPress) (teaModel, teaCmd) {
	review, comment, search := &m.review, &m.comments, &m.search
	cursor, viewport := review.cursor, review.viewport
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
	nextPending := ""
	view := review.view
	align := view.Align
	scrollHorizontal := view.ScrollHorizontal
	jumpFile, switchPane := m.jumpFile, m.switchPane
	setCursor, move, halfPage := m.setCursor, m.move, m.halfPage
	repeatSearch, cancelSelection := m.repeatSearch, m.cancelSelection
	switch pending {
	case "[", "]":
		sequence := pending + name
		if sequence == "]f" {
			jumpFile(Forward)
		}
		if sequence == "[f" {
			jumpFile(Backward)
		}
		return m, nil
	case "z":
		switch name {
		case "z":
			review.viewport = align(viewport, cursor, Middle)
		case "t":
			review.viewport = align(viewport, cursor, Top)
		case "b":
			review.viewport = align(viewport, cursor, Bottom)
		}
		return m, nil
	case "ctrl+w":
		switch name {
		case "h":
			switchPane(Left)
		case "l":
			switchPane(Right)
		case "ctrl+w":
			switchPane(cursor.Pane.Other())
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
		cancelSelection()
		m.mode = modeSearch
		search.query = nil
		search.from = cursor
		search.miss = false
	case "n":
		repeatSearch(Forward)
	case "N":
		repeatSearch(Backward)
	case "j", "down":
		move(Forward)
	case "k", "up":
		move(Backward)
	case "h", "left":
		review.viewport = scrollHorizontal(viewport, -horizontalScrollStep)
	case "l", "right":
		review.viewport = scrollHorizontal(viewport, horizontalScrollStep)
	case "0":
		review.viewport.LeftColumn = 0
	case "$":
		review.viewport = scrollHorizontal(viewport, int(^uint(0)>>1))
	case "ctrl+d":
		halfPage(Forward)
	case "ctrl+u":
		halfPage(Backward)
	case "ctrl+w":
		nextPending = name
	case "g":
		if pending == "g" {
			if cursor, ok := view.First(); ok {
				setCursor(cursor)
			}
		} else {
			nextPending = "g"
		}
	case "G":
		if cursor, ok := view.Last(); ok {
			setCursor(cursor)
		}
	case "z":
		nextPending = name
	case "]", "[":
		nextPending = name
	case "v":
		if review.selection == nil {
			selection := view.BeginSelection(cursor)
			review.selection = &selection
		} else {
			cancelSelection()
		}
	case "esc":
		cancelSelection()
	case "c", "e":
		open := m.beginComment
		if name == "e" {
			open = m.openCurrentLine
		}
		cmd, err := open()
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, cmd
	case "C":
		m.mode = modeComments
		comment.row = min(comment.row, max(0, len(comment.items)-1))
		return m, m.loadComments()
	case "R":
		return m, m.loadRefresh()
	case "tab":
		if m.defaultBranch == "" {
			return m, nil
		}
		m.showDefault = !m.showDefault
		cancelSelection()
		return m, m.loadRefresh()
	case "t":
		m.toggleSideBySide()
	}
	m.pendingKey = nextPending
	return m, nil
}

func (m Model) currentBranch() string {
	if !m.showDefault {
		return ""
	}
	return m.defaultBranch
}
func (m Model) screenBodyHeight() int { return max(1, m.height-3) }
