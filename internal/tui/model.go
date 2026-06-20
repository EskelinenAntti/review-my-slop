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
type RefreshDiffFunc func(parent string) (patch.Patch, error)
type SaveSideBySideFunc func(bool) error

type refreshDiffMsg struct {
	patch  patch.Patch
	parent string
	err    error
}

type commentEditorFinishedMsg struct {
	body string
	err  error
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

type Model struct {
	patch       patch.Patch
	view        view.View
	cursor      view.Cursor
	viewport    view.Viewport
	selection   *view.Selection
	comments    []review.Comment
	commentRow  int
	width       int
	height      int
	mode        mode
	commentBody string
	editIndex   int
	editAnchor  review.Anchor
	save        SaveCommentFunc
	delete      DeleteCommentFunc
	refresh     RefreshDiffFunc
	err         error
	quitting    bool
	pendingKey  string
	sideBySide  bool
	saveLayout  SaveSideBySideFunc
	parents     []string
	target      int
	search      []rune
	searchTerm  string
	searchFrom  view.Cursor
	searchMiss  bool
	dark        bool
}

func New(p patch.Patch, comments []review.Comment, save SaveCommentFunc) Model {
	m := Model{patch: p, comments: comments, width: 100, height: 30, editIndex: -1, save: save, dark: true}
	m.view = view.NewUnifiedView(p, m.dark)
	m.viewport = m.view.NewViewport(m.width, m.viewportHeight())
	m.cursor, _ = m.view.First()
	return m
}

func (m *Model) SetRefresh(refresh RefreshDiffFunc) { m.refresh = refresh }
func (m *Model) SetDelete(delete DeleteCommentFunc) { m.delete = delete }
func (m *Model) SetParents(parents []string) {
	m.parents = append([]string(nil), parents...)
	m.target = min(m.target, len(m.parents))
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
			m.rebuildView(m.patch)
		}
	case tea.WindowSizeMsg:
		activeBefore := m.sideBySideActive()
		m.width, m.height = msg.Width, msg.Height
		m.viewport = m.view.Resize(m.viewport, m.width, m.viewportHeight())
		if activeBefore != m.sideBySideActive() {
			m.rebuildView(m.patch)
		} else {
			m.viewport = m.view.KeepVisible(m.viewport, m.cursor)
		}
	case commentEditorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.clearCommentEdit()
		} else {
			m.commentBody = msg.body
			m.finishCommentEdit()
		}
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
		}
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		if msg.parent != m.currentParent() {
			return m, nil
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", msg.err)
		} else if msg.patch.Fingerprint != m.patch.Fingerprint {
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
	parent := m.currentParent()
	return func() tea.Msg { p, err := m.refresh(parent); return refreshDiffMsg{patch: p, parent: parent, err: err} }
}

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor view.Cursor
	valid  bool
}

func (m Model) identify(cursor view.Cursor) cursorIdentity {
	file, fileOK := m.view.File(cursor)
	hunk, hunkOK := m.view.Hunk(cursor)
	line, lineOK := m.view.Line(cursor)
	return cursorIdentity{file: file, hunk: hunk, line: line, cursor: cursor, valid: fileOK && hunkOK && lineOK}
}

func (m *Model) rebuildView(p patch.Patch) {
	cursor := m.identify(m.cursor)
	var first, last cursorIdentity
	if m.selection != nil {
		first, last = m.identify(m.selection.First), m.identify(m.selection.Last)
	}
	rowsAbove := m.cursor.Coordinate.Y - m.viewport.Top.Y
	m.patch = p
	if m.sideBySideActive() {
		m.view = view.NewSplitView(p, m.dark)
	} else {
		m.view = view.NewUnifiedView(p, m.dark)
	}
	m.viewport = m.view.NewViewport(m.width, m.viewportHeight())
	if cursor.valid {
		m.cursor, cursor.valid = m.view.FindCursor(cursor.file, cursor.hunk, cursor.line, cursor.cursor.Coordinate, cursor.cursor.Pane)
	}
	if !cursor.valid {
		m.cursor, _ = m.view.First()
	}
	m.selection = nil
	if first.valid && last.valid {
		translatedFirst, firstOK := m.view.FindCursor(first.file, first.hunk, first.line, first.cursor.Coordinate, first.cursor.Pane)
		translatedLast, lastOK := m.view.FindCursor(last.file, last.hunk, last.line, last.cursor.Coordinate, last.cursor.Pane)
		if firstOK && lastOK {
			selection := view.Selection{First: translatedFirst, Last: translatedLast}
			if firstHunk, ok := m.view.Hunk(translatedFirst); ok {
				if firstFile, fileOK := m.view.File(translatedFirst); fileOK {
					if lastFile, lastFileOK := m.view.File(translatedLast); lastFileOK && samePatchFile(firstFile, lastFile) {
						if lastHunk, lastOK := m.view.Hunk(translatedLast); lastOK && firstHunk.Header == lastHunk.Header {
							m.selection = &selection
						}
					}
				}
			}
		}
	}
	m.viewport.Top.Y = max(0, m.cursor.Coordinate.Y-rowsAbove)
	m.viewport = m.view.KeepVisible(m.viewport, m.cursor)
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
			m.viewport = m.view.Align(m.viewport, m.cursor, view.Middle)
		case "t":
			m.viewport = m.view.Align(m.viewport, m.cursor, view.Top)
		case "b":
			m.viewport = m.view.Align(m.viewport, m.cursor, view.Bottom)
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
			m.switchPane(m.cursor.Pane.Other())
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
		m.search = nil
		m.searchFrom = m.cursor
		m.searchMiss = false
	case "n":
		m.repeatSearch(view.Forward)
	case "N":
		m.repeatSearch(view.Backward)
	case "j", "down":
		m.move(view.Forward)
	case "k", "up":
		m.move(view.Backward)
	case "h", "left":
		m.viewport = m.view.ScrollHorizontal(m.viewport, -horizontalScrollStep)
	case "l", "right":
		m.viewport = m.view.ScrollHorizontal(m.viewport, horizontalScrollStep)
	case "0":
		m.viewport.LeftColumn = 0
	case "$":
		m.viewport = m.view.ScrollHorizontal(m.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(view.Forward)
	case "ctrl+u":
		m.halfPage(view.Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			if cursor, ok := m.view.First(); ok {
				m.setCursor(cursor)
			}
		} else {
			m.pendingKey = "g"
		}
	case "G":
		if cursor, ok := m.view.Last(); ok {
			m.setCursor(cursor)
		}
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if m.selection == nil {
			selection := m.view.BeginSelection(m.cursor)
			m.selection = &selection
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
		m.commentRow = min(m.commentRow, max(0, len(m.comments)-1))
	case "R":
		return m, m.loadRefresh()
	case "tab":
		m.target = (m.target + 1) % (len(m.parents) + 1)
		m.cancelSelection()
		return m, m.loadRefresh()
	case "t":
		m.toggleSideBySide()
	}
	return m, nil
}

func (m Model) currentParent() string {
	if m.target <= 0 || m.target > len(m.parents) {
		return ""
	}
	return m.parents[m.target-1]
}
func (m Model) viewportHeight() int { return max(1, m.height-3) }
