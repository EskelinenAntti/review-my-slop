package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/highlight"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/xdg"
)

type SaveCommentFunc func(review.StoredComment, review.Diff) (review.StoredComment, error)
type DeleteCommentFunc func(review.StoredComment, review.Diff) error
type RefreshDiffFunc func(parent string) (review.Diff, error)
type SaveSideBySideFunc func(bool) error

type refreshDiffMsg struct {
	diff   review.Diff
	parent string
	err    error
}

type commentEditorFinishedMsg struct {
	body string
	err  error
}

type sourceEditorFinishedMsg struct {
	err error
}

type rowKind uint8

const (
	rowFile rowKind = iota
	rowMetadata
	rowHunk
	rowCode
)

const horizontalScrollStep = 4

type row struct {
	kind      rowKind
	fileIndex int
	hunkIndex int
	lineIndex int
	text      string
	line      review.Line
}

type mode uint8

const (
	modeBrowse mode = iota
	modeComments
	modeHelp
	modeSearch
)

type Model struct {
	diff        review.Diff
	rows        []row
	comments    []review.StoredComment
	commentRow  int
	cursor      int
	viewportTop int
	xOffset     int
	width       int
	height      int
	selecting   bool
	selectFrom  int
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
	activePane  pane
	parents     []string
	target      int
	search      []rune
	searchTerm  string
	searchFrom  int
	searchPane  pane
	searchMiss  bool
	dark        bool
}

func New(diff review.Diff, comments []review.StoredComment, save SaveCommentFunc) Model {
	model := Model{
		diff:       diff,
		comments:   comments,
		width:      100,
		height:     30,
		selectFrom: -1,
		editIndex:  -1,
		save:       save,
		dark:       true,
	}
	model.rows = flatten(diff, model.dark)
	model.cursor = firstCodeRow(model.rows)
	return model
}

func (m *Model) SetRefresh(refresh RefreshDiffFunc) {
	m.refresh = refresh
}

func (m *Model) SetDelete(deleteComment DeleteCommentFunc) {
	m.delete = deleteComment
}

func (m *Model) SetParents(parents []string) {
	m.parents = append([]string(nil), parents...)
	m.target = min(m.target, len(m.parents))
}

func (m *Model) SetSideBySide(enabled bool, save SaveSideBySideFunc) {
	m.saveLayout = save
	m.setSideBySide(enabled)
}

func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		return tea.RequestBackgroundColor()
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		dark := msg.IsDark()
		if dark != m.dark {
			m.dark = dark
			m.applyDiff(m.diff)
		}
		return m, nil
	case tea.WindowSizeMsg:
		layout := m.visualLayout()
		cursorRow := layout.position(m.cursor) - m.viewportTop
		m.width = msg.Width
		m.height = msg.Height
		m.viewportTop = m.visualLayout().position(m.cursor) - cursorRow
		m.xOffset = min(m.xOffset, m.maxHorizontalOffset())
		m.ensureVisible()
		return m, nil
	case commentEditorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.clearCommentEdit()
		} else {
			m.commentBody = msg.body
			m.finishCommentEdit()
		}
		return m, nil
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
		}
		return m, nil
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		if msg.parent != m.currentParent() {
			return m, nil
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", msg.err)
		} else if msg.diff.Fingerprint != m.diff.Fingerprint {
			m.applyDiff(msg.diff)
			m.err = nil
		}
		return m, nil
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
	return func() tea.Msg {
		diff, err := m.refresh(parent)
		return refreshDiffMsg{diff: diff, parent: parent, err: err}
	}
}

func (m *Model) applyDiff(diff review.Diff) {
	layout := m.visualLayout()
	cursorRow := layout.position(m.cursor) - m.viewportTop
	cursor := m.rowAnchor(m.cursor)
	selection := m.rowAnchor(m.selectFrom)
	cursorFallback := m.cursor
	selectionFallback := m.selectFrom

	m.diff = diff
	m.rows = flatten(diff, m.dark)
	m.cursor = m.findRow(cursor, cursorFallback)
	if m.selecting {
		m.selectFrom = m.findRow(selection, selectionFallback)
		if !m.isCode(m.selectFrom) || !m.isCode(m.cursor) ||
			m.rows[m.selectFrom].fileIndex != m.rows[m.cursor].fileIndex ||
			m.rows[m.selectFrom].hunkIndex != m.rows[m.cursor].hunkIndex {
			m.cancelSelection()
		}
	}
	m.viewportTop = m.visualLayout().position(m.cursor) - cursorRow
	m.xOffset = min(m.xOffset, m.maxHorizontalOffset())
	m.ensureVisible()
}

type rowAnchor struct {
	valid     bool
	file      string
	kind      rowKind
	lineKind  review.LineKind
	oldNumber int
	newNumber int
	text      string
}

func (m Model) rowAnchor(index int) rowAnchor {
	if index < 0 || index >= len(m.rows) {
		return rowAnchor{}
	}
	current := m.rows[index]
	anchor := rowAnchor{
		valid: true,
		file:  m.diff.Files[current.fileIndex].Display,
		kind:  current.kind,
		text:  current.text,
	}
	if current.kind == rowCode {
		anchor.lineKind = current.line.Kind
		anchor.oldNumber = current.line.OldNumber
		anchor.newNumber = current.line.NewNumber
	}
	return anchor
}

func (m Model) findRow(anchor rowAnchor, fallback int) int {
	if !anchor.valid || len(m.rows) == 0 {
		return min(max(0, fallback), max(0, len(m.rows)-1))
	}
	for index, current := range m.rows {
		if m.rowMatches(anchor, current, true) {
			return index
		}
	}
	for index, current := range m.rows {
		if m.rowMatches(anchor, current, false) {
			return index
		}
	}
	return min(max(0, fallback), len(m.rows)-1)
}

func (m Model) rowMatches(anchor rowAnchor, current row, exact bool) bool {
	if current.fileIndex < 0 || current.fileIndex >= len(m.diff.Files) ||
		m.diff.Files[current.fileIndex].Display != anchor.file ||
		current.kind != anchor.kind {
		return false
	}
	if exact && current.kind == rowCode {
		return current.line.Kind == anchor.lineKind &&
			current.line.OldNumber == anchor.oldNumber &&
			current.line.NewNumber == anchor.newNumber
	}
	return current.text == anchor.text
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
	pendingKey := m.pendingKey
	m.pendingKey = ""
	if pendingKey == "[" || pendingKey == "]" {
		switch pendingKey + name {
		case "]f":
			m.jump(rowFile, 1)
		case "[f":
			m.jump(rowFile, -1)
		}
		return m, nil
	}
	if pendingKey == "z" {
		switch name {
		case "z":
			m.alignCursor(m.viewportHeight() / 2)
		case "t":
			m.alignCursor(0)
		case "b":
			m.alignCursor(m.viewportHeight() - 1)
		}
		return m, nil
	}
	if pendingKey == "ctrl+w" {
		switch name {
		case "h":
			m.switchSidePane(paneLeft)
		case "l":
			m.switchSidePane(paneRight)
		case "ctrl+w":
			m.switchSidePane(m.activePane.other())
		}
		return m, nil
	}
	switch name {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		m.cancelSelection()
		m.mode = modeSearch
		m.search = nil
		m.searchFrom = m.cursor
		m.searchPane = m.activePane
		m.searchMiss = false
	case "n":
		m.repeatSearch(1)
	case "N":
		m.repeatSearch(-1)
	case "j", "down":
		m.moveVertical(1)
	case "k", "up":
		m.moveVertical(-1)
	case "h", "left":
		m.scrollHorizontal(-horizontalScrollStep)
	case "l", "right":
		m.scrollHorizontal(horizontalScrollStep)
	case "0":
		m.xOffset = 0
	case "$":
		m.xOffset = m.maxHorizontalOffset()
	case "ctrl+d":
		m.moveHalfPage(1)
	case "ctrl+u":
		m.moveHalfPage(-1)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pendingKey == "g" {
			m.cursor = firstCodeRow(m.rows)
			m.ensureVisible()
		} else {
			m.pendingKey = "g"
		}
	case "G":
		m.cursor = lastCodeRow(m.rows)
		m.ensureVisible()
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if m.isCode(m.cursor) {
			if m.selecting {
				m.cancelSelection()
			} else {
				m.selecting = true
				m.selectFrom = m.cursor
			}
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

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.cursor = m.searchFrom
		m.activePane = m.searchPane
		m.ensureVisible()
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
		m.cursor = m.searchFrom
		m.activePane = m.searchPane
		m.searchMiss = false
		m.ensureVisible()
		return
	}
	match := m.findSearch(string(m.search), m.searchFrom, 1)
	m.searchMiss = match < 0
	if match >= 0 {
		m.setSearchCursor(match)
	}
}

func (m *Model) repeatSearch(direction int) {
	if m.searchTerm == "" {
		return
	}
	match := m.findSearch(m.searchTerm, m.cursor, direction)
	if match < 0 {
		m.err = fmt.Errorf("no matches for %q", m.searchTerm)
		return
	}
	m.setSearchCursor(match)
}

func (m *Model) setSearchCursor(index int) {
	m.cursor = index
	if m.sideBySideActive() && m.isCode(index) {
		switch m.rows[index].line.Kind {
		case review.LineRemoved:
			m.activePane = paneLeft
		case review.LineAdded:
			m.activePane = paneRight
		}
	}
	m.ensureVisible()
}

func (m Model) findSearch(query string, start, direction int) int {
	if query == "" || len(m.rows) == 0 {
		return -1
	}
	query = strings.ToLower(query)
	for step := 1; step <= len(m.rows); step++ {
		index := (start + direction*step) % len(m.rows)
		if index < 0 {
			index += len(m.rows)
		}
		if strings.Contains(strings.ToLower(ansi.Strip(m.rows[index].text)), query) {
			return index
		}
	}
	return -1
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
			m.commentBody = m.comments[m.editIndex].Comment.Body
			m.editAnchor = m.comments[m.editIndex].Comment.Anchor
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
	var stored review.StoredComment
	if m.editIndex >= 0 {
		stored = m.comments[m.editIndex]
		stored.Comment.Body = body
	} else {
		stored.Comment = review.Comment{Anchor: m.editAnchor, Body: body}
	}
	saved, err := m.save(stored, m.diff)
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
	deleted := m.comments[index]
	if err := m.delete(deleted, m.diff); err != nil {
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

func (m Model) openCommentEditor() (tea.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	path, err := createCommentFile(m.commentBody, m.editAnchor)
	if err != nil {
		return nil, err
	}

	command := commentEditorCommand(editor, path)
	return tea.ExecProcess(command, func(editorErr error) tea.Msg {
		return readCommentEditorResult(path, m.editAnchor, editorErr)
	}), nil
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
	draft.WriteString("\n")
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

func suggestionLines(quotedLines []string) []string {
	lines := make([]string, 0, len(quotedLines))
	for _, line := range quotedLines {
		if line == "" || line[0] == '-' {
			continue
		}
		lines = append(lines, line[1:])
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

func stripUnchangedSuggestion(body string, lines []string) string {
	if len(lines) == 0 {
		return body
	}

	lines = suggestionLines(lines)
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
	location := "+" + strconv.Itoa(line)
	return exec.Command("sh", "-c", editor+" "+location+" "+shellQuote(path))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func (m Model) openCurrentLine() (tea.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	if !m.isCode(m.cursor) {
		return nil, fmt.Errorf("select a code line to open in $EDITOR")
	}

	current := m.rows[m.cursor]
	file := m.diff.Files[current.fileIndex]
	path := file.NewPath
	line := current.line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if line == 0 {
		line = current.line.OldNumber
	}
	if path == "" || path == "/dev/null" || line < 1 {
		return nil, fmt.Errorf("current line has no editable working-tree location")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.diff.Repository, filepath.FromSlash(path))
	}

	return tea.ExecProcess(sourceEditorCommand(editor, path, line), func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	}), nil
}

func (m *Model) beginComment() (tea.Cmd, error) {
	if !m.isCode(m.cursor) {
		return nil, nil
	}
	anchor, err := m.currentAnchor()
	if err != nil {
		return nil, err
	}
	m.commentBody = ""
	m.editIndex = -1
	m.editAnchor = anchor
	cmd, err := m.openCommentEditor()
	if err != nil {
		m.clearCommentEdit()
		return nil, err
	}
	return cmd, nil
}

func (m Model) currentAnchor() (review.Anchor, error) {
	start, end := m.cursor, m.cursor
	if m.selecting {
		start, end = ordered(m.selectFrom, m.cursor)
	}
	if start < 0 || end >= len(m.rows) || !m.isCode(start) || !m.isCode(end) {
		return review.Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	first, last := m.rows[start], m.rows[end]
	if first.fileIndex != last.fileIndex || first.hunkIndex != last.hunkIndex {
		return review.Anchor{}, fmt.Errorf("a comment selection cannot cross a hunk")
	}
	file := m.diff.Files[first.fileIndex]
	hunk := file.Hunks[first.hunkIndex]
	anchor := review.Anchor{
		File:     file.Display,
		Hunk:     hunk.Header,
		StartRow: first.lineIndex,
		EndRow:   last.lineIndex,
	}
	for index := start; index <= end; index++ {
		current := m.rows[index]
		if current.kind != rowCode {
			return review.Anchor{}, fmt.Errorf("a comment selection cannot include headers")
		}
		prefix := " "
		switch current.line.Kind {
		case review.LineAdded:
			prefix = "+"
		case review.LineRemoved:
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+current.line.Text)
		accumulateRange(&anchor.OldStart, &anchor.OldEnd, current.line.OldNumber)
		accumulateRange(&anchor.NewStart, &anchor.NewEnd, current.line.NewNumber)
	}
	return anchor, nil
}

func (m Model) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	content := m.render()
	view := tea.NewView(content)
	view.AltScreen = true
	view.ReportFocus = true
	return view
}

func (m Model) render() string {
	if m.mode == modeHelp {
		return m.renderHelp()
	}
	if m.mode == modeComments {
		return m.renderComments()
	}
	var out strings.Builder
	title := titleStyle.Render("review-my-slop")
	added, removed := diffLineCounts(m.diff)
	summary := mutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	out.WriteString(title + "  " + summary + "\n")

	if len(m.rows) == 0 {
		empty := "No unstaged or untracked changes."
		if m.currentParent() != "" {
			empty = "No branch or worktree changes."
		}
		out.WriteString("\n" + mutedStyle.Render(empty) + "\n")
	} else {
		layout := m.visualLayout()
		height := m.viewportHeight()
		end := min(layout.len(), m.viewportTop+height)
		for position := m.viewportTop; position < end; position++ {
			out.WriteString(m.renderVisualRow(layout.row(position)))
			out.WriteByte('\n')
		}
		for position := end; position < m.viewportTop+height; position++ {
			out.WriteByte('\n')
		}
	}

	if m.err != nil {
		out.WriteString(m.renderFooter(errorStyle.Render(m.err.Error())) + "\n")
	} else {
		out.WriteString(m.renderStatus() + "\n")
	}
	return out.String()
}

func diffLineCounts(diff review.Diff) (added, removed int) {
	for _, file := range diff.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				switch line.Kind {
				case review.LineAdded:
					added++
				case review.LineRemoved:
					removed++
				}
			}
		}
	}
	return added, removed
}

func (m Model) renderRow(index int) string {
	layout := m.visualLayout()
	return m.renderVisualRow(layout.row(layout.position(index)))
}

func (m Model) renderVisualRow(visual visualRow) string {
	index := visual.source
	if index < 0 || index >= len(m.rows) {
		return ""
	}
	current := m.rows[index]
	width := max(20, m.width)
	switch current.kind {
	case rowFile:
		style := fileStyle
		if index == m.cursor {
			style = cursorStyle
		}
		return style.Width(width).Render(current.text)
	case rowMetadata:
		if index == m.cursor {
			return renderStyledRow(cursorStyle, "  "+current.text, width, true)
		}
		return metadataStyle.Render("  " + current.text)
	case rowHunk:
		if index == m.cursor {
			return renderStyledRow(cursorStyle, current.text, width, true)
		}
		return hunkStyle.Render(current.text)
	case rowCode:
		if m.sideBySideActive() {
			return m.renderSideBySide(visual)
		}
		oldNumber := number(current.line.OldNumber)
		newNumber := number(current.line.NewNumber)
		prefix := " "
		switch current.line.Kind {
		case review.LineAdded:
			prefix = addedStyle.Render("+")
		case review.LineRemoved:
			prefix = removedStyle.Render("-")
		}
		text := current.text
		gutter := fmt.Sprintf("%5s %5s %s ", oldNumber, newNumber, prefix)
		line := gutter + fitANSIWindow(text, m.xOffset, width-lipgloss.Width(gutter))
		style := lineStyle(current.line.Kind, m.dark)
		if m.selected(index) {
			style = selectionRowStyle(m.dark)
		}
		stripForeground := m.selected(index)
		if index == m.cursor {
			style = cursorStyle
			stripForeground = true
		}
		return renderStyledRow(style, line, width, stripForeground)
	default:
		return ""
	}
}

func (m Model) renderStatus() string {
	var status string
	if m.mode == modeSearch {
		status = "/" + string(m.search) + editorCursorStyle.Render(" ")
		if m.searchMiss {
			status += errorStyle.Render("  no matches")
		}
	} else if m.selecting {
		status = "visual selection  j/k extend  c comment  Esc cancel"
		status = mutedStyle.Render(status)
	} else {
		status = mutedStyle.Render("j/k/h/l move  c comment  ? help  q quit")
	}
	return m.renderFooter(status)
}

func (m Model) renderFooter(left string) string {
	right := mutedStyle.Render(m.viewLabel())
	width := max(20, m.width)
	rightWidth := lipgloss.Width(right)
	left = ansi.Truncate(left, max(0, width-rightWidth-1), "")
	padding := max(1, width-lipgloss.Width(left)-rightWidth)
	return left + strings.Repeat(" ", padding) + right
}

func (m Model) viewLabel() string {
	parent := m.currentParent()
	if parent == "" {
		return "local changes"
	}
	return "branch changes from " + parent
}

func (m Model) currentParent() string {
	if m.target <= 0 || m.target > len(m.parents) {
		return ""
	}
	return m.parents[m.target-1]
}

func (m Model) renderComments() string {
	var out strings.Builder
	out.WriteString(titleStyle.Render("comments"))
	out.WriteString("  ")
	out.WriteString(mutedStyle.Render(fmt.Sprintf("%d pending", len(m.comments))))
	out.WriteByte('\n')
	if len(m.comments) == 0 {
		out.WriteString("\n" + mutedStyle.Render("No pending comments.") + "\n")
	} else {
		for index, stored := range m.comments {
			prefix := "  "
			style := contextStyle
			if index == m.commentRow {
				prefix = "> "
				style = cursorStyle
			}
			comment := stored.Comment
			location := comment.Anchor.File
			if comment.Anchor.NewStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
			} else if comment.Anchor.OldStart > 0 {
				location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
			}
			body := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
			line := fmt.Sprintf("%s%s  %s", prefix, location, body)
			out.WriteString(renderStyledRow(style, line, max(20, m.width), index == m.commentRow))
			out.WriteByte('\n')
		}
	}
	footer := mutedStyle.Render("j/k move  Enter/e edit  D delete  Esc/q return")
	if m.err != nil {
		footer = errorStyle.Render(m.err.Error())
	}
	out.WriteString(footer)
	out.WriteByte('\n')
	return out.String()
}

func (m Model) renderHelp() string {
	bindings := []keyBinding{
		{"j/k, arrows", "move"},
		{"h/l, left/right", "scroll horizontally"},
		{"Ctrl-w h/l/w", "switch side-by-side pane"},
		{"0/$", "start/end of lines"},
		{"gg/G", "first/last changed line"},
		{"zz/zt/zb", "center/top/bottom current line"},
		{"Ctrl-d/Ctrl-u", "half-page down/up"},
		{"/", "search diff text"},
		{"n/N", "next/previous search match"},
		{"]f/[f", "next/previous file"},
		{"v", "select a line range"},
		{"c", "comment on selection/current line"},
		{"e", "open current line in $EDITOR"},
		{"C", "view comments"},
		{"R", "refresh diff"},
		{"Tab", "cycle local/parent branch changes"},
		{"t", "toggle unified/side-by-side"},
		{"q", "quit"},
	}
	lines := []string{titleStyle.Render("review-my-slop help"), ""}
	lines = append(lines, renderKeyBindings(bindings)...)
	lines = append(lines, "", mutedStyle.Render("? or Esc closes help"))
	return strings.Join(lines, "\n")
}

type keyBinding struct {
	keys        string
	description string
}

func renderKeyBindings(bindings []keyBinding) []string {
	width := 0
	for _, binding := range bindings {
		width = max(width, lipgloss.Width(binding.keys))
	}
	lines := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		padding := strings.Repeat(" ", width-lipgloss.Width(binding.keys))
		lines = append(lines, binding.keys+padding+"  "+binding.description)
	}
	return lines
}

func (m Model) renderSideBySide(visual visualRow) string {
	leftWidth := max(20, (m.width-3)/2)
	rightWidth := max(20, m.width-3-leftWidth)
	left := m.renderSidePane(visual.left, paneLeft, leftWidth)
	right := m.renderSidePane(visual.right, paneRight, rightWidth)
	return left + " │ " + right
}

func (m Model) renderSidePane(index int, currentPane pane, width int) string {
	if index < 0 {
		return strings.Repeat(" ", width)
	}
	current := m.rows[index]
	lineNumber := current.line.NewNumber
	text := "  " + current.text
	if currentPane == paneLeft {
		lineNumber = current.line.OldNumber
	}
	switch current.line.Kind {
	case review.LineRemoved:
		text = removedStyle.Render("-") + " " + current.text
	case review.LineAdded:
		text = addedStyle.Render("+") + " " + current.text
	}
	gutter := fmt.Sprintf("%5s ", number(lineNumber))
	line := gutter + fitANSIWindow(text, m.xOffset, width-lipgloss.Width(gutter))
	style := lineStyle(current.line.Kind, m.dark)
	stripForeground := false
	if m.selected(index) {
		style = selectionRowStyle(m.dark)
		stripForeground = true
	}
	if index == m.cursor && currentPane == m.activePane {
		style = cursorStyle
		stripForeground = true
	}
	return renderStyledRow(style, line, width, stripForeground)
}

func (m Model) visualLayout() visualLayout {
	return newVisualLayout(m.rows, m.sideBySideActive())
}

func (m *Model) switchSidePane(targetPane pane) {
	if !m.sideBySideActive() {
		return
	}
	layout := m.visualLayout()
	position := layout.position(m.cursor)
	target, ok := layout.paneIndexAtOrAbove(position, targetPane)
	if !ok {
		target, ok = layout.paneChangeIndexAtOrBelow(position, targetPane)
	}
	if !ok {
		return
	}
	if m.selecting {
		selectionPosition := layout.position(m.selectFrom)
		selection, selectionOK := layout.paneIndexAtOrAbove(selectionPosition, targetPane)
		if !selectionOK {
			selection, selectionOK = layout.paneChangeIndexAtOrBelow(selectionPosition, targetPane)
		}
		if !selectionOK {
			return
		}
		m.selectFrom = selection
	}
	m.activePane = targetPane
	m.cursor = target
	m.ensureVisible()
}

func (m *Model) moveVertical(direction int) {
	if len(m.rows) == 0 {
		return
	}
	layout := m.visualLayout()
	_, cursor := layout.navigable(layout.position(m.cursor), direction, m.activePane)
	if cursor < 0 {
		return
	}
	if m.selecting && !m.sameHunk(m.selectFrom, cursor) {
		return
	}
	m.cursor = cursor
	m.ensureVisible()
}

func (m Model) sideBySideActive() bool {
	return m.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *Model) toggleSideBySide() {
	enabled := !m.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	m.saveSideBySide()
}

func (m *Model) setSideBySide(enabled bool) {
	oldLayout := m.visualLayout()
	cursorRow := oldLayout.position(m.cursor) - m.viewportTop
	m.sideBySide = enabled
	if enabled {
		layout := m.visualLayout()
		m.activePane = paneRight
		m.cursor = layout.cursorIndex(layout.position(m.cursor), paneRight)
		if m.selecting {
			m.selectFrom = layout.cursorIndex(layout.position(m.selectFrom), paneRight)
		}
	}
	m.viewportTop = m.visualLayout().position(m.cursor) - cursorRow
	m.xOffset = min(m.xOffset, m.maxHorizontalOffset())
	m.ensureVisible()
}

func (m *Model) saveSideBySide() {
	if m.saveLayout == nil {
		return
	}
	if err := m.saveLayout(m.sideBySide); err != nil {
		m.err = fmt.Errorf("save side-by-side preference: %w", err)
	}
}

func flatten(diff review.Diff, darkBackground bool) []row {
	var rows []row
	for fileIndex, file := range diff.Files {
		rows = append(rows, row{kind: rowFile, fileIndex: fileIndex, hunkIndex: -1, lineIndex: -1, text: file.Display})
		for _, metadata := range file.Metadata {
			rows = append(rows, row{kind: rowMetadata, fileIndex: fileIndex, hunkIndex: -1, lineIndex: -1, text: metadata})
		}
		highlighted := highlight.Sources(file.Language, file.OldSource, file.NewSource, darkBackground)
		for hunkIndex, hunk := range file.Hunks {
			header := hunk.Header
			if !strings.HasPrefix(header, "@@") {
				header = "@@ " + header
			}
			rows = append(rows, row{kind: rowHunk, fileIndex: fileIndex, hunkIndex: hunkIndex, lineIndex: -1, text: header})
			for lineIndex, line := range hunk.Lines {
				text := line.Text
				switch line.Kind {
				case review.LineRemoved:
					text = highlightedLine(highlighted.Old, line.OldNumber, text)
				default:
					text = highlightedLine(highlighted.New, line.NewNumber, text)
				}
				rows = append(rows, row{
					kind: rowCode, fileIndex: fileIndex, hunkIndex: hunkIndex,
					lineIndex: lineIndex, line: line, text: text,
				})
			}
		}
	}
	return rows
}

func lineStyle(kind review.LineKind, darkBackground bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(darkBackground)
	switch kind {
	case review.LineAdded:
		return lipgloss.NewStyle().Background(lightDark(
			lipgloss.Color("#dafbe1"),
			lipgloss.Color("#1b3823"),
		))
	case review.LineRemoved:
		return lipgloss.NewStyle().Background(lightDark(
			lipgloss.Color("#ffebe9"),
			lipgloss.Color("#402222"),
		))
	default:
		return contextStyle
	}
}

func selectionRowStyle(darkBackground bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(darkBackground)
	return lipgloss.NewStyle().Background(lightDark(
		lipgloss.Color("#dbeafe"),
		lipgloss.Color("#1e3a5f"),
	))
}

func highlightedLine(lines []string, number int, fallback string) string {
	if number <= 0 || number > len(lines) {
		return fallback
	}
	return lines[number-1]
}

func (m *Model) moveHalfPage(direction int) {
	if len(m.rows) == 0 {
		return
	}

	layout := m.visualLayout()
	height := m.viewportHeight()
	delta := max(1, height/2) * direction
	maxOffset := max(0, layout.len()-height)
	offset := max(0, min(m.viewportTop+delta, maxOffset))
	cursorTarget := layout.position(m.cursor) + offset - m.viewportTop
	first, last := offset, min(layout.len()-1, offset+height-1)
	cursorTarget = max(first, min(cursorTarget, last))
	_, cursor := layout.codeNearBetween(cursorTarget, direction, first, last, m.activePane)
	if cursor < 0 {
		_, cursor = layout.codeNearBetween(cursorTarget, -direction, first, last, m.activePane)
	}
	if cursor < 0 || m.selecting && !m.sameHunk(m.selectFrom, cursor) {
		return
	}

	m.cursor = cursor
	m.viewportTop = offset
}

func (m *Model) jump(kind rowKind, direction int) {
	if len(m.rows) == 0 {
		return
	}
	m.cancelSelection()
	for index := m.cursor + direction; index >= 0 && index < len(m.rows); index += direction {
		if m.rows[index].kind == kind {
			if code := codeNear(m.rows, index, direction); code >= 0 {
				m.cursor = code
				m.ensureVisible()
			}
			return
		}
	}
}

func (m *Model) ensureVisible() {
	layout := m.visualLayout()
	height := m.viewportHeight()
	if layout.len() == 0 {
		m.viewportTop = 0
		return
	}

	cursor := layout.position(m.cursor)
	if cursor < m.viewportTop {
		m.viewportTop = cursor
	}
	if cursor >= m.viewportTop+height {
		m.viewportTop = cursor - height + 1
	}
	m.viewportTop = max(0, min(m.viewportTop, max(0, layout.len()-height)))
}

func (m *Model) alignCursor(viewportRow int) {
	layout := m.visualLayout()
	height := m.viewportHeight()
	if layout.len() == 0 {
		m.viewportTop = 0
		return
	}
	m.viewportTop = layout.position(m.cursor) - viewportRow
	m.viewportTop = max(0, min(m.viewportTop, max(0, layout.len()-height)))
}

func (m *Model) scrollHorizontal(delta int) {
	m.xOffset = max(0, min(m.xOffset+delta, m.maxHorizontalOffset()))
}

func (m Model) maxHorizontalOffset() int {
	contentWidth := max(1, m.width-14)
	extraWidth := 0
	if m.sideBySideActive() {
		contentWidth = max(1, (m.width-3)/2-6)
		extraWidth = 2
	}
	longest := 0
	for _, current := range m.rows {
		if current.kind != rowCode {
			continue
		}
		longest = max(longest, lipgloss.Width(expandTabs(current.text))+extraWidth)
	}
	return max(0, longest-contentWidth)
}

func (m Model) viewportHeight() int {
	return max(1, m.height-3)
}

func (m Model) isCode(index int) bool {
	return index >= 0 && index < len(m.rows) && m.rows[index].kind == rowCode
}

func (m Model) sameHunk(a, b int) bool {
	return m.isCode(a) && m.isCode(b) &&
		m.rows[a].fileIndex == m.rows[b].fileIndex &&
		m.rows[a].hunkIndex == m.rows[b].hunkIndex
}

func (m Model) selected(index int) bool {
	if !m.selecting {
		return false
	}
	start, end := ordered(m.selectFrom, m.cursor)
	return index >= start && index <= end && m.rows[index].kind == rowCode
}

func (m *Model) cancelSelection() {
	m.selecting = false
	m.selectFrom = -1
}

func firstCodeRow(rows []row) int {
	for index, row := range rows {
		if row.kind == rowCode {
			return index
		}
	}
	return 0
}

func lastCodeRow(rows []row) int {
	for index := len(rows) - 1; index >= 0; index-- {
		if rows[index].kind == rowCode {
			return index
		}
	}
	return 0
}

func codeNear(rows []row, start, direction int) int {
	for index := start; index >= 0 && index < len(rows); index += direction {
		if rows[index].kind == rowCode {
			return index
		}
	}
	return -1
}

func accumulateRange(start, end *int, value int) {
	if value == 0 {
		return
	}
	if *start == 0 || value < *start {
		*start = value
	}
	if value > *end {
		*end = value
	}
}

func number(value int) string {
	if value == 0 {
		return ""
	}
	return fmt.Sprintf("%d", value)
}

func ordered(a, b int) (int, int) {
	if a > b {
		return b, a
	}
	return a, b
}

func renderStyledRow(style lipgloss.Style, value string, width int, stripForeground bool) string {
	value = filterANSIColors(value, stripForeground)
	fitted := fitANSI(value, width)
	prefix := stylePrefix(style)
	if prefix != "" {
		// Syntax highlighters reset SGR state between tokens. Reapply the row
		// style after those resets so backgrounds and reverse-video cursors span
		// the complete terminal row.
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func filterANSIColors(value string, stripForeground bool) string {
	return ansiSGRPattern.ReplaceAllStringFunc(value, func(sequence string) string {
		parameters := sequence[2 : len(sequence)-1]
		if parameters == "" {
			return sequence
		}
		parts := strings.Split(parameters, ";")
		filtered := make([]string, 0, len(parts))
		for index := 0; index < len(parts); index++ {
			code, err := strconv.Atoi(parts[index])
			if err != nil {
				filtered = append(filtered, parts[index])
				continue
			}
			switch {
			case code == 48:
				if index+1 < len(parts) {
					switch parts[index+1] {
					case "2":
						index = min(index+4, len(parts)-1)
					case "5":
						index = min(index+2, len(parts)-1)
					}
				}
				continue
			case stripForeground && code == 38:
				if index+1 < len(parts) {
					switch parts[index+1] {
					case "2":
						index = min(index+4, len(parts)-1)
					case "5":
						index = min(index+2, len(parts)-1)
					}
				}
				continue
			case code >= 40 && code <= 49:
				continue
			case code >= 100 && code <= 107:
				continue
			case stripForeground && code >= 30 && code <= 39:
				continue
			case stripForeground && code >= 90 && code <= 97:
				continue
			default:
				filtered = append(filtered, parts[index])
			}
		}
		if len(filtered) == 0 {
			return ""
		}
		return "\x1b[" + strings.Join(filtered, ";") + "m"
	})
}

func fitANSI(value string, width int) string {
	return fitANSIWindow(value, 0, width)
}

func fitANSIWindow(value string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	value = expandTabs(value)
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func expandTabs(value string) string {
	return strings.ReplaceAll(value, "\t", "    ")
}

func stylePrefix(style lipgloss.Style) string {
	const marker = "\x00"
	rendered := style.Render(marker)
	index := strings.Index(rendered, marker)
	if index < 0 {
		return ""
	}
	return rendered[:index]
}

var (
	ansiSGRPattern    = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Cyan)
	fileStyle         = lipgloss.NewStyle().Bold(true)
	metadataStyle     = lipgloss.NewStyle().Faint(true)
	hunkStyle         = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	contextStyle      = lipgloss.NewStyle()
	addedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Green)
	removedStyle      = lipgloss.NewStyle().Foreground(lipgloss.Red)
	cursorStyle       = lipgloss.NewStyle().Reverse(true)
	editorCursorStyle = lipgloss.NewStyle().Reverse(true)
	mutedStyle        = lipgloss.NewStyle().Faint(true)
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Red).Bold(true)
)
