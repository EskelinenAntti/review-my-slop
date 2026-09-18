package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/diff"
)

type SaveCommentFunc func(comments.Comment) (comments.Comment, error)

type DeleteCommentFunc func(comments.Comment) error

type LoadCommentsFunc func() ([]comments.Comment, error)

// ViewMode selects the comparison shown by the model.
type ViewMode uint8

const (
	LocalChanges ViewMode = iota
	BranchChanges
)

// RefreshTarget identifies the comparison used by a refresh operation.
type RefreshTarget struct {
	Mode   ViewMode
	Branch string
}

type RefreshDiffFunc func(RefreshTarget) (diff.ChangeSet, error)

type Dependencies struct {
	SaveComment    SaveCommentFunc
	DeleteComment  DeleteCommentFunc
	LoadComments   LoadCommentsFunc
	RefreshDiff    RefreshDiffFunc
	SaveSideBySide func(bool) error
}

type Size struct {
	Width  int
	Height int
}

type Options struct {
	SideBySide    bool
	Size          Size
	DefaultBranch string
}

var DefaultSize = Size{Width: 80, Height: 30}

const (
	horizontalScrollStep   = 4
	minimumSideBySideWidth = 100
)

type refreshDiffMsg struct {
	changes diff.ChangeSet
	target  RefreshTarget
	request uint64
	err     error
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

type reviewState struct {
	changes    diff.ChangeSet
	present    *presentation
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
	deps          Dependencies
	err           error
	quitting      bool
	pendingKey    string
	defaultBranch string
	viewMode      ViewMode
	dark          bool
	refreshSerial uint64
	latestRefresh uint64
}

func New(changes diff.ChangeSet, pending []comments.Comment, deps Dependencies, options Options) Model {
	size := options.Size
	if size.Width <= 0 || size.Height <= 0 {
		size = DefaultSize
	}
	m := Model{
		review:        reviewState{changes: changes, sideBySide: options.SideBySide},
		comments:      commentState{items: append([]comments.Comment(nil), pending...), editIndex: -1},
		width:         size.Width,
		height:        size.Height,
		deps:          deps,
		defaultBranch: options.DefaultBranch,
		dark:          true,
	}
	m.review.present = &presentation{}
	m.review.present.newView(changes, m.sideBySideActive(), m.dark)
	m.review.viewport = m.review.present.NewViewport(m.width, m.screenBodyHeight())
	m.review.cursor, _ = m.review.present.First()
	return m
}

func (m Model) Init() tea.Cmd { return func() tea.Msg { return tea.RequestBackgroundColor() } }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := message.IsDark(); dark != m.dark {
			m.dark = dark
			m.rebuildView(m.review.changes)
		}
	case tea.WindowSizeMsg:
		activeBefore := m.sideBySideActive()
		m.width, m.height = message.Width, message.Height
		m.review.viewport = m.review.present.Resize(m.review.viewport, m.width, m.screenBodyHeight())
		if activeBefore != m.sideBySideActive() {
			m.rebuildView(m.review.changes)
		} else {
			m.review.viewport = m.review.present.KeepVisible(m.review.viewport, m.review.cursor)
		}
	case commentEditorFinishedMsg:
		if message.err != nil {
			m.err = message.err
			m.clearCommentEdit()
		} else {
			m.comments.body = message.body
			m.finishCommentEdit()
		}
	case commentsLoadedMsg:
		if message.revision != m.comments.revision {
			break
		}
		if message.err != nil {
			m.err = fmt.Errorf("refresh comments: %w", message.err)
		} else {
			m.comments.items = append([]comments.Comment(nil), message.comments...)
			m.comments.row = min(m.comments.row, max(0, len(m.comments.items)-1))
			m.err = nil
		}
	case sourceEditorFinishedMsg:
		if message.err != nil {
			m.err = fmt.Errorf("editor: %w", message.err)
			break
		}
		return m, m.loadRefresh()
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		if message.request != m.latestRefresh || message.target != m.currentTarget() {
			break
		}
		if message.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", message.err)
		} else if message.changes.Fingerprint != m.review.changes.Fingerprint {
			m.rebuildView(message.changes)
			m.err = nil
		}
	case tea.KeyPressMsg:
		return m.updateKey(message)
	}
	return m, nil
}

func (m *Model) loadRefresh() tea.Cmd {
	if m.deps.RefreshDiff == nil {
		return nil
	}
	m.refreshSerial++
	request := m.refreshSerial
	m.latestRefresh = request
	target := m.currentTarget()
	return func() tea.Msg {
		changes, err := m.deps.RefreshDiff(target)
		return refreshDiffMsg{changes: changes, target: target, request: request, err: err}
	}
}

func (m *Model) loadComments() tea.Cmd {
	if m.deps.LoadComments == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		items, err := m.deps.LoadComments()
		return commentsLoadedMsg{comments: items, revision: revision, err: err}
	}
}

func (m Model) identify(cursor Cursor) lineIdentity {
	return m.review.present.identify(cursor)
}

func (m *Model) rebuildView(changes diff.ChangeSet) {
	cursor := m.identify(m.review.cursor)
	var first, last lineIdentity
	if m.review.selection != nil {
		first, last = m.identify(m.review.selection.First), m.identify(m.review.selection.Last)
	}
	rowsAbove := m.review.cursor.Coordinate.Y - m.review.viewport.Top.Y
	m.review.changes = changes
	m.review.present = &presentation{}
	m.review.present.newView(changes, m.sideBySideActive(), m.dark)
	m.review.viewport = m.review.present.NewViewport(m.width, m.screenBodyHeight())
	if cursor.valid {
		m.review.cursor, cursor.valid = m.review.present.FindCursor(cursor.file, cursor.hunk, cursor.line, cursor.cursor.Coordinate, cursor.cursor.Pane)
	}
	if !cursor.valid {
		m.review.cursor, _ = m.review.present.First()
	}
	m.review.selection = nil
	if first.valid && last.valid {
		translatedFirst, firstOK := m.review.present.FindCursor(first.file, first.hunk, first.line, first.cursor.Coordinate, first.cursor.Pane)
		translatedLast, lastOK := m.review.present.FindCursor(last.file, last.hunk, last.line, last.cursor.Coordinate, last.cursor.Pane)
		if firstOK && lastOK {
			selection := Selection{First: translatedFirst, Last: translatedLast}
			firstHunk, hunkOK := m.review.present.hunk(translatedFirst)
			firstFile, fileOK := m.review.present.file(translatedFirst)
			lastFile, lastFileOK := m.review.present.file(translatedLast)
			lastHunk, lastHunkOK := m.review.present.hunk(translatedLast)
			if hunkOK && fileOK && lastFileOK && lastHunkOK && firstFile.Key() == lastFile.Key() && firstHunk.Header == lastHunk.Header {
				m.review.selection = &selection
			}
		}
	}
	m.review.viewport.Top.Y = max(0, m.review.cursor.Coordinate.Y-rowsAbove)
	m.review.viewport = m.review.present.KeepVisible(m.review.viewport, m.review.cursor)
}

func (m Model) currentTarget() RefreshTarget {
	if m.viewMode == BranchChanges {
		return RefreshTarget{Mode: BranchChanges, Branch: m.defaultBranch}
	}
	return RefreshTarget{Mode: LocalChanges}
}

func (m Model) sideBySideActive() bool {
	return m.review.sideBySide && m.width >= minimumSideBySideWidth
}

func (m Model) screenBodyHeight() int { return max(1, m.height-3) }
