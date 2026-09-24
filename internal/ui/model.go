package ui

import (
	"context"
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comments"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Size struct {
	Width  int
	Height int
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

var DefaultSize = Size{80, 30}

type reviewState struct {
	patch      patch.Patch
	view       *diffView
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
	save          func(comments.Comment, patch.Patch) (comments.Comment, error)
	delete        func(comments.Comment, patch.Patch) error
	load          func() ([]comments.Comment, error)
	refresh       func(string) (patch.Patch, error)
	err           error
	quitting      bool
	pendingKey    string
	saveLayout    func(bool) error
	defaultBranch string
	showDefault   bool
	dark          bool
}

func NewWithReview(loader patch.Loader, store comments.Store, ctx context.Context, directory string, p patch.Patch, items []comments.Comment, size Size, defaultBranch string) (Model, error) {
	sideBySide, err := loadLayoutSettings()
	if err != nil {
		return Model{}, err
	}
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	m := Model{
		review:   reviewState{patch: p},
		comments: commentState{items: items, editIndex: -1},
		width:    size.Width,
		height:   size.Height,
		save: func(comment comments.Comment, p patch.Patch) (comments.Comment, error) {
			comment.Repository = p.Repository
			if comment.ID == "" {
				return store.Add(comment)
			}
			if err := store.Update(comment); err != nil {
				return comments.Comment{}, err
			}
			return comment, nil
		},
		saveLayout: saveLayoutSettings,
		dark:       true,
	}
	review := &m.review
	review.sideBySide = sideBySide
	review.view = newDiffView(p, m.dark, m.sideBySideActive())
	review.viewport = review.view.Resize(Viewport{}, m.width, m.screenBodyHeight())
	review.cursor, _ = review.view.First()
	m.delete = func(comment comments.Comment, p patch.Patch) error {
		return store.Delete(p.Repository, comment.ID)
	}
	m.load = func() ([]comments.Comment, error) {
		return store.List(m.review.patch.Repository)
	}
	m.refresh = func(branch string) (patch.Patch, error) {
		if branch == "" {
			return loader.Load(ctx, directory)
		}
		return loader.LoadBranch(ctx, directory, branch)
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
	return func() tea.Msg { p, err := m.refresh(branch); return refreshDiffMsg{p, branch, err} }
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
	review.view = newDiffView(p, m.dark, m.sideBySideActive())
	state := Preserve(oldView, oldState, review.view)
	review.viewport = state.Viewport
	review.selection = state.Selection
	if state.Cursor != nil {
		review.cursor = *state.Cursor
	}
}

func (m Model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	review := &m.review
	comments := &m.comments
	search := &m.search
	cursor := review.cursor
	view := review.view
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
		height := review.viewport.Height
		alignmentOffset := 0
		switch name {
		case "z":
			alignmentOffset = height / 2
		case "t":
		case "b":
			alignmentOffset = height - 1
		default:
			return m, nil
		}
		headerHeight := 0
		if height > 1 {
			headerHeight = 1
		}
		offset := max(0, alignmentOffset-headerHeight)
		review.viewport.Top = cursor.Coordinate - offset
		if !view.hasStickyHeader(review.viewport.Top, height) {
			review.viewport.Top = cursor.Coordinate - alignmentOffset
		}
		review.viewport = view.clampViewport(review.viewport)
		return m, nil
	}
	if pending == "ctrl+w" {
		switch name {
		case "h":
			m.switchPane(Left)
		case "l":
			m.switchPane(Right)
		case "ctrl+w":
			m.switchPane(Right - cursor.Pane)
		}
		return m, nil
	}
	var cmd tea.Cmd
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		cmd = tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		m.cancelSelection()
		m.mode = modeSearch
		search.query = nil
		search.from = cursor
		search.miss = false
	case "n":
		m.repeatSearch(Forward)
	case "N":
		m.repeatSearch(Backward)
	case "j", "down":
		m.move(Forward)
	case "k", "up":
		m.move(Backward)
	case "h", "left", "l", "right":
		delta := horizontalScrollStep
		if name == "h" || name == "left" {
			delta = -delta
		}
		review.viewport.LeftColumn += delta
		review.viewport = view.clampViewport(review.viewport)
	case "0":
		review.viewport.LeftColumn = 0
	case "$":
		review.viewport.LeftColumn += int(^uint(0) >> 1)
		review.viewport = view.clampViewport(review.viewport)
	case "ctrl+d":
		m.halfPage(Forward)
	case "ctrl+u":
		m.halfPage(Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			if cursor, ok := view.First(); ok {
				m.setCursor(cursor)
			}
		} else {
			m.pendingKey = "g"
		}
	case "G":
		cursor, ok := view.scan(len(view.rows), Right, Backward, false)
		if !ok {
			cursor, ok = view.scan(len(view.rows), Left, Backward, false)
		}
		if ok {
			m.setCursor(cursor)
		}
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if review.selection == nil {
			selection := view.BeginSelection(cursor)
			review.selection = &selection
		} else {
			m.cancelSelection()
		}
	case "esc":
		m.cancelSelection()
	case "c":
		var err error
		cmd, err = m.beginComment()
		if err != nil {
			m.err = err
			cmd = nil
		}
	case "e":
		var err error
		cmd, err = m.openCurrentLine()
		if err != nil {
			m.err = err
			cmd = nil
		}
	case "C":
		m.mode = modeComments
		comments.row = min(comments.row, max(0, len(comments.items)-1))
		if m.load != nil {
			revision := comments.revision
			cmd = func() tea.Msg {
				items, err := m.load()
				return commentsLoadedMsg{items, revision, err}
			}
		}
	case "R":
		cmd = m.loadRefresh()
	case "tab":
		if m.defaultBranch == "" {
			break
		}
		m.showDefault = !m.showDefault
		m.cancelSelection()
		cmd = m.loadRefresh()
	case "t":
		m.toggleSideBySide()
	}
	return m, cmd
}

func (m Model) currentBranch() string {
	if !m.showDefault {
		return ""
	}
	return m.defaultBranch
}

func (m Model) screenBodyHeight() int { return max(1, m.height-3) }
