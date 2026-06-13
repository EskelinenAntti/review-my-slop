package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/anttieskelinen/review-my-slop/internal/highlight"
	"github.com/anttieskelinen/review-my-slop/internal/review"
)

type SaveCommentFunc func(review.StoredComment, review.Diff) (review.StoredComment, error)
type RefreshDiffFunc func(parent string) (review.Diff, error)

const refreshInterval = time.Second

type refreshTickMsg struct{}

type refreshDiffMsg struct {
	diff       review.Diff
	parent     string
	reschedule bool
	err        error
}

type externalEditorFinishedMsg struct {
	body string
	err  error
}

type lineEditorFinishedMsg struct {
	err error
}

type rowKind uint8

const (
	rowFile rowKind = iota
	rowMetadata
	rowHunk
	rowCode
)

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
	modeComment
	modeComments
	modeHelp
	modeSearch
)

type Model struct {
	diff       review.Diff
	rows       []row
	comments   []review.StoredComment
	commentRow int
	cursor     int
	offset     int
	xOffset    int
	width      int
	height     int
	selecting  bool
	selectFrom int
	mode       mode
	editor     []rune
	editorPos  int
	editIndex  int
	save       SaveCommentFunc
	refresh    RefreshDiffFunc
	err        error
	quitting   bool
	gPending   bool
	bracket    string
	sideBySide bool
	parents    []string
	target     int
	search     []rune
	searchTerm string
	searchFrom int
	searchMiss bool
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
	}
	model.rows = flatten(diff)
	model.cursor = firstCodeRow(model.rows)
	return model
}

func (m *Model) SetRefresh(refresh RefreshDiffFunc) {
	m.refresh = refresh
}

func (m *Model) SetParents(parents []string) {
	m.parents = append([]string(nil), parents...)
	m.target = min(m.target, len(m.parents))
}

func (m Model) Init() tea.Cmd {
	return m.nextRefresh()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.xOffset = min(m.xOffset, m.maxHorizontalOffset())
		m.ensureVisible()
		return m, nil
	case externalEditorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.editor = []rune(msg.body)
			m.editorPos = len(m.editor)
			m.err = nil
		}
		return m, nil
	case lineEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
		}
		return m, nil
	case refreshTickMsg:
		return m, m.loadRefresh(true)
	case refreshDiffMsg:
		if msg.parent != m.currentParent() {
			return m, rescheduleRefresh(m, msg.reschedule)
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", msg.err)
		} else if msg.diff.Fingerprint != m.diff.Fingerprint {
			m.applyDiff(msg.diff)
			m.err = nil
		}
		return m, rescheduleRefresh(m, msg.reschedule)
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) nextRefresh() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) loadRefresh(reschedule bool) tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	parent := m.currentParent()
	return func() tea.Msg {
		diff, err := m.refresh(parent)
		return refreshDiffMsg{diff: diff, parent: parent, reschedule: reschedule, err: err}
	}
}

func rescheduleRefresh(m Model, reschedule bool) tea.Cmd {
	if !reschedule {
		return nil
	}
	return m.nextRefresh()
}

func (m *Model) applyDiff(diff review.Diff) {
	cursor := m.rowAnchor(m.cursor)
	selection := m.rowAnchor(m.selectFrom)
	cursorFallback := m.cursor
	selectionFallback := m.selectFrom

	m.diff = diff
	m.rows = flatten(diff)
	m.cursor = m.findRow(cursor, cursorFallback)
	if m.selecting {
		m.selectFrom = m.findRow(selection, selectionFallback)
		if !m.isCode(m.selectFrom) || !m.isCode(m.cursor) ||
			m.rows[m.selectFrom].fileIndex != m.rows[m.cursor].fileIndex ||
			m.rows[m.selectFrom].hunkIndex != m.rows[m.cursor].hunkIndex {
			m.cancelSelection()
		}
	}
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
	if m.mode == modeComment {
		return m.updateEditor(name, key)
	}
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
	if m.bracket != "" {
		prefix := m.bracket
		m.bracket = ""
		switch prefix + name {
		case "]f":
			m.jump(rowFile, 1)
		case "[f":
			m.jump(rowFile, -1)
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
		m.searchMiss = false
	case "n":
		m.repeatSearch(1)
	case "N":
		m.repeatSearch(-1)
	case "j", "down":
		m.gPending = false
		m.move(1)
	case "k", "up":
		m.gPending = false
		m.move(-1)
	case "h", "left":
		m.scrollHorizontal(-1)
	case "l", "right":
		m.scrollHorizontal(1)
	case "0":
		m.xOffset = 0
	case "$":
		m.xOffset = m.maxHorizontalOffset()
	case "ctrl+d":
		m.move(max(1, m.viewportHeight()/2))
	case "ctrl+u":
		m.move(-max(1, m.viewportHeight()/2))
	case "g":
		if m.gPending {
			m.cursor = firstCodeRow(m.rows)
			m.gPending = false
			m.ensureVisible()
		} else {
			m.gPending = true
		}
	case "G":
		m.cursor = lastCodeRow(m.rows)
		m.gPending = false
		m.ensureVisible()
	case "]", "[":
		m.gPending = false
		m.bracket = name
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
		m.beginComment()
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
	case "tab":
		m.target = (m.target + 1) % (len(m.parents) + 1)
		m.cancelSelection()
		return m, m.loadRefresh(false)
	case "t":
		if m.width >= 100 {
			m.sideBySide = !m.sideBySide
			m.xOffset = min(m.xOffset, m.maxHorizontalOffset())
		} else {
			m.err = fmt.Errorf("side-by-side view requires a terminal at least 100 columns wide")
		}
	}
	return m, nil
}

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.cursor = m.searchFrom
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
		m.searchMiss = false
		m.ensureVisible()
		return
	}
	match := m.findSearch(string(m.search), m.searchFrom, 1)
	m.searchMiss = match < 0
	if match >= 0 {
		m.cursor = match
		m.ensureVisible()
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
	m.cursor = match
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
	switch name {
	case "esc", "C":
		m.mode = modeBrowse
	case "q", "ctrl+c":
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
			m.editor = []rune(m.comments[m.editIndex].Comment.Body)
			m.editorPos = len(m.editor)
			m.mode = modeComment
		}
	}
	return m, nil
}

func (m Model) updateEditor(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		if m.editIndex >= 0 {
			m.mode = modeComments
		} else {
			m.mode = modeBrowse
		}
		m.editor = nil
		m.editorPos = 0
		m.editIndex = -1
	case "enter":
		m.saveComment()
	case "shift+enter":
		m.insertEditorText("\n")
	case "ctrl+g":
		cmd, err := m.openExternalEditor()
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, cmd
	case "left":
		m.editorPos = max(0, m.editorPos-1)
	case "right":
		m.editorPos = min(len(m.editor), m.editorPos+1)
	case "up":
		m.moveEditorVertically(-1)
	case "down":
		m.moveEditorVertically(1)
	case "home":
		m.editorPos = editorLineStart(m.editor, m.editorPos)
	case "end":
		m.editorPos = editorLineEnd(m.editor, m.editorPos)
	case "backspace":
		if m.editorPos > 0 {
			m.editor = append(m.editor[:m.editorPos-1], m.editor[m.editorPos:]...)
			m.editorPos--
		}
	case "delete":
		if m.editorPos < len(m.editor) {
			m.editor = append(m.editor[:m.editorPos], m.editor[m.editorPos+1:]...)
		}
	default:
		if key.Text != "" {
			m.insertEditorText(key.Text)
		}
	}
	return m, nil
}

func (m *Model) insertEditorText(text string) {
	inserted := []rune(text)
	tail := append([]rune(nil), m.editor[m.editorPos:]...)
	m.editor = append(m.editor[:m.editorPos], inserted...)
	m.editor = append(m.editor, tail...)
	m.editorPos += len(inserted)
}

func (m *Model) moveEditorVertically(direction int) {
	start := editorLineStart(m.editor, m.editorPos)
	column := m.editorPos - start
	if direction < 0 {
		if start == 0 {
			return
		}
		previousEnd := start - 1
		previousStart := editorLineStart(m.editor, previousEnd)
		m.editorPos = min(previousStart+column, previousEnd)
		return
	}
	end := editorLineEnd(m.editor, m.editorPos)
	if end == len(m.editor) {
		return
	}
	nextStart := end + 1
	nextEnd := editorLineEnd(m.editor, nextStart)
	m.editorPos = min(nextStart+column, nextEnd)
}

func editorLineStart(text []rune, position int) int {
	position = min(max(0, position), len(text))
	for position > 0 && text[position-1] != '\n' {
		position--
	}
	return position
}

func editorLineEnd(text []rune, position int) int {
	position = min(max(0, position), len(text))
	for position < len(text) && text[position] != '\n' {
		position++
	}
	return position
}

func (m *Model) saveComment() {
	body := strings.TrimSpace(string(m.editor))
	if body == "" {
		m.err = fmt.Errorf("comment cannot be empty")
		return
	}
	if m.save == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return
	}
	var stored review.StoredComment
	if m.editIndex >= 0 {
		stored = m.comments[m.editIndex]
		stored.Comment.Body = body
	} else {
		anchor, err := m.currentAnchor()
		if err != nil {
			m.err = err
			return
		}
		stored.Comment = review.Comment{Anchor: anchor, Body: body}
	}
	saved, err := m.save(stored, m.diff)
	if err != nil {
		m.err = err
		return
	}
	if m.editIndex >= 0 {
		m.comments[m.editIndex] = saved
		m.mode = modeComments
	} else {
		m.comments = append(m.comments, saved)
		m.commentRow = len(m.comments) - 1
		m.mode = modeBrowse
	}
	m.editor = nil
	m.editorPos = 0
	m.editIndex = -1
	m.err = nil
	m.cancelSelection()
}

func (m Model) openExternalEditor() (tea.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return nil, fmt.Errorf("$EDITOR is not set")
	}
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return nil, fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if _, err := file.WriteString(string(m.editor)); err != nil {
		cleanup()
		return nil, fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, fmt.Errorf("close comment file: %w", err)
	}

	command := editorCommand(editor, path)
	return tea.ExecProcess(command, func(editorErr error) tea.Msg {
		return readExternalEditorResult(path, editorErr)
	}), nil
}

func readExternalEditorResult(path string, editorErr error) externalEditorFinishedMsg {
	defer os.Remove(path)
	if editorErr != nil {
		return externalEditorFinishedMsg{err: fmt.Errorf("editor: %w", editorErr)}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return externalEditorFinishedMsg{err: fmt.Errorf("read comment file: %w", err)}
	}
	return externalEditorFinishedMsg{body: string(body)}
}

func editorCommand(editor, path string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", editor+" "+strconv.Quote(path))
	}
	return exec.Command("sh", "-c", editor+" "+shellQuote(path))
}

func editorLineCommand(editor, path string, line int) *exec.Cmd {
	location := "+" + strconv.Itoa(line)
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", editor+" "+location+" "+strconv.Quote(path))
	}
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

	return tea.ExecProcess(editorLineCommand(editor, path, line), func(err error) tea.Msg {
		return lineEditorFinishedMsg{err: err}
	}), nil
}

func (m *Model) beginComment() {
	if !m.isCode(m.cursor) {
		return
	}
	if _, err := m.currentAnchor(); err != nil {
		m.err = err
		return
	}
	m.mode = modeComment
	m.editor = nil
	m.editorPos = 0
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
		height := m.viewportHeight()
		end := min(len(m.rows), m.offset+height)
		for index := m.offset; index < end; index++ {
			out.WriteString(m.renderRow(index))
			out.WriteByte('\n')
		}
		for index := end; index < m.offset+height; index++ {
			out.WriteByte('\n')
		}
	}

	if m.mode == modeComment {
		out.WriteString(commentBorder.Width(max(20, m.width-2)).Render(m.renderEditor()))
		out.WriteByte('\n')
	} else if m.err != nil {
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
		if m.sideBySide && m.width >= 100 {
			return m.renderSideBySide(index)
		}
		oldNumber := number(current.line.OldNumber)
		newNumber := number(current.line.NewNumber)
		prefix := " "
		style := contextStyle
		switch current.line.Kind {
		case review.LineAdded:
			prefix, style = "+", addedStyle
		case review.LineRemoved:
			prefix, style = "-", removedStyle
		}
		text := current.text
		gutter := fmt.Sprintf("%5s %5s %s ", oldNumber, newNumber, prefix)
		line := gutter + fitANSIWindow(text, m.xOffset, width-lipgloss.Width(gutter))
		if m.selected(index) {
			style = selectionStyle
		}
		stripForeground := false
		if index == m.cursor {
			style = cursorStyle
			stripForeground = true
		}
		return renderStyledRow(style, line, width, stripForeground)
	default:
		return ""
	}
}

func (m Model) renderEditor() string {
	label := "New comment"
	if m.editIndex >= 0 {
		label = "Edit comment"
	}
	return label + "\n" + m.renderEditorBody() + "\n" +
		mutedStyle.Render("Enter save  Shift-Enter newline  arrows move  Ctrl-G $EDITOR  Esc cancel")
}

func (m Model) renderEditorBody() string {
	if len(m.editor) == 0 {
		return editorCursorStyle.Render(" ") + mutedStyle.Render("Type feedback...")
	}
	position := min(max(0, m.editorPos), len(m.editor))
	if position == len(m.editor) {
		return string(m.editor) + editorCursorStyle.Render(" ")
	}
	return string(m.editor[:position]) +
		editorCursorStyle.Render(string(m.editor[position])) +
		string(m.editor[position+1:])
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
	out.WriteString(mutedStyle.Render("j/k move  Enter/e edit  Esc return  q quit"))
	out.WriteByte('\n')
	return out.String()
}

func (m Model) renderHelp() string {
	bindings := []keyBinding{
		{"j/k, arrows", "move"},
		{"h/l, left/right", "scroll horizontally"},
		{"0/$", "start/end of lines"},
		{"gg/G", "first/last changed line"},
		{"Ctrl-d/Ctrl-u", "half-page down/up"},
		{"/", "search diff text"},
		{"n/N", "next/previous search match"},
		{"]f/[f", "next/previous file"},
		{"v", "select a line range"},
		{"c", "comment on selection/current line"},
		{"e", "open current line in $EDITOR"},
		{"C", "view/edit comments"},
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

func (m Model) renderSideBySide(index int) string {
	current := m.rows[index]
	half := max(20, (m.width-3)/2)
	leftNumber, rightNumber := number(current.line.OldNumber), number(current.line.NewNumber)
	leftText, rightText := "", ""
	switch current.line.Kind {
	case review.LineRemoved:
		leftText = "- " + current.text
	case review.LineAdded:
		rightText = "+ " + current.text
	default:
		leftText = "  " + current.text
		rightText = "  " + current.text
	}
	leftGutter := fmt.Sprintf("%5s ", leftNumber)
	rightGutter := fmt.Sprintf("%5s ", rightNumber)
	left := leftGutter + fitANSIWindow(leftText, m.xOffset, half-lipgloss.Width(leftGutter))
	right := rightGutter + fitANSIWindow(rightText, m.xOffset, half-lipgloss.Width(rightGutter))
	style := contextStyle
	switch current.line.Kind {
	case review.LineRemoved:
		style = removedStyle
	case review.LineAdded:
		style = addedStyle
	}
	if m.selected(index) {
		style = selectionStyle
	}
	stripForeground := false
	if index == m.cursor {
		style = cursorStyle
		stripForeground = true
	}
	return renderStyledRow(style, left+" │ "+right, m.width, stripForeground)
}

func flatten(diff review.Diff) []row {
	var rows []row
	for fileIndex, file := range diff.Files {
		rows = append(rows, row{kind: rowFile, fileIndex: fileIndex, hunkIndex: -1, lineIndex: -1, text: file.Display})
		for _, metadata := range file.Metadata {
			rows = append(rows, row{kind: rowMetadata, fileIndex: fileIndex, hunkIndex: -1, lineIndex: -1, text: metadata})
		}
		highlighted := highlight.Sources(file.Language, file.OldSource, file.NewSource)
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

func highlightedLine(lines []string, number int, fallback string) string {
	if number <= 0 || number > len(lines) {
		return fallback
	}
	return lines[number-1]
}

func (m *Model) move(delta int) {
	if len(m.rows) == 0 {
		return
	}
	target := m.cursor
	step := 1
	if delta < 0 {
		step = -1
	}
	for moved := 0; moved < abs(delta); {
		next := target + step
		for next >= 0 && next < len(m.rows) && m.rows[next].kind != rowCode {
			next += step
		}
		if next < 0 || next >= len(m.rows) {
			break
		}
		if m.selecting && !m.sameHunk(m.selectFrom, next) {
			break
		}
		target = next
		moved++
	}
	m.cursor = target
	m.ensureVisible()
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
	height := m.viewportHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+height {
		m.offset = m.cursor - height + 1
	}
	m.offset = max(0, min(m.offset, max(0, len(m.rows)-height)))
}

func (m *Model) scrollHorizontal(delta int) {
	m.xOffset = max(0, min(m.xOffset+delta, m.maxHorizontalOffset()))
}

func (m Model) maxHorizontalOffset() int {
	contentWidth := max(1, m.width-14)
	extraWidth := 0
	if m.sideBySide && m.width >= 100 {
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
	reserved := 3
	if m.mode == modeComment {
		reserved = 8
	}
	return max(1, m.height-reserved)
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

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

var (
	ansiSGRPattern    = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#83a598"))
	fileStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ebdbb2")).Background(lipgloss.Color("#3c3836"))
	metadataStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	hunkStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#d3869b"))
	contextStyle      = lipgloss.NewStyle()
	addedStyle        = lipgloss.NewStyle().Background(lipgloss.Color("#34432f"))
	removedStyle      = lipgloss.NewStyle().Background(lipgloss.Color("#4b302e"))
	selectionStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#83a598")).Bold(true)
	cursorStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#282828")).Background(lipgloss.Color("#fabd2f"))
	editorCursorStyle = lipgloss.NewStyle().Reverse(true)
	mutedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#928374"))
	errorStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#fb4934")).Bold(true)
	commentBorder     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#83a598")).Padding(0, 1)
)
