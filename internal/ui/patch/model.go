package patch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/layout"
	"github.com/eskelinenantti/review-my-slop/internal/navigation"
	corepatch "github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/render"
)

const (
	horizontalScrollStep   = 4
	minimumSideBySideWidth = 100
)

type Initial struct {
	Patch         corepatch.Patch
	DefaultBranch string
	SideBySide    bool
	Width, Height int
}

type Dependencies struct {
	Load       func(base string) (corepatch.Patch, error)
	SaveLayout func(bool) error
}

type CommentRequested struct{ Anchor corepatch.Anchor }
type CommentsRequested struct{}
type HelpRequested struct{}
type QuitRequested struct{}
type CommentSaved struct{}

type screenMode uint8

const (
	modeBrowse screenMode = iota
	modeSearch
)

type refreshResult struct {
	patch      corepatch.Patch
	branch     string
	generation uint64
	err        error
}
type sourceEditorResult struct{ err error }

type Model struct {
	patch         corepatch.Patch
	defaultBranch string
	showDefault   bool
	sideBySide    bool
	width, height int
	dark          bool
	doc           *layout.Document
	nav           *navigation.Navigation
	deps          Dependencies
	mode          screenMode
	searchQuery   []rune
	searchTerm    string
	searchFrom    *layout.Cell
	searchMiss    bool
	pendingKey    string
	generation    uint64
	err           error
}

func New(initial Initial, dependencies Dependencies) *Model {
	width, height := initial.Width, initial.Height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 30
	}
	m := &Model{
		patch:         initial.Patch,
		defaultBranch: initial.DefaultBranch,
		sideBySide:    initial.SideBySide,
		width:         width,
		height:        height,
		dark:          true,
		deps:          dependencies,
	}
	m.rebuild(false)
	return m
}

func (m *Model) Resize(width, height int) {
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	oldFormat := m.format()
	m.width, m.height = width, height
	if m.nav != nil {
		m.nav.Resize(width, m.bodyHeight())
	}
	if oldFormat != m.format() {
		m.rebuild(true)
	}
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
	case tea.WindowSizeMsg:
		m.Resize(msg.Width, msg.Height)
	case tea.FocusMsg:
		return m.requestRefresh()
	case refreshResult:
		if msg.generation != m.generation || msg.branch != m.currentBranch() {
			return nil
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", msg.err)
			return nil
		}
		if msg.patch.Fingerprint != m.patch.Fingerprint {
			m.patch = msg.patch
			m.rebuild(true)
		}
		m.err = nil
	case sourceEditorResult:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			return nil
		}
		return m.requestRefresh()
	case CommentSaved:
		m.nav.CancelSelection()
	case Failure:
		m.err = msg.Err
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return nil
}

func (m *Model) requestRefresh() tea.Cmd {
	if m.deps.Load == nil {
		return nil
	}
	m.generation++
	generation, branch, load := m.generation, m.currentBranch(), m.deps.Load
	return func() tea.Msg {
		p, err := load(branch)
		return refreshResult{patch: p, branch: branch, generation: generation, err: err}
	}
}

func (m *Model) currentBranch() string {
	if m.showDefault {
		return m.defaultBranch
	}
	return ""
}
func (m *Model) bodyHeight() int { return max(1, m.height-3) }
func (m *Model) format() layout.Format {
	if m.sideBySide && m.width >= minimumSideBySideWidth {
		return layout.Split
	}
	return layout.Unified
}
func (m *Model) rebuild(preserve bool) {
	m.doc = layout.Build(m.patch, m.format())
	if m.nav == nil {
		m.nav = navigation.New(m.doc, m.width, m.bodyHeight())
	} else if preserve {
		m.nav.Replace(m.doc)
	} else {
		m.nav = navigation.New(m.doc, m.width, m.bodyHeight())
	}
}

func (m *Model) updateKey(key tea.KeyPressMsg) tea.Cmd {
	name := key.String()
	if m.mode == modeSearch {
		return m.updateSearch(name, key)
	}
	m.err = nil
	pending := m.pendingKey
	m.pendingKey = ""
	if pending == "[" || pending == "]" {
		if pending+name == "]f" {
			m.nav.JumpFile(layout.Forward)
		}
		if pending+name == "[f" {
			m.nav.JumpFile(layout.Backward)
		}
		return nil
	}
	if pending == "z" {
		switch name {
		case "z":
			m.nav.Align(navigation.Middle)
		case "t":
			m.nav.Align(navigation.Top)
		case "b":
			m.nav.Align(navigation.Bottom)
		}
		return nil
	}
	if pending == "ctrl+w" {
		switch name {
		case "h":
			m.nav.SwitchPane(layout.Left)
		case "l":
			m.nav.SwitchPane(layout.Right)
		case "ctrl+w":
			if snap := m.nav.Snapshot(); snap.Cursor != nil {
				m.nav.SwitchPane(snap.Cursor.Pane.Other())
			}
		}
		return nil
	}
	switch name {
	case "q", "ctrl+c":
		return event(QuitRequested{})
	case "?":
		return event(HelpRequested{})
	case "/":
		m.nav.CancelSelection()
		m.mode = modeSearch
		m.searchQuery = nil
		m.searchFrom = cloneCell(m.nav.Snapshot().Cursor)
		m.searchMiss = false
	case "n":
		m.repeatSearch(layout.Forward)
	case "N":
		m.repeatSearch(layout.Backward)
	case "j", "down":
		m.nav.Move(layout.Forward)
	case "k", "up":
		m.nav.Move(layout.Backward)
	case "h", "left":
		m.nav.ScrollHorizontal(-horizontalScrollStep)
	case "l", "right":
		m.nav.ScrollHorizontal(horizontalScrollStep)
	case "0":
		m.nav.ScrollHorizontal(-int(^uint(0) >> 1))
	case "$":
		m.nav.ScrollHorizontal(int(^uint(0) >> 1))
	case "ctrl+d":
		m.nav.HalfPage(layout.Forward)
	case "ctrl+u":
		m.nav.HalfPage(layout.Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			m.nav.First()
		} else {
			m.pendingKey = "g"
		}
	case "G":
		m.nav.Last()
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if m.nav.Snapshot().Selection == nil {
			m.nav.BeginSelection()
		} else {
			m.nav.CancelSelection()
		}
	case "esc":
		m.nav.CancelSelection()
	case "c":
		return m.commentRequest()
	case "e":
		return m.openCurrentLine()
	case "C":
		return event(CommentsRequested{})
	case "R":
		return m.requestRefresh()
	case "tab":
		if m.defaultBranch == "" {
			return nil
		}
		m.showDefault = !m.showDefault
		m.nav.CancelSelection()
		return m.requestRefresh()
	case "t":
		enabled := !m.sideBySide
		if enabled && m.width < minimumSideBySideWidth {
			m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
			return nil
		}
		m.sideBySide = enabled
		m.rebuild(true)
		if m.deps.SaveLayout != nil {
			if err := m.deps.SaveLayout(enabled); err != nil {
				m.err = fmt.Errorf("save side-by-side preference: %w", err)
			}
		}
	}
	return nil
}

func event(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }
func cloneCell(cell *layout.Cell) *layout.Cell {
	if cell == nil {
		return nil
	}
	copy := *cell
	return &copy
}

func (m *Model) commentRequest() tea.Cmd {
	snapshot := m.nav.Snapshot()
	selection := snapshot.Selection
	if selection == nil {
		if snapshot.Cursor == nil {
			return nil
		}
		selection = &layout.Selection{First: *snapshot.Cursor, Last: *snapshot.Cursor}
	}
	r, err := m.doc.Range(*selection)
	if err != nil {
		m.err = err
		return nil
	}
	anchor, err := m.patch.Anchor(r)
	if err != nil {
		m.err = err
		return nil
	}
	return event(CommentRequested{Anchor: anchor})
}

func (m *Model) openCurrentLine() tea.Cmd {
	command := strings.TrimSpace(os.Getenv("EDITOR"))
	if command == "" {
		m.err = fmt.Errorf("$EDITOR is not set")
		return nil
	}
	snapshot := m.nav.Snapshot()
	if snapshot.Cursor == nil {
		m.err = fmt.Errorf("select a code line to open in $EDITOR")
		return nil
	}
	position, ok := m.doc.Position(*snapshot.Cursor)
	if !ok {
		m.err = fmt.Errorf("select a code line to open in $EDITOR")
		return nil
	}
	location, err := m.patch.SourceLocation(position)
	if err != nil {
		m.err = err
		return nil
	}
	path := location.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.patch.Repository, filepath.FromSlash(path))
	}
	return tea.ExecProcess(editor.Command(command, path, location.Line), func(err error) tea.Msg { return sourceEditorResult{err: err} })
}

// Failure reports an asynchronous failure from a Patch initiated operation.
type Failure struct{ Err error }

func (m *Model) updateSearch(name string, key tea.KeyPressMsg) tea.Cmd {
	switch name {
	case "esc":
		if m.searchFrom != nil {
			m.nav.Jump(*m.searchFrom)
		}
		m.mode = modeBrowse
		m.searchQuery = nil
		m.searchMiss = false
	case "enter":
		if len(m.searchQuery) > 0 && !m.searchMiss {
			m.searchTerm = string(m.searchQuery)
		}
		m.mode = modeBrowse
		m.searchQuery = nil
		m.searchMiss = false
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		}
		m.updateIncrementalSearch()
	default:
		if key.Text != "" {
			m.searchQuery = append(m.searchQuery, []rune(key.Text)...)
			m.updateIncrementalSearch()
		}
	}
	return nil
}

func (m *Model) updateIncrementalSearch() {
	if len(m.searchQuery) == 0 {
		if m.searchFrom != nil {
			m.nav.Jump(*m.searchFrom)
		}
		m.searchMiss = false
		return
	}
	from := m.searchFrom
	if from == nil {
		m.searchMiss = true
		return
	}
	match, ok := m.doc.Find(string(m.searchQuery), *from, layout.Forward)
	m.searchMiss = !ok
	if ok {
		m.nav.Jump(match)
	}
}

func (m *Model) repeatSearch(direction layout.Direction) {
	if m.searchTerm == "" {
		return
	}
	snapshot := m.nav.Snapshot()
	if snapshot.Cursor == nil {
		return
	}
	match, ok := m.doc.Find(m.searchTerm, *snapshot.Cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.searchTerm)
		return
	}
	m.nav.Jump(match)
}
