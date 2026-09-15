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

type reviewState struct {
	changes   diff.ChangeSet
	cursor    Cursor
	selection *Selection
}

type layoutState struct {
	size       Size
	sideBySide bool
	dark       bool
	view       View
	viewport   Viewport
}

func (layout layoutState) bodyHeight() int {
	return max(1, layout.size.Height-3)
}

func (layout layoutState) sideBySideActive() bool {
	return layout.sideBySide && layout.size.Width >= minimumSideBySideWidth
}

func (layout *layoutState) resize(width, height int) bool {
	wasActive := layout.sideBySideActive()
	layout.size = Size{Width: width, Height: height}
	layout.viewport = layout.view.Resize(layout.viewport, width, layout.bodyHeight())
	return wasActive != layout.sideBySideActive()
}

func (layout *layoutState) keepCursorVisible(cursor Cursor) {
	layout.viewport = layout.view.KeepVisible(layout.viewport, cursor)
}

func (layout *layoutState) setSideBySide(enabled bool) bool {
	wasActive := layout.sideBySideActive()
	layout.sideBySide = enabled
	return wasActive != layout.sideBySideActive()
}

func (layout layoutState) viewFor(changes diff.ChangeSet) View {
	if layout.sideBySideActive() {
		return NewSideBySideView(changes, layout.dark)
	}
	return NewUnifiedView(changes, layout.dark)
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
	review        reviewState
	comments      commentState
	search        searchState
	layout        layoutState
	mode          mode
	dependencies  Dependencies
	err           error
	quitting      bool
	pendingKey    string
	defaultBranch string
	showDefault   bool
}

func New(changes diff.ChangeSet, comments []comment.Comment, dependencies Dependencies, layout Layout) Model {
	size := layout.Size
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	model := Model{
		review:       reviewState{changes: changes},
		comments:     commentState{items: comments, editIndex: -1},
		layout:       layoutState{size: size, sideBySide: layout.SideBySide, dark: true},
		dependencies: dependencies,
	}
	model.layout.view = model.layout.viewFor(changes)
	model.layout.viewport = model.layout.view.NewViewport(size.Width, model.layout.bodyHeight())
	model.review.cursor, _ = model.layout.view.First()
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
	if dark := message.IsDark(); dark != m.layout.dark {
		m.layout.dark = dark
		m.rebuildReviewView(m.review.changes)
	}
}

func (m *Model) resize(width, height int) {
	if m.layout.resize(width, height) {
		m.rebuildReviewView(m.review.changes)
		return
	}
	m.layout.keepCursorVisible(m.review.cursor)
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
	if message.changes.Fingerprint != m.review.changes.Fingerprint {
		m.rebuildReviewView(message.changes)
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

type reviewSnapshot struct {
	cursor       cursorIdentity
	first        cursorIdentity
	last         cursorIdentity
	hasSelection bool
	rowsAbove    int
}

func (m Model) identify(cursor Cursor) cursorIdentity {
	file, fileOK := m.layout.view.File(cursor)
	hunk, hunkOK := m.layout.view.Hunk(cursor)
	line, lineOK := m.layout.view.Line(cursor)
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: fileOK && hunkOK && lineOK}
}

func (m Model) snapshotReview() reviewSnapshot {
	snapshot := reviewSnapshot{
		cursor:    m.identify(m.review.cursor),
		rowsAbove: m.review.cursor.Coordinate.Y - m.layout.viewport.Top.Y,
	}
	if m.review.selection != nil {
		snapshot.first = m.identify(m.review.selection.First)
		snapshot.last = m.identify(m.review.selection.Last)
		snapshot.hasSelection = true
	}
	return snapshot
}

func (m *Model) rebuildReviewView(changes diff.ChangeSet) {
	snapshot := m.snapshotReview()
	m.review.changes = changes
	m.layout.view = m.layout.viewFor(changes)
	m.layout.viewport = m.layout.view.NewViewport(m.layout.size.Width, m.layout.bodyHeight())
	m.review.cursor = m.restoreCursor(snapshot.cursor)
	if snapshot.hasSelection {
		m.review.selection = m.restoreSelection(snapshot.first, snapshot.last)
	} else {
		m.review.selection = nil
	}
	m.layout.viewport.Top.Y = max(0, m.review.cursor.Coordinate.Y-snapshot.rowsAbove)
	m.layout.keepCursorVisible(m.review.cursor)
}

func (m Model) restoreCursor(identity cursorIdentity) Cursor {
	if identity.valid {
		if cursor, ok := m.layout.view.FindCursor(identity.file, identity.hunk, identity.line, identity.cursor.Coordinate, identity.cursor.Pane); ok {
			return cursor
		}
	}
	cursor, _ := m.layout.view.First()
	return cursor
}

func (m Model) restoreSelection(first, last cursorIdentity) *Selection {
	if !first.valid || !last.valid {
		return nil
	}
	translatedFirst, firstOK := m.layout.view.FindCursor(first.file, first.hunk, first.line, first.cursor.Coordinate, first.cursor.Pane)
	translatedLast, lastOK := m.layout.view.FindCursor(last.file, last.hunk, last.line, last.cursor.Coordinate, last.cursor.Pane)
	if !firstOK || !lastOK {
		return nil
	}
	firstHunk, firstHunkOK := m.layout.view.Hunk(translatedFirst)
	firstFile, firstFileOK := m.layout.view.File(translatedFirst)
	lastHunk, lastHunkOK := m.layout.view.Hunk(translatedLast)
	lastFile, lastFileOK := m.layout.view.File(translatedLast)
	if !firstHunkOK || !firstFileOK || !lastHunkOK || !lastFileOK {
		return nil
	}
	if !sameChangeFile(firstFile, lastFile) || firstHunk.Header != lastHunk.Header {
		return nil
	}
	return &Selection{First: translatedFirst, Last: translatedLast}
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
	return m.handleBrowseKey(name, pending)
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
			m.layout.viewport = m.layout.view.Align(m.layout.viewport, m.review.cursor, Middle)
		case "t":
			m.layout.viewport = m.layout.view.Align(m.layout.viewport, m.review.cursor, Top)
		case "b":
			m.layout.viewport = m.layout.view.Align(m.layout.viewport, m.review.cursor, Bottom)
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
			m.switchPane(m.review.cursor.Pane.Other())
		}
		return true
	}
	return false
}

func (m Model) handleBrowseKey(name, pending string) (tea.Model, tea.Cmd) {
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
		m.layout.viewport = m.layout.view.ScrollHorizontal(m.layout.viewport, -horizontalScrollStep)
	case "l", "right":
		m.layout.viewport = m.layout.view.ScrollHorizontal(m.layout.viewport, horizontalScrollStep)
	case "0":
		m.layout.viewport.LeftColumn = 0
	case "$":
		m.layout.viewport = m.layout.view.ScrollHorizontal(m.layout.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(Forward)
	case "ctrl+u":
		m.halfPage(Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			if cursor, ok := m.layout.view.First(); ok {
				m.setCursor(cursor)
			}
		} else {
			m.pendingKey = "g"
		}
	case "G":
		if cursor, ok := m.layout.view.Last(); ok {
			m.setCursor(cursor)
		}
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if m.review.selection == nil {
			selection := m.layout.view.BeginSelection(m.review.cursor)
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
