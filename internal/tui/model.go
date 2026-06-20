package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/view"
	"github.com/eskelinenantti/review-my-slop/internal/xdg"
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

func (m *Model) move(direction view.Direction) {
	next, ok := m.view.Move(m.cursor, direction)
	if !ok {
		return
	}
	if m.selection != nil {
		selection, selectionOK := m.view.ExtendSelection(*m.selection, next)
		if !selectionOK {
			return
		}
		m.selection = &selection
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor view.Cursor) {
	m.cursor = cursor
	m.viewport = m.view.KeepVisible(m.viewport, cursor)
}

func (m *Model) halfPage(direction view.Direction) {
	viewport, cursor := m.view.ScrollHalfPage(m.viewport, m.cursor, direction)
	if m.selection != nil {
		selection, ok := m.view.ExtendSelection(*m.selection, cursor)
		if !ok {
			return
		}
		m.selection = &selection
	}
	m.viewport, m.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction view.Direction) {
	m.cancelSelection()
	if cursor, ok := m.view.JumpFile(m.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane view.Pane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.view.SwitchPane(m.cursor, pane)
	if !ok {
		return
	}
	if m.selection != nil {
		first, firstOK := m.view.SwitchPane(m.selection.First, pane)
		last, lastOK := m.view.SwitchPane(m.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.view.BeginSelection(first)
		selection, ok = m.view.ExtendSelection(selection, last)
		if !ok {
			return
		}
		m.selection = &selection
	}
	m.setCursor(cursor)
}

func (m Model) sideBySideActive() bool { return m.sideBySide && m.width >= minimumSideBySideWidth }

func (m *Model) toggleSideBySide() {
	enabled := !m.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *Model) setSideBySide(enabled bool) {
	wasActive := m.sideBySideActive()
	m.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.patch)
	}
}

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.setCursor(m.searchFrom)
		m.mode = modeBrowse
		m.search = nil
		m.searchMiss = false
	case "enter":
		if len(m.search) > 0 && !m.searchMiss {
			m.searchTerm = string(m.search)
		}
		m.mode = modeBrowse
		m.search = nil
		m.searchMiss = false
	case "backspace":
		if len(m.search) > 0 {
			m.search = m.search[:len(m.search)-1]
		}
		m.updateIncrementalSearch()
	default:
		if key.Text != "" {
			m.search = append(m.search, []rune(key.Text)...)
			m.updateIncrementalSearch()
		}
	}
	return m, nil
}

func (m *Model) updateIncrementalSearch() {
	if len(m.search) == 0 {
		m.setCursor(m.searchFrom)
		m.searchMiss = false
		return
	}
	match, ok := m.view.Search(string(m.search), m.searchFrom, view.Forward)
	m.searchMiss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction view.Direction) {
	if m.searchTerm == "" {
		return
	}
	match, ok := m.view.Search(m.searchTerm, m.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.searchTerm)
		return
	}
	m.setCursor(match)
}

func (m Model) updateComments(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		if m.commentRow < len(m.comments)-1 {
			m.commentRow++
		}
	case "k", "up":
		if m.commentRow > 0 {
			m.commentRow--
		}
	case "enter", "e":
		if len(m.comments) > 0 {
			m.editIndex = m.commentRow
			m.commentBody = m.comments[m.editIndex].Body
			m.editAnchor = m.comments[m.editIndex].Anchor
			cmd, err := m.openCommentEditor()
			if err != nil {
				m.err = err
				m.clearCommentEdit()
				return m, nil
			}
			return m, cmd
		}
	case "D":
		if len(m.comments) > 0 {
			m.deleteComment(m.commentRow)
		}
	}
	return m, nil
}

func (m *Model) beginComment() (tea.Cmd, error) {
	selection := m.selection
	if selection == nil {
		current := m.view.BeginSelection(m.cursor)
		selection = &current
	}
	anchor, err := m.view.Anchor(*selection)
	if err != nil {
		return nil, err
	}
	m.commentBody, m.editIndex, m.editAnchor = "", -1, anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.clearCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m *Model) finishCommentEdit() {
	body := strings.TrimSpace(m.commentBody)
	if body == "" {
		if m.editIndex >= 0 {
			m.deleteComment(m.editIndex)
		}
		m.clearCommentEdit()
		m.cancelSelection()
		return
	}
	if m.save == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		m.clearCommentEdit()
		return
	}
	var comment review.Comment
	if m.editIndex >= 0 {
		comment = m.comments[m.editIndex]
		comment.Body = body
	} else {
		comment = review.Comment{Anchor: m.editAnchor, Body: body}
	}
	saved, err := m.save(comment, m.patch)
	if err != nil {
		m.err = err
		m.clearCommentEdit()
		return
	}
	if m.editIndex >= 0 {
		m.comments[m.editIndex] = saved
	} else {
		m.comments = append(m.comments, saved)
		m.commentRow = len(m.comments) - 1
	}
	m.clearCommentEdit()
	m.err = nil
	m.cancelSelection()
}

func (m *Model) deleteComment(index int) {
	if index < 0 || index >= len(m.comments) {
		return
	}
	if m.delete == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	if err := m.delete(m.comments[index], m.patch); err != nil {
		m.err = err
		return
	}
	m.comments = append(m.comments[:index], m.comments[index+1:]...)
	m.commentRow = min(m.commentRow, max(0, len(m.comments)-1))
	m.err = nil
}

func (m *Model) clearCommentEdit() {
	m.commentBody = ""
	m.editIndex = -1
	m.editAnchor = review.Anchor{}
}
func (m *Model) cancelSelection() { m.selection = nil }

func (m Model) openCurrentLine() (tea.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	file, fileOK := m.view.File(m.cursor)
	line, lineOK := m.view.Line(m.cursor)
	if !fileOK || !lineOK {
		return nil, fmt.Errorf("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if path == "" || path == "/dev/null" || number < 1 {
		return nil, fmt.Errorf("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.patch.Repository, filepath.FromSlash(path))
	}
	return tea.ExecProcess(sourceEditorCommand(editor, path, int(number)), func(err error) tea.Msg { return sourceEditorFinishedMsg{err: err} }), nil
}

func (m Model) openCommentEditor() (tea.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	path, err := createCommentFile(m.commentBody, m.editAnchor)
	if err != nil {
		return nil, err
	}
	return tea.ExecProcess(commentEditorCommand(editor, path), func(editorErr error) tea.Msg { return readCommentEditorResult(path, m.editAnchor, editorErr) }), nil
}

func createCommentFile(body string, anchor review.Anchor) (string, error) {
	state, err := xdg.StateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(state, 0o700); err != nil {
		return "", fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(state, 0o700); err != nil {
		return "", fmt.Errorf("secure state directory: %w", err)
	}
	file, err := os.CreateTemp(state, "comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(commentEditorDraft(body, anchor)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close comment file: %w", err)
	}
	return path, nil
}

func readCommentEditorResult(path string, anchor review.Anchor, editorErr error) commentEditorFinishedMsg {
	defer os.Remove(path)
	if editorErr != nil {
		return commentEditorFinishedMsg{err: fmt.Errorf("editor: %w", editorErr)}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return commentEditorFinishedMsg{err: fmt.Errorf("read comment file: %w", err)}
	}
	return commentEditorFinishedMsg{body: stripUnchangedSuggestion(string(body), anchor.QuotedLines)}
}

func commentEditorDraft(body string, anchor review.Anchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := suggestionLines(anchor.QuotedLines)
	var draft strings.Builder
	draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		draft.WriteByte('\n')
	}
	draft.WriteByte('\n')
	fence := commentContextFence(lines)
	draft.WriteString(fence)
	draft.WriteString("suggestion\n")
	for _, line := range lines {
		draft.WriteString(line)
		draft.WriteByte('\n')
	}
	draft.WriteString(fence)
	draft.WriteByte('\n')
	return draft.String()
}

func suggestionLines(quoted []string) []string {
	lines := make([]string, 0, len(quoted))
	for _, line := range quoted {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func commentContextFence(lines []string) string {
	longest := 0
	for _, line := range lines {
		run := 0
		for _, char := range line {
			if char == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

func stripUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	lines := suggestionLines(quoted)
	fence := commentContextFence(lines)
	var suggestion strings.Builder
	suggestion.WriteString(fence)
	suggestion.WriteString("suggestion\n")
	for _, line := range lines {
		suggestion.WriteString(line)
		suggestion.WriteByte('\n')
	}
	suggestion.WriteString(fence)
	start := strings.Index(body, suggestion.String())
	if start < 0 {
		return body
	}
	end := start + suggestion.Len()
	before := strings.TrimRight(body[:start], "\n")
	after := body[end:]
	if strings.TrimSpace(after) == "" {
		return before
	}
	return before + "\n" + strings.TrimLeft(after, "\n")
}

func commentEditorCommand(editor, path string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" "+shellQuote(path))
}
func sourceEditorCommand(editor, path string, line int) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" +"+strconv.Itoa(line)+" "+shellQuote(path))
}
func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	result := tea.NewView(m.render())
	result.AltScreen = true
	result.ReportFocus = true
	return result
}

func (m Model) render() string {
	if m.mode == modeHelp {
		return m.renderHelp()
	}
	if m.mode == modeComments {
		return m.renderComments()
	}
	var out strings.Builder
	added, removed := patchLineCounts(m.patch)
	out.WriteString(titleStyle.Render("review-my-slop") + "  " + mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed)) + "\n")
	if len(m.patch.Files) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentParent() != "" {
			empty = "No branch or worktree changes."
		}
		lines := make([]string, m.viewportHeight())
		lines[min(1, len(lines)-1)] = mutedStyle.Render(empty)
		out.WriteString(strings.Join(lines, "\n"))
		out.WriteByte('\n')
	} else {
		out.WriteString(m.view.Render(m.viewport, m.cursor, m.selection))
		out.WriteByte('\n')
	}
	if m.err != nil {
		out.WriteString(m.renderFooter(errorStyle.Render(m.err.Error())))
	} else {
		out.WriteString(m.renderStatus())
	}
	out.WriteByte('\n')
	return out.String()
}

func patchLineCounts(p patch.Patch) (added, removed int) {
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind == patch.Addition {
					added++
				}
				if line.Kind == patch.Deletion {
					removed++
				}
			}
		}
	}
	return
}

func (m Model) renderStatus() string {
	status := "j/k/h/l move  c comment  ? help  q quit"
	if m.mode == modeSearch {
		status = "/" + string(m.search) + editorCursorStyle.Render(" ")
		if m.searchMiss {
			status += errorStyle.Render("  no matches")
		}
	} else if m.selection != nil {
		status = "visual selection  j/k extend  c comment  Esc cancel"
	}
	return m.renderFooter(mutedStyle.Render(status))
}

func (m Model) renderFooter(left string) string {
	right := mutedStyle.Render(m.viewLabel())
	width := max(20, m.width)
	rightWidth := lipgloss.Width(right)
	left = ansi.Truncate(left, max(0, width-rightWidth-1), "")
	return left + strings.Repeat(" ", max(1, width-lipgloss.Width(left)-rightWidth)) + right
}

func (m Model) viewLabel() string {
	if parent := m.currentParent(); parent != "" {
		return "branch changes from " + parent
	}
	return "local changes"
}
func (m Model) currentParent() string {
	if m.target <= 0 || m.target > len(m.parents) {
		return ""
	}
	return m.parents[m.target-1]
}
func (m Model) viewportHeight() int { return max(1, m.height-3) }

func (m Model) renderComments() string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("comments") + "  " + mutedStyle.Render(fmt.Sprintf("%d pending", len(m.comments))) + "\n")
	if len(m.comments) == 0 {
		out.WriteString("\n" + mutedStyle.Render("No pending comments.") + "\n")
	} else {
		for index, comment := range m.comments {
			prefix, style := "  ", contextStyle
			if index == m.commentRow {
				prefix, style = "> ", cursorStyle
			}
			location := comment.Anchor.FilePath
			if comment.Anchor.NewStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
			} else if comment.Anchor.OldStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
			}
			body := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
			out.WriteString(style.Width(max(20, m.width)).Render(fmt.Sprintf("%s%s  %s", prefix, location, body)) + "\n")
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if m.err != nil {
		footer = errorStyle.Render(m.err.Error())
	}
	out.WriteString(footer + "\n")
	return out.String()
}

func (m Model) renderHelp() string {
	bindings := []keyBinding{{"j/k, arrows", "move"}, {"h/l, left/right", "scroll horizontally"}, {"Ctrl-w h/l/w", "switch side-by-side pane"}, {"0/$", "start/end of lines"}, {"gg/G", "first/last changed line"}, {"zz/zt/zb", "center/top/bottom current line"}, {"Ctrl-d/Ctrl-u", "half-page down/up"}, {"/", "search diff text"}, {"n/N", "next/previous search match"}, {"]f/[f", "next/previous file"}, {"v", "select a line range"}, {"c", "comment on selection/current line"}, {"e", "open current line in $EDITOR"}, {"C", "view comments"}, {"R", "refresh diff"}, {"Tab", "cycle local/parent branch changes"}, {"t", "toggle unified/side-by-side"}, {"q", "quit"}}
	lines := []string{titleStyle.Render("review-my-slop help"), ""}
	lines = append(lines, renderKeyBindings(bindings)...)
	lines = append(lines, "", mutedStyle.Render("? or Esc closes help"))
	return strings.Join(lines, "\n")
}

type keyBinding struct{ keys, description string }

func renderKeyBindings(bindings []keyBinding) []string {
	width := 0
	for _, binding := range bindings {
		width = max(width, lipgloss.Width(binding.keys))
	}
	lines := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		lines = append(lines, binding.keys+strings.Repeat(" ", width-lipgloss.Width(binding.keys))+"  "+binding.description)
	}
	return lines
}

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	contextStyle      = lipgloss.NewStyle()
	cursorStyle       = lipgloss.NewStyle().Reverse(true)
	editorCursorStyle = lipgloss.NewStyle().Reverse(true)
	mutedStyle        = lipgloss.NewStyle().Faint(true)
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
