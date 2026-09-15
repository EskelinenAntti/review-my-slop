package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

type SaveCommentFunc func(comment.Comment, diff.ChangeSet) (comment.Comment, error)
type DeleteCommentFunc func(comment.Comment, diff.ChangeSet) error
type LoadCommentsFunc func() ([]comment.Comment, error)
type RefreshDiffFunc func(branch string) (diff.ChangeSet, error)
type SaveSideBySideFunc func(bool) error

type Dependencies struct {
	SaveComment    SaveCommentFunc
	DeleteComment  DeleteCommentFunc
	LoadComments   LoadCommentsFunc
	RefreshDiff    RefreshDiffFunc
	SaveSideBySide SaveSideBySideFunc
	Editor         Editor
}

type Size struct {
	Width  int
	Height int
}

type Layout struct {
	SideBySide bool
	Size       Size
}

type refreshDiffMsg struct {
	changes diff.ChangeSet
	branch  string
	err     error
}

// CommentEditedMsg is returned by an Editor after a comment draft closes.
type CommentEditedMsg struct {
	Body string
	Err  error
}

type commentsLoadedMsg struct {
	comments []comment.Comment
	revision uint64
	err      error
}

// SourceEditedMsg is returned by an Editor after a source file closes.
type SourceEditedMsg struct{ Err error }

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

type changeState struct {
	changes    diff.ChangeSet
	view       View
	cursor     Cursor
	viewport   Viewport
	selection  *Selection
	sideBySide bool
}

type commentState struct {
	items      []comment.Comment
	row        int
	body       string
	editIndex  int
	editAnchor comment.Anchor
	revision   uint64
}

type searchState struct {
	query []rune
	term  string
	from  Cursor
	miss  bool
}

type Model struct {
	changes       changeState
	comments      commentState
	search        searchState
	width         int
	height        int
	mode          mode
	dependencies  Dependencies
	err           error
	quitting      bool
	pendingKey    string
	defaultBranch string
	showDefault   bool
	dark          bool
}

func New(changes diff.ChangeSet, comments []comment.Comment, dependencies Dependencies, layout Layout) Model {
	size := layout.Size
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	model := Model{
		changes:      changeState{changes: changes, sideBySide: layout.SideBySide},
		comments:     commentState{items: comments, editIndex: -1},
		width:        size.Width,
		height:       size.Height,
		dependencies: dependencies,
		dark:         true,
	}
	model.changes.view = model.newView(changes)
	model.changes.viewport = model.changes.view.NewViewport(model.width, model.bodyHeight())
	model.changes.cursor, _ = model.changes.view.First()
	return model
}

func (m *Model) SetDefaultBranch(branch string) {
	m.defaultBranch = branch
	if branch == "" {
		m.showDefault = false
	}
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.BackgroundColorMsg:
		m.updateBackground(message)
	case tea.WindowSizeMsg:
		m.resize(message.Width, message.Height)
	case CommentEditedMsg:
		m.finishEditor(message)
	case commentsLoadedMsg:
		m.applyLoadedComments(message)
	case SourceEditedMsg:
		if message.Err != nil {
			m.err = fmt.Errorf("editor: %w", message.Err)
			break
		}
		return m, m.refresh()
	case tea.FocusMsg:
		return m, m.refresh()
	case refreshDiffMsg:
		m.applyRefresh(message)
	case tea.KeyPressMsg:
		return m.handleKey(message)
	}
	return m, nil
}

func (m *Model) updateBackground(message tea.BackgroundColorMsg) {
	if dark := message.IsDark(); dark != m.dark {
		m.dark = dark
		m.rebuildView(m.changes.changes)
	}
}

func (m *Model) resize(width, height int) {
	wasActive := m.sideBySideActive()
	m.width, m.height = width, height
	m.changes.viewport = m.changes.view.Resize(m.changes.viewport, m.width, m.bodyHeight())
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.changes.changes)
		return
	}
	m.changes.viewport = m.changes.view.KeepVisible(m.changes.viewport, m.changes.cursor)
}

func (m *Model) finishEditor(message CommentEditedMsg) {
	if message.Err != nil {
		m.err = message.Err
		m.clearCommentEdit()
		return
	}
	m.comments.body = message.Body
	m.finishCommentEdit()
}

func (m *Model) applyLoadedComments(message commentsLoadedMsg) {
	if message.revision != m.comments.revision {
		return
	}
	if message.err != nil {
		m.err = fmt.Errorf("refresh comments: %w", message.err)
		return
	}
	m.comments.items = message.comments
	m.comments.row = min(m.comments.row, max(0, len(m.comments.items)-1))
	m.err = nil
}

func (m *Model) applyRefresh(message refreshDiffMsg) {
	if message.branch != m.currentBranch() {
		return
	}
	if message.err != nil {
		m.err = fmt.Errorf("refresh diff: %w", message.err)
		return
	}
	if message.changes.Fingerprint != m.changes.changes.Fingerprint {
		m.rebuildView(message.changes)
		m.err = nil
	}
}

func (m Model) refresh() tea.Cmd {
	if m.dependencies.RefreshDiff == nil {
		return nil
	}
	branch := m.currentBranch()
	return func() tea.Msg {
		changes, err := m.dependencies.RefreshDiff(branch)
		return refreshDiffMsg{changes: changes, branch: branch, err: err}
	}
}

func (m Model) loadComments() tea.Cmd {
	if m.dependencies.LoadComments == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		items, err := m.dependencies.LoadComments()
		return commentsLoadedMsg{comments: items, revision: revision, err: err}
	}
}

type cursorIdentity struct {
	file   diff.File
	hunk   diff.Hunk
	line   diff.Line
	cursor Cursor
	valid  bool
}

func (m Model) identify(cursor Cursor) cursorIdentity {
	file, fileOK := m.changes.view.File(cursor)
	hunk, hunkOK := m.changes.view.Hunk(cursor)
	line, lineOK := m.changes.view.Line(cursor)
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: fileOK && hunkOK && lineOK}
}

func (m *Model) rebuildView(changes diff.ChangeSet) {
	cursor := m.identify(m.changes.cursor)
	var first, last cursorIdentity
	if m.changes.selection != nil {
		first, last = m.identify(m.changes.selection.First), m.identify(m.changes.selection.Last)
	}
	rowsAbove := m.changes.cursor.Coordinate.Y - m.changes.viewport.Top.Y
	m.changes.changes = changes
	m.changes.view = m.newView(changes)
	m.changes.viewport = m.changes.view.NewViewport(m.width, m.bodyHeight())
	if cursor.valid {
		m.changes.cursor, cursor.valid = m.changes.view.FindCursor(cursor.file, cursor.hunk, cursor.line, cursor.cursor.Coordinate, cursor.cursor.Pane)
	}
	if !cursor.valid {
		m.changes.cursor, _ = m.changes.view.First()
	}
	m.changes.selection = nil
	if first.valid && last.valid {
		translatedFirst, firstOK := m.changes.view.FindCursor(first.file, first.hunk, first.line, first.cursor.Coordinate, first.cursor.Pane)
		translatedLast, lastOK := m.changes.view.FindCursor(last.file, last.hunk, last.line, last.cursor.Coordinate, last.cursor.Pane)
		if firstOK && lastOK {
			selection := Selection{First: translatedFirst, Last: translatedLast}
			if firstHunk, ok := m.changes.view.Hunk(translatedFirst); ok {
				if firstFile, fileOK := m.changes.view.File(translatedFirst); fileOK {
					if lastFile, lastFileOK := m.changes.view.File(translatedLast); lastFileOK && sameChangeFile(firstFile, lastFile) {
						if lastHunk, hunkOK := m.changes.view.Hunk(translatedLast); hunkOK && firstHunk.Header == lastHunk.Header {
							m.changes.selection = &selection
						}
					}
				}
			}
		}
	}
	m.changes.viewport.Top.Y = max(0, m.changes.cursor.Coordinate.Y-rowsAbove)
	m.changes.viewport = m.changes.view.KeepVisible(m.changes.viewport, m.changes.cursor)
}

func (m Model) newView(changes diff.ChangeSet) View {
	if m.sideBySideActive() {
		return NewSideBySideView(changes, m.dark)
	}
	return NewUnifiedView(changes, m.dark)
}

func sameChangeFile(first, last diff.File) bool {
	return first.OldPath == last.OldPath && first.NewPath == last.NewPath
}

func (m Model) handleKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	if m.handlePendingKey(pending, name) {
		return m, nil
	}
	return m.handleBrowseKey(name, key, pending)
}

func (m *Model) handlePendingKey(pending, name string) bool {
	if pending == "[" || pending == "]" {
		if pending+name == "]f" {
			m.jumpFile(Forward)
		}
		if pending+name == "[f" {
			m.jumpFile(Backward)
		}
		return true
	}
	if pending == "z" {
		switch name {
		case "z":
			m.changes.viewport = m.changes.view.Align(m.changes.viewport, m.changes.cursor, Middle)
		case "t":
			m.changes.viewport = m.changes.view.Align(m.changes.viewport, m.changes.cursor, Top)
		case "b":
			m.changes.viewport = m.changes.view.Align(m.changes.viewport, m.changes.cursor, Bottom)
		}
		return true
	}
	if pending == "ctrl+w" {
		switch name {
		case "h":
			m.switchPane(Left)
		case "l":
			m.switchPane(Right)
		case "ctrl+w":
			m.switchPane(m.changes.cursor.Pane.Other())
		}
		return true
	}
	return false
}

func (m Model) handleBrowseKey(name string, key tea.KeyPressMsg, pending string) (tea.Model, tea.Cmd) {
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
		m.search.from = m.changes.cursor
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
		m.changes.viewport = m.changes.view.ScrollHorizontal(m.changes.viewport, -horizontalScrollStep)
	case "l", "right":
		m.changes.viewport = m.changes.view.ScrollHorizontal(m.changes.viewport, horizontalScrollStep)
	case "0":
		m.changes.viewport.LeftColumn = 0
	case "$":
		m.changes.viewport = m.changes.view.ScrollHorizontal(m.changes.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(Forward)
	case "ctrl+u":
		m.halfPage(Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			if cursor, ok := m.changes.view.First(); ok {
				m.setCursor(cursor)
			}
		} else {
			m.pendingKey = "g"
		}
	case "G":
		if cursor, ok := m.changes.view.Last(); ok {
			m.setCursor(cursor)
		}
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if m.changes.selection == nil {
			selection := m.changes.view.BeginSelection(m.changes.cursor)
			m.changes.selection = &selection
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
		return m, m.refresh()
	case "tab":
		if m.defaultBranch == "" {
			return m, nil
		}
		m.showDefault = !m.showDefault
		m.cancelSelection()
		return m, m.refresh()
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

func (m Model) bodyHeight() int { return max(1, m.height-3) }
