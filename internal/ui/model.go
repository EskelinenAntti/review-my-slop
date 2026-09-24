package ui

import (
	"context"
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comments"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type (
	Size struct {
		Width, Height int
	}
	refreshDiffMsg struct {
		patch  patch.Patch
		branch string
		err    error
	}
	commentEditorFinishedMsg struct {
		body string
		err  error
	}
	commentsLoadedMsg struct {
		comments []comments.Comment
		revision uint64
		err      error
	}
	sourceEditorFinishedMsg struct{ err error }
	mode                    uint8
)

const (
	modeBrowse mode = iota
	modeComments
	modeHelp
	modeSearch

	// Layout constants are explicit so the mode iota remains independent.
	horizontalScrollStep   = 4
	minimumSideBySideWidth = 100
)

var DefaultSize = Size{80, 30}

type (
	reviewState struct {
		patch      patch.Patch
		view       *diffView
		cursor     Cursor
		viewport   Viewport
		selection  *Selection
		sideBySide bool
	}
	commentState struct {
		items          []comments.Comment
		row, editIndex int
		body           string
		editAnchor     comments.Anchor
		revision       uint64
	}
	searchState struct {
		query []rune
		term  string
		from  Cursor
		miss  bool
	}
	Model struct {
		review                      reviewState
		comments                    commentState
		search                      searchState
		width, height               int
		mode                        mode
		save                        func(comments.Comment, patch.Patch) (comments.Comment, error)
		delete                      func(comments.Comment, patch.Patch) error
		load                        func() ([]comments.Comment, error)
		refresh                     func(string) (patch.Patch, error)
		err                         error
		pendingKey                  string
		saveLayout                  func(bool) error
		defaultBranch               string
		quitting, showDefault, dark bool
	}
)

func NewWithReview(loader patch.Loader, store comments.Store, ctx context.Context, directory string, p patch.Patch, items []comments.Comment, size Size, defaultBranch string) (Model, error) {
	sideBySide, err := loadLayoutSettings()
	if err != nil {
		return Model{}, err
	}
	if min(size.Width, size.Height) <= 0 {
		size = DefaultSize
	}
	m := Model{
		review:   reviewState{patch: p, sideBySide: sideBySide},
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
		saveLayout:    saveLayoutSettings,
		defaultBranch: defaultBranch,
		dark:          true,
	}
	review := &m.review
	review.view = newDiffView(p, m.dark, m.sideBySideActive())
	review.viewport = review.view.Resize(Viewport{}, m.width, m.screenBodyHeight())
	review.cursor, _ = review.view.First()
	m.delete = func(comment comments.Comment, p patch.Patch) error {
		return store.Delete(p.Repository, comment.ID)
	}
	m.load = func() ([]comments.Comment, error) {
		return store.List(review.patch.Repository)
	}
	m.refresh = func(branch string) (patch.Patch, error) {
		if branch == "" {
			return loader.Load(ctx, directory)
		}
		return loader.LoadBranch(ctx, directory, branch)
	}
	return m, nil
}

func (Model) Init() tea.Cmd { return func() tea.Msg { return tea.RequestBackgroundColor() } }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	review, comments := &m.review, &m.comments
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
		if msg.revision == comments.revision {
			if msg.err != nil {
				m.err = fmt.Errorf("refresh comments: %w", msg.err)
			} else {
				comments.items = msg.comments
				comments.row = min(comments.row, max(0, len(comments.items)-1))
				m.err = nil
			}
		}
	case sourceEditorFinishedMsg:
		if msg.err == nil {
			return m, m.loadRefresh()
		}
		m.err = fmt.Errorf("editor: %w", msg.err)
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
	review.patch = p
	review.view = newDiffView(p, m.dark, m.sideBySideActive())
	state := Preserve(oldView, State{&review.cursor, review.selection, review.viewport}, review.view)
	review.viewport = state.Viewport
	review.selection = state.Selection
	if state.Cursor != nil {
		review.cursor = *state.Cursor
	}
}

func (m Model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	review, comments, search := &m.review, &m.comments, &m.search
	cursor, view, viewport := review.cursor, review.view, &review.viewport
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
		height, alignmentOffset := viewport.Height, 0
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
		viewport.Top = cursor.Coordinate - offset
		if !view.hasStickyHeader(viewport.Top, height) {
			viewport.Top = cursor.Coordinate - alignmentOffset
		}
		*viewport = view.clampViewport(*viewport)
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
		m.quitting, cmd = true, tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		review.selection = nil
		m.mode = modeSearch
		search.query, search.from, search.miss = nil, cursor, false
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
		viewport.LeftColumn += delta
		*viewport = view.clampViewport(*viewport)
	case "0":
		viewport.LeftColumn = 0
	case "$":
		viewport.LeftColumn += int(^uint(0) >> 1)
		*viewport = view.clampViewport(*viewport)
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
		last := len(view.rows)
		cursor, ok := view.scan(last, Right, Backward)
		if !ok {
			cursor, ok = view.scan(last, Left, Backward)
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
			review.selection = &Selection{cursor, cursor}
		} else {
			review.selection = nil
		}
	case "esc":
		review.selection = nil
	case "c":
		cmd, m.err = m.beginComment()
	case "e":
		cmd, m.err = m.openCurrentLine()
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
		if m.defaultBranch != "" {
			m.showDefault = !m.showDefault
			review.selection = nil
			cmd = m.loadRefresh()
		}
	case "t":
		m.toggleSideBySide()
	}
	return m, cmd
}

func (m Model) currentBranch() string {
	if m.showDefault {
		return m.defaultBranch
	}
	return ""
}

func (m Model) screenBodyHeight() int { return max(1, m.height-3) }
