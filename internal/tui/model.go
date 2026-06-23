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
)

type reviewState struct {
	patch      patch.Patch
	view       view.View
	cursor     view.Cursor
	viewport   view.Viewport
	selection  *view.Selection
	sideBySide bool
}

type commentState struct {
	items      []review.Comment
	row        int
	body       string
	editIndex  int
	editAnchor review.Anchor
	revision   uint64
}

type searchState struct {
	query []rune
	term  string
	from  view.Cursor
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

func New(p patch.Patch, comments []review.Comment, save SaveCommentFunc) Model {
	m := Model{
		review:   reviewState{patch: p},
		comments: commentState{items: comments, editIndex: -1},
		width:    100,
		height:   30,
		save:     save,
		dark:     true,
	}
	m.review.view = view.NewUnifiedView(p, m.dark)
	m.review.viewport = m.review.view.NewViewport(m.width, m.screenBodyHeight())
	m.review.cursor, _ = m.review.view.First()
	return m
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
		}
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

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor view.Cursor
	valid  bool
}

func (m Model) identify(cursor view.Cursor) cursorIdentity {
	file, fileOK := m.review.view.File(cursor)
	hunk, hunkOK := m.review.view.Hunk(cursor)
	line, lineOK := m.review.view.Line(cursor)
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: fileOK && hunkOK && lineOK}
}

func (m *Model) rebuildView(p patch.Patch) {
	cursor := m.identify(m.review.cursor)
	var first, last cursorIdentity
	if m.review.selection != nil {
		first, last = m.identify(m.review.selection.First), m.identify(m.review.selection.Last)
	}
	rowsAbove := m.review.cursor.Coordinate.Y - m.review.viewport.Top.Y
	m.review.patch = p
	if m.sideBySideActive() {
		m.review.view = view.NewSplitView(p, m.dark)
	} else {
		m.review.view = view.NewUnifiedView(p, m.dark)
	}
	m.review.viewport = m.review.view.NewViewport(m.width, m.screenBodyHeight())
	if cursor.valid {
		m.review.cursor, cursor.valid = m.review.view.FindCursor(cursor.file, cursor.hunk, cursor.line, cursor.cursor.Coordinate, cursor.cursor.Pane)
	}
	if !cursor.valid {
		m.review.cursor, _ = m.review.view.First()
	}
	m.review.selection = nil
	if first.valid && last.valid {
		translatedFirst, firstOK := m.review.view.FindCursor(first.file, first.hunk, first.line, first.cursor.Coordinate, first.cursor.Pane)
		translatedLast, lastOK := m.review.view.FindCursor(last.file, last.hunk, last.line, last.cursor.Coordinate, last.cursor.Pane)
		if firstOK && lastOK {
			selection := view.Selection{First: translatedFirst, Last: translatedLast}
			if firstHunk, ok := m.review.view.Hunk(translatedFirst); ok {
				if firstFile, fileOK := m.review.view.File(translatedFirst); fileOK {
					if lastFile, lastFileOK := m.review.view.File(translatedLast); lastFileOK && samePatchFile(firstFile, lastFile) {
						if lastHunk, lastOK := m.review.view.Hunk(translatedLast); lastOK && firstHunk.Header == lastHunk.Header {
							m.review.selection = &selection
						}
					}
				}
			}
		}
	}
	m.review.viewport.Top.Y = max(0, m.review.cursor.Coordinate.Y-rowsAbove)
	m.review.viewport = m.review.view.KeepVisible(m.review.viewport, m.review.cursor)
}

func samePatchFile(first, last patch.File) bool {
	return first.OldPath == last.OldPath && first.NewPath == last.NewPath
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
			m.jumpFile(view.Forward)
		}
		if pending+name == "[f" {
			m.jumpFile(view.Backward)
		}
		return m, nil
	}
	if pending == "z" {
		switch name {
		case "z":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, view.Middle)
		case "t":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, view.Top)
		case "b":
			m.review.viewport = m.review.view.Align(m.review.viewport, m.review.cursor, view.Bottom)
		}
		return m, nil
	}
	if pending == "ctrl+w" {
		switch name {
		case "h":
			m.switchPane(view.Left)
		case "l":
			m.switchPane(view.Right)
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
