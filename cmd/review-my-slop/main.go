// Command review-my-slop provides interactive patch review.
package main

import (
	"bytes"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/alecthomas/chroma/v2/quick"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/sourcegraph/go-diff/diff"
	bolt "go.etcd.io/bbolt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type diffscreenCursorIdentity struct {
	file   patchFile
	hunk   patchHunk
	line   patchLine
	cursor diffscreenDiffCursor
	valid  bool
}

func (m appModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	result := tea.NewView(func() string {
		switch m.mode {
		case appModeHelp:
			return renderAppHelp(m.width, m.height)
		case appModeComments:
			return m.commentView.Render(m.err)
		default:
			return m.diffView.Render(m.err)
		}
	}())
	result.AltScreen = true
	result.ReportFocus = true
	result.MouseMode = tea.MouseModeCellMotion
	return result
}

func (s commentsStore) WritePending(w io.Writer, repository string) error {
	pending, err := s.List(repository)
	if err != nil {
		return err
	}
	if err := writePendingComments(w, pending); err != nil {
		return err
	}
	return s.Acknowledge(repository, commentIDs(pending))
}

const (
	commentsMaxCommentBytes = 64 << 10
	commentsMaxPendingBytes = 16 << 20
)

var commentsMessagesBucket = []byte("messages")

type commentsStore struct {
	Path string
}

func (s commentsStore) Add(comment commentsComment) (commentsComment, error) {
	if comment.Repository == "" {
		return commentsComment{}, errors.New("comment requires a repository")
	}
	if err := validateCommentBody(comment.Body); err != nil {
		return commentsComment{}, err
	}
	if comment.ID == "" {
		comment.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	if comment.CreatedAt.IsZero() {
		comment.CreatedAt = time.Now().UTC()
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return commentsComment{}, fmt.Errorf("encode comment: %w", err)
	}
	db, err := s.open()
	if err != nil {
		return commentsComment{}, err
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		return addStoredComment(tx, comment, data)
	})
	return comment, err
}

func (s commentsStore) List(repository string) ([]commentsComment, error) {
	var comments []commentsComment
	db, err := s.open()
	if errors.Is(err, os.ErrNotExist) {
		return comments, nil
	}
	if err != nil {
		return nil, err
	}
	defer db.Close()
	err = db.View(func(tx *bolt.Tx) error {
		var err error
		comments, err = repositoryComments(tx, repository)
		return err
	})
	return comments, err
}

func (s commentsStore) Update(comment commentsComment) error {
	if comment.Repository == "" || comment.ID == "" {
		return errors.New("repository and comment ID are required")
	}
	if err := validateCommentBody(comment.Body); err != nil {
		return err
	}
	data, err := json.Marshal(comment)
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		return updateStoredComment(tx, comment, data)
	})
}

func (s commentsStore) Delete(repository, id string) error {
	if repository == "" || id == "" {
		return errors.New("repository and comment ID are required")
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		return deleteStoredComment(tx, repository, id)
	})
}

func (s commentsStore) Acknowledge(repository string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		return acknowledgeStoredComments(tx, repository, wanted)
	})
}

type commentsLegacyComment struct {
	Anchor commentsAnchor `json:"anchor"`
	Body   string         `json:"body"`
}

type commentsLegacyRecord struct {
	ID         string                 `json:"id"`
	Repository string                 `json:"repository"`
	CreatedAt  time.Time              `json:"created_at"`
	Comment    *commentsLegacyComment `json:"comment"`
}

type settingsPreferences struct {
	SideBySide bool `json:"side_by_side"`
}

var appNavigationMotions = map[string]diffscreenMotion{
	"j":             diffscreenNextLine,
	"down":          diffscreenNextLine,
	"k":             diffscreenPreviousLine,
	"up":            diffscreenPreviousLine,
	"ctrl+d":        diffscreenNextPage,
	"ctrl+u":        diffscreenPreviousPage,
	"g g":           diffscreenFirstLine,
	"G":             diffscreenLastLine,
	"] f":           diffscreenNextFile,
	"[ f":           diffscreenPreviousFile,
	"ctrl+w h":      diffscreenOldPane,
	"ctrl+w l":      diffscreenNewPane,
	"ctrl+w ctrl+w": diffscreenOtherPane,
}

var appNavigationAlignments = map[string]diffscreenAlignment{
	"z z": diffscreenCenter,
	"z t": diffscreenTop,
	"z b": diffscreenBottom,
}

type commentscreenView struct {
	items         []commentsComment
	row           int
	top           int
	width, height int
}

func (v *commentscreenView) Update(items []commentsComment) {
	selected, ok := v.Selected()
	v.items = append([]commentsComment(nil), items...)
	v.row = min(v.row, max(0, len(items)-1))
	if ok && selected.ID != "" {
		for index, item := range items {
			if item.ID == selected.ID {
				v.row = index
				break
			}
		}
	}
	{
		height := (func(height int) int { return max(1, height-3) })(v.height)
		v.top = min(v.top, v.row)
		v.top = max(v.top, v.row-height+1)
		v.top = min(max(0, v.top), max(0, len(v.items)-height))
	}
}

func (v *commentscreenView) Resize(width, height int) {
	v.width, v.height = width, height
	{
		height := (func(height int) int { return max(1, height-3) })(v.height)
		v.top = min(v.top, v.row)
		v.top = max(v.top, v.row-height+1)
		v.top = min(max(0, v.top), max(0, len(v.items)-height))
	}
}

func (v *commentscreenView) Move(delta int) {
	delta = max(-v.row, min(delta, max(0, len(v.items)-1)-v.row))
	v.row += delta
	{
		height := (func(height int) int { return max(1, height-3) })(v.height)
		v.top = min(v.top, v.row)
		v.top = max(v.top, v.row-height+1)
		v.top = min(max(0, v.top), max(0, len(v.items)-height))
	}
}

func (v *commentscreenView) Selected() (commentsComment, bool) {
	if len(v.items) == 0 {
		return commentsComment{}, false
	}
	return v.items[v.row], true
}

func (v *commentscreenView) Click(x, y int) bool {
	height := (func(height int) int { return max(1, height-3) })(v.height)
	row, ok := (func(x, y, width, height int) (int, bool) {
		row := y - 1
		return row, x >= 0 && x < width && row >= 0 &&
			row < (func(height int) int {
				return max(1, height-3)
			})(height) && y < height-2
	})(x, y, v.width, v.height)
	if !ok {
		return false
	}
	row += min(max(0, v.top), max(0, len(v.items)-height))
	if row >= len(v.items) {
		return false
	}
	v.row = row
	return true
}

func (v *commentscreenView) Render(err error) string {
	pending := commentscreenMutedStyle.Render(fmt.Sprintf("%d pending", len(
		v.items,
	)))
	header := commentscreenTitleStyle.Render("comments") + "  " + pending
	body := v.renderRows()
	footer := v.renderFooter(err)
	return renderCommentScreenFrame(header, body, footer, v.height)
}

var (
	commentscreenTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Cyan)
	commentscreenMutedStyle = lipgloss.NewStyle().Faint(true)
	commentscreenErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Red).
				Bold(true)
)

type helpscreenBinding struct {
	Keys        string
	Description string
}

const patchMaxFileBytes = 2 << 20

type patchRunner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type patchExecRunner struct{}

func (patchExecRunner) Run(
	ctx context.Context,
	dir string,
	args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
	}
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf(
			"git %s: %s",
			strings.Join(args, " "),
			strings.TrimSpace(string(exitErr.Stderr)),
		)
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

type patchSourceReader func(context.Context, patchRunner, string, string) string

func (v *diffscreenView) BeginDrag(x, y int) bool {
	v.EndDrag()
	cursor, ok := diffscreenPointerCursor(
		v.view,
		v.viewport,
		x,
		y,
		v.width,
		v.height,
	)
	if !ok {
		return false
	}
	v.ClearSelection()
	v.cursor = cursor
	v.drag = &cursor
	return true
}

func (v *diffscreenView) DragTo(x, y int) {
	if v.drag == nil {
		return
	}
	cursor, ok := diffscreenPointerCursor(
		v.view,
		v.viewport,
		x,
		y,
		v.width,
		v.height,
	)
	if !ok || cursor.pane != v.drag.pane {
		return
	}
	if cursor == *v.drag && v.selection == nil {
		return
	}
	selection, ok := diffscreenSelectionForDrag(v.view, *v.drag, cursor)
	if !ok {
		return
	}
	v.cursor, v.selection = cursor, &selection
}

func (v *diffscreenView) EndDrag() { v.drag = nil }

const commentsAppName = "review-my-slop"

type keymapSequence struct {
	Prefix         string
	Keys           []string
	RetryUnmatched bool
}

type keymapMatcher struct {
	sequences map[string]keymapSequence
	pending   string
}

func (m *keymapMatcher) Feed(key string) string {
	if m.pending != "" {
		prefix := m.pending
		m.pending = ""
		sequence := m.sequences[prefix]
		if slices.Contains(sequence.Keys, key) {
			return prefix + " " + key
		}
		if !sequence.RetryUnmatched {
			return ""
		}
	}
	if _, ok := m.sequences[key]; ok {
		m.pending = key
		return ""
	}
	return key
}

type appSaveCommentFunc func(commentsComment, patchPatch,

) (commentsComment, error)

type appDeleteCommentFunc func(commentsComment, patchPatch) error

type appLoadCommentsFunc func() ([]commentsComment, error)

type appRefreshDiffFunc func(patchKind) (patchPatch, error)

type appSaveSideBySideFunc func(bool) error

type appCommentStore interface {
	Add(commentsComment) (commentsComment, error)
	List(string) ([]commentsComment, error)
	Update(commentsComment) error
	Delete(string, string) error
}

type appSize struct {
	Width  int
	Height int
}

type appInitialLayout struct {
	SideBySide     bool
	SaveSideBySide appSaveSideBySideFunc
	size           appSize
}

type appRefreshDiffMsg struct {
	patch patchPatch
	kind  patchKind
	err   error
}

type appCommentEditorFinishedMsg struct {
	body string
	err  error
}

type appCommentsLoadedMsg struct {
	comments []commentsComment
	revision uint64
	err      error
}

type appSourceEditorFinishedMsg struct{ err error }

type appMode uint8

const (
	appModeBrowse appMode = iota
	appModeComments
	appModeHelp
	appModeSearch
)

const appHorizontalScrollStep = 4

const appVerticalScrollStep = 3

var appDefaultSize = appSize{Width: 80, Height: 30}

type appCommentState struct {
	items    []commentsComment
	revision uint64
}

type appCommentEdit struct {
	body   string
	index  int
	anchor commentsAnchor
}

type appModel struct {
	ctx context.Context

	diffView    *diffscreenView
	commentView *commentscreenView
	width       int
	height      int
	mode        appMode
	keys        keymapMatcher

	currentPatch patchPatch
	kind         patchKind
	diffOptions  diffscreenOptions
	comments     appCommentState
	edit         appCommentEdit

	save       appSaveCommentFunc
	delete     appDeleteCommentFunc
	load       appLoadCommentsFunc
	refresh    appRefreshDiffFunc
	saveLayout appSaveSideBySideFunc
	err        error
	quitting   bool
}

func (appModel) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.diffOptions.Dark {
			m.diffOptions.Dark = dark
			m.diffView.Configure(m.diffOptions)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.diffView.Resize(m.width, m.height)
		m.commentView.Resize(m.width, m.height)

	case tea.KeyPressMsg:
		return m.updateKeyPress(msg)
	case tea.MouseClickMsg, tea.MouseMotionMsg,
		tea.MouseReleaseMsg, tea.MouseWheelMsg:
		m.handleMouseInput(msg)
		return m, nil
	default:
		return m.updateOtherMessage(msg)
	}
	return m, nil
}

type diffscreenOptions struct {
	SideBySide bool
	Dark       bool
}

type diffscreenMotion uint8

const (
	diffscreenPreviousLine diffscreenMotion = iota
	diffscreenNextLine
	diffscreenPreviousPage
	diffscreenNextPage
	diffscreenFirstLine
	diffscreenLastLine
	diffscreenPreviousFile
	diffscreenNextFile
	diffscreenOldPane
	diffscreenNewPane
	diffscreenOtherPane
)

type diffscreenAlignment uint8

const (
	diffscreenTop diffscreenAlignment = iota
	diffscreenCenter
	diffscreenBottom
)

type diffscreenDirection int8

const (
	diffscreenBackward diffscreenDirection = -1
	diffscreenForward  diffscreenDirection = 1
)

type diffscreenView struct {
	view          *diffscreenDiffView
	cursor        diffscreenDiffCursor
	viewport      diffscreenDiffViewport
	selection     *diffscreenDiffSelection
	drag          *diffscreenDiffCursor
	options       diffscreenOptions
	width, height int
	search        diffscreenSearchState
}

type diffscreenSearchState struct {
	active           bool
	query, term      string
	from             diffscreenDiffCursor
	miss, repeatMiss bool
}

func (v *diffscreenView) Update(p patchPatch) {
	v.EndDrag()
	v.search.repeatMiss = false
	next := diffscreenBuildDiffView(p, diffscreenOptions{
		SideBySide: v.options.SideBySide && v.width >= 100,
		Dark:       v.options.Dark,
	})
	old := v.view
	leftColumn := v.viewport.LeftColumn
	rowsAbove := v.cursor.row - v.viewport.top
	cursor := diffscreenCursorIdentityAt(old, v.cursor)
	origin := diffscreenCursorIdentityAt(old, v.search.from)
	v.viewport = diffscreenInitialViewport(
		next,
		v.viewport.Width,
		v.viewport.Height,
	)
	v.viewport = diffscreenPreservedViewport(next, v.viewport, leftColumn)
	translated, ok := diffscreenFindCursorMatch(next, cursor)
	if !ok {
		translated, _ = diffscreenFirstCursor(next)
	}
	v.cursor = translated
	v.selection = diffscreenTranslatedSelection(next, old, v)
	v.view = next
	if diffscreenCursorLine(next, v.cursor) >= 0 {
		v.viewport.top = max(0, v.cursor.row-rowsAbove)
		v.viewport = diffscreenCursorViewport(next, v.viewport, v.cursor)
	}

	diffscreenPreserveSearch(v, next, origin)
}

func (v *diffscreenView) Configure(options diffscreenOptions) {
	active := v.options.SideBySide && v.width >= 100
	dark := v.options.Dark
	v.options = options
	if active != (v.options.SideBySide && v.width >= 100) ||
		dark != options.Dark {
		v.Update(v.view.patch)
	}
}

func (v *diffscreenView) Resize(width, height int) {
	v.EndDrag()
	wasActive := v.options.SideBySide && v.width >= 100
	v.width, v.height = width, height
	v.viewport.Width = max(1, width)
	v.viewport.Height = max(1, height-3)
	v.viewport = diffscreenClampViewport(v.view, v.viewport)
	if wasActive != (v.options.SideBySide && v.width >= 100) {
		v.Update(v.view.patch)
		return
	}
	v.viewport = diffscreenCursorViewport(v.view, v.viewport, v.cursor)
}

func (v *diffscreenView) Move(motion diffscreenMotion) {
	v.EndDrag()
	v.search.repeatMiss = false
	switch motion {
	case diffscreenPreviousLine:
		diffscreenMoveLine(v, diffscreenPrivateBackward)
	case diffscreenNextLine:
		diffscreenMoveLine(v, diffscreenPrivateForward)
	case diffscreenPreviousPage:
		diffscreenMovePage(v, diffscreenPrivateBackward)
	case diffscreenNextPage:
		diffscreenMovePage(v, diffscreenPrivateForward)
	case diffscreenFirstLine:
		if cursor, ok := diffscreenFirstCursor(v.view); ok {
			v.cursor = cursor
			v.viewport = diffscreenCursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
		}
	case diffscreenLastLine:
		if cursor, ok := diffscreenLastCursor(v.view); ok {
			v.cursor = cursor
			v.viewport = diffscreenCursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
		}
	default:
		diffscreenMoveFileOrPane(v, motion)
	}
}

func (v *diffscreenView) Align(alignment diffscreenAlignment) {
	v.search.repeatMiss = false
	if diffscreenCursorLine(v.view, v.cursor) < 0 {
		return
	}
	viewport := v.viewport
	headerHeight := 0
	if viewport.Height > 1 {
		headerHeight = 1
	}
	requestedOffset := diffscreenAlignmentOffset(alignment, viewport.Height)
	viewport.top = v.cursor.row - max(0, requestedOffset-headerHeight)
	if !diffscreenViewportHasFileHeader(v.view, viewport) {
		viewport.top = v.cursor.row - requestedOffset
	}
	v.viewport = diffscreenClampViewport(v.view, viewport)
}

func (v *diffscreenView) ScrollHorizontal(columns int) {
	v.search.repeatMiss = false
	maxColumn := diffscreenViewportLeft(v.view, v.viewport)
	columns = max(-v.viewport.LeftColumn, min(
		columns,
		maxColumn-v.viewport.LeftColumn,
	))
	v.viewport.LeftColumn += columns
	v.viewport = diffscreenClampViewport(v.view, v.viewport)
}

func (v *diffscreenView) ScrollVertical(rows int) {
	v.search.repeatMiss = false
	initialViewport := v.viewport
	maxTop := diffscreenViewportBottom(v.view, v.viewport.Height)
	rows = max(-v.viewport.top, min(rows, maxTop-v.viewport.top))
	v.viewport.top += rows
	v.viewport = diffscreenClampViewport(v.view, v.viewport)
	if diffscreenCursorVisible(v.view, v.viewport, v.cursor) {
		return
	}
	height := diffscreenViewportHeight(v.view, v.viewport)
	target := max(v.viewport.top, min(v.cursor.row, v.viewport.top+height-1))
	direction := diffscreenScrollDirection(v.cursor.row, target)
	candidate, ok := diffscreenNearestScrollCursor(
		v.view, v.viewport, v.cursor, target, direction,
	)
	if !ok {
		return
	}
	selection, valid := diffscreenScrollSelection(
		v.view,
		v.selection,
		candidate,
	)
	if !valid {
		v.viewport = initialViewport
		return
	}
	v.selection = selection
	v.cursor = candidate
}

func (v *diffscreenView) ToggleSelection() {
	v.search.repeatMiss = false
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	if _, _, ok := diffscreenPatchLineAtCursor(v.view, v.cursor); !ok {
		return
	}
	selection := diffscreenDiffSelection{First: v.cursor, Last: v.cursor}
	v.selection = &selection
}

func (v *diffscreenView) ClearSelection() {
	v.EndDrag()
	v.selection = nil
	v.search.repeatMiss = false
}

func (v *diffscreenView) Selected() (patchFile, []patchLine, bool) {
	selection := diffscreenDiffSelection{First: v.cursor, Last: v.cursor}
	if v.selection != nil {
		selection = *v.selection
	}
	file, valid := diffscreenSelectionFile(v.view, selection)
	if !valid {
		return file, nil, false
	}
	lines := diffscreenLinesInSelection(v.view, selection)
	return file, lines, len(lines) > 0
}

func (v *diffscreenView) Current() (patchFile, patchLine, bool) {
	return diffscreenPatchLineAtCursor(v.view, v.cursor)
}

func (v *diffscreenView) BeginSearch() {
	v.ClearSelection()
	v.search.active = true
	v.search.query, v.search.miss, v.search.repeatMiss = "", false, false
	v.search.from = v.cursor
}

func (v *diffscreenView) InsertSearch(text string) {
	if !v.search.active {
		v.BeginSearch()
	}
	func() {
		var query = v.search.query + text
		v.search.query = query
		if query == "" {
			var cursor = v.search.from
			v.cursor = cursor
			v.viewport = diffscreenCursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
			v.search.miss = false
			return
		}
		cursor, ok := diffscreenFindSearchCursor(v.view, query, v.search.from)
		v.search.miss = !ok
		if ok {
			v.cursor = cursor
			v.viewport = diffscreenCursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
		}
	}()
}

func (v *diffscreenView) BackspaceSearch() {
	if !v.search.active {
		return
	}
	query := []rune(v.search.query)
	if len(query) > 0 {
		query = query[:len(query)-1]
	}
	func() {
		v.search.query = string(query)
		if string(query) == "" {
			var cursor = v.search.from
			v.cursor = cursor
			v.viewport = diffscreenCursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
			v.search.miss = false
			return
		}
		cursor, ok := diffscreenFindSearchCursor(
			v.view,
			string(query),
			v.search.from,
		)
		v.search.miss = !ok
		if ok {
			v.cursor = cursor
			v.viewport = diffscreenCursorVisibleViewport(
				cursor,
				v.view,
				v.viewport,
			)
		}
	}()
}

func (v *diffscreenView) AcceptSearch() {
	if !v.search.active {
		return
	}
	if v.search.query != "" && !v.search.miss {
		v.search.term = v.search.query
	}
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

func (v *diffscreenView) CancelSearch() {
	if !v.search.active {
		return
	}
	v.cursor = v.search.from
	v.viewport = diffscreenCursorViewport(v.view, v.viewport, v.cursor)
	v.search.active, v.search.miss = false, false
	v.search.query = ""
}

func (v *diffscreenView) Find(d diffscreenDirection) bool {
	if v.search.term == "" {
		return false
	}
	if d != diffscreenForward && d != diffscreenBackward {
		return false
	}
	cursor, ok := diffscreenFindSearchCursorDirection(
		v.view, v.search.term, v.cursor, diffscreenPrivateDirection(d),
	)
	v.search.repeatMiss = !ok
	if ok {
		v.cursor = cursor
		v.viewport = diffscreenCursorVisibleViewport(cursor, v.view, v.viewport)
	}
	return ok
}

func (v *diffscreenView) Render(err error) string {
	added, removed := diffscreenPatchChangeCounts(v.view.patch)
	counts := diffscreenMutedStyle.Render(fmt.Sprintf("+%d-%d", added, removed))
	header := diffscreenTitleStyle.Render("review-my-slop") + "  " + counts
	body := diffscreenRenderBody(v)
	footer := diffscreenRenderFooter(v, err)
	return diffscreenRenderFrame(header, body, footer, v.height)
}

var (
	diffscreenTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Cyan)
	diffscreenMutedStyle = lipgloss.NewStyle().Faint(true)
	diffscreenErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Red).
				Bold(true)
)

type patchKind uint8

const (
	patchUnstaged patchKind = iota

	patchBranch
)

type patchPatch struct {
	Root   string
	Kind   patchKind
	Branch string
	Files  []patchFile
}

type patchFile struct {
	OldPath     string
	NewPath     string
	DisplayPath string
	OldSource   string
	NewSource   string
	Metadata    []string
	Hunks       []patchHunk
}

type patchHunk struct {
	Header string
	Lines  []patchLine
}

type diffscreenForegroundPolicy uint8

const (
	diffscreenPreserveForeground diffscreenForegroundPolicy = iota
	diffscreenDiscardForeground
)

var (
	diffscreenAnsiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	diffscreenFileStyle      = lipgloss.NewStyle().Bold(true)
	diffscreenMetadataStyle  = lipgloss.NewStyle().Faint(true)
	diffscreenHunkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Magenta)
	diffscreenContextStyle   = lipgloss.NewStyle()
	diffscreenAddedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Green)
	diffscreenRemovedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Red)
	diffscreenCursorStyle    = lipgloss.NewStyle().Reverse(true)
)

type diffscreenCursorMatch struct {
	cursor         diffscreenDiffCursor
	rank, distance int
}

type diffscreenDiffPane uint8

const (
	diffscreenLeft diffscreenDiffPane = iota
	diffscreenRight
)

type diffscreenDiffCursor struct {
	row  int
	pane diffscreenDiffPane
}

type diffscreenDiffViewport struct {
	top        int
	LeftColumn int
	Width      int
	Height     int
}

type diffscreenDiffSelection struct {
	First diffscreenDiffCursor
	Last  diffscreenDiffCursor
}

type diffscreenPrivateDirection int8

const (
	diffscreenPrivateBackward diffscreenPrivateDirection = -1
	diffscreenPrivateForward  diffscreenPrivateDirection = 1
)

type diffscreenVerticalAlignment uint8

const (
	diffscreenPrivateTop diffscreenVerticalAlignment = iota
	diffscreenMiddle
	diffscreenPrivateBottom
)

type commentsAnchor struct {
	FilePath    string   `json:"file"`
	OldStart    int      `json:"old_start,omitempty"`
	OldEnd      int      `json:"old_end,omitempty"`
	NewStart    int      `json:"new_start,omitempty"`
	NewEnd      int      `json:"new_end,omitempty"`
	QuotedLines []string `json:"quoted_lines"`
}

type commentsComment struct {
	ID         string         `json:"id"`
	Repository string         `json:"repository"`
	CreatedAt  time.Time      `json:"created_at"`
	Anchor     commentsAnchor `json:"anchor"`
	Body       string         `json:"body"`
}

const mainErrorExitCode = 1

func main() {
	err := runCommand(context.Background(), os.Args[1:], os.Stdout)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(mainErrorExitCode)
	}
}

type diffscreenRowKind uint8

const (
	diffscreenFileRow diffscreenRowKind = iota
	diffscreenMetadataRow
	diffscreenHunkRow
	diffscreenLineRow
)

type diffscreenEntry struct {
	kind              diffscreenRowKind
	file, hunk        int
	leftLine          int
	rightLine         int
	text, left, right string
}

type diffscreenDiffView struct {
	patch patchPatch
	rows  []diffscreenEntry
	split bool
	dark  bool
}

type patchLine struct {
	Kind      patchLineKind
	Text      string
	OldNumber patchLineNumber
	NewNumber patchLineNumber
}

type patchLineNumber int

type patchLineKind uint8

const (
	patchContext patchLineKind = iota
	patchAddition
	patchDeletion
)

type editorCommandAdapter struct {
	run            func() error
	body           string
	stdin          io.Reader
	stdout, stderr io.Writer
}

func (c *editorCommandAdapter) Run() error { return c.run() }

func (c *editorCommandAdapter) SetStdin(r io.Reader) { c.stdin = r }

func (c *editorCommandAdapter) SetStdout(w io.Writer) { c.stdout = w }

func (c *editorCommandAdapter) SetStderr(w io.Writer) { c.stderr = w }

type diffscreenPair struct {
	Old []string
	New []string
}

const (
	diffscreenLightTheme = "catppuccin-latte"
	diffscreenDarkTheme  = "catppuccin-mocha"
)

func renderAppHelp(width, height int) string {
	return renderHelpBindings(appHelpBindings(), width, height)
}

func appHelpBindings() []helpscreenBinding {
	return []helpscreenBinding{
		{Keys: "j/k, arrows", Description: "move"},
		{Keys: "h/l, left/right", Description: "scroll horizontally"},
		{Keys: "Mouse wheel", Description: "scroll content"},
		{Keys: "Click/drag", Description: "focus line/select range"},
		{Keys: "Ctrl-w h/l/w", Description: "switch side-by-side pane"},
		{Keys: "0/$", Description: "start/end of lines"},
		{Keys: "gg/G", Description: "first/last changed line"},
		{Keys: "zz/zt/zb", Description: "center/top/bottom current line"},
		{Keys: "Ctrl-d/Ctrl-u", Description: "half-page down/up"},
		{Keys: "/", Description: "search diff text"},
		{Keys: "n/N", Description: "next/previous search match"},
		{Keys: "]f/[f", Description: "next/previous file"},
		{Keys: "v", Description: "select a line range"},
		{Keys: "c", Description: "comment on selection/current line"},
		{Keys: "e", Description: "open current line in $EDITOR"},
		{Keys: "C", Description: "view comments"},
		{Keys: "R", Description: "refresh diff"},
		{Keys: "Tab", Description: "toggle local/branch changes"},
		{Keys: "t", Description: "toggle unified/side-by-side"},
		{Keys: "q", Description: "quit"},
	}
}

func renderHelpBindings(
	bindings []helpscreenBinding,
	width, height int,
) string {
	column := 0
	for _, binding := range bindings {
		column = max(column, lipgloss.Width(binding.Keys))
	}
	body := []string{""}
	for _, binding := range bindings {
		line := binding.Keys + strings.Repeat(" ", column-lipgloss.Width(
			binding.Keys,
		)) +
			"  " + binding.Description
		body = append(body, ansi.Truncate(line, max(20, width), ""))
	}
	return renderHelpContent(body, height)
}

func renderHelpContent(body []string, height int) string {
	bodyHeight := max(1, height-3)
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, lipgloss.NewStyle().Bold(
		true,
	).Foreground(lipgloss.Cyan).Render(
		"review-my-slop help",
	))
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+1 {
		lines = append(lines, "")
	}
	lines = append(lines, lipgloss.NewStyle().Faint(
		true,
	).Render("? or Esc closes help"), "")
	return strings.Join(lines, "\n")
}

func (v *commentscreenView) renderRows() []string {
	height := max(1, v.height-3)
	if len(v.items) == 0 {
		body := make([]string, height)
		row := min(1, height-1)
		body[row] = commentscreenMutedStyle.Render("No pending comments.")
		return body
	}
	start := min(max(0, v.top), max(0, len(v.items)-height))
	end := min(len(v.items), start+height)
	body := make([]string, 0, end-start)
	for index := start; index < end; index++ {
		body = append(body, v.renderRow(index))
	}
	return body
}

func (v *commentscreenView) renderRow(index int) string {
	comment := v.items[index]
	prefix, style := "  ", lipgloss.NewStyle()
	if index == v.row {
		prefix, style = "> ", lipgloss.NewStyle().Reverse(true)
	}
	location := comment.Anchor.FilePath
	if comment.Anchor.NewStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.NewStart)
	} else if comment.Anchor.OldStart > 0 {
		location += fmt.Sprintf(":%d", comment.Anchor.OldStart)
	}
	text := strings.ReplaceAll(strings.TrimSpace(comment.Body), "\n", " ")
	line := ansi.Truncate(
		fmt.Sprintf("%s%s  %s", prefix, location, text), max(20, v.width), "",
	)
	return style.Width(max(20, v.width)).Render(line)
}

func (v *commentscreenView) renderFooter(err error) string {
	if err == nil {
		return commentscreenMutedStyle.Render(
			"j/k move  Enter/e edit  D delete  Esc/q return",
		)
	}
	message := strings.ReplaceAll(err.Error(), "\n", " ")
	return commentscreenErrorStyle.Render(ansi.Truncate(message, max(
		20,
		v.width,
	), ""))
}

func renderCommentScreenFrame(
	header string,
	body []string,
	footer string,
	height int,
) string {
	bodyHeight := max(1, height-3)
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, header)
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+1 {
		lines = append(lines, "")
	}
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

func (m appModel) updateOtherMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case appCommentEditorFinishedMsg:
		m.finishCommentEditor(msg)
	case appCommentsLoadedMsg:
		m.applyLoadedComments(msg)
	case appSourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			return m, nil
		}
		return m, m.refreshDiffCommand()
	case tea.FocusMsg:
		return m, m.refreshDiffCommand()
	case appRefreshDiffMsg:
		m.applyRefreshedPatch(msg)
	default:
		return m, nil
	}
	return m, nil
}

func (m *appModel) finishCommentEditor(msg appCommentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.edit = appCommentEdit{index: -1}
		return
	}
	m.edit.body = msg.body
	body := strings.TrimSpace(m.edit.body)
	if body == "" {
		if m.edit.index >= 0 {
			m.deleteEditedComment()
		}
		m.edit = appCommentEdit{index: -1}
		m.diffView.ClearSelection()
		return
	}
	m.saveEditedComment(body)
}

func (m *appModel) deleteEditedComment() {
	index := m.edit.index
	if index < 0 || index >= len(m.comments.items) {
		return
	}
	if m.delete == nil {
		m.err = errors.New("comment storage is unavailable")
		return
	}
	if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
		m.err = err
		return
	}
	m.comments.items = append(
		m.comments.items[:index],
		m.comments.items[index+1:]...,
	)
	m.commentView.Update(m.comments.items)
	m.comments.revision++
	m.err = nil
}

func (m *appModel) saveEditedComment(body string) {
	if m.save == nil {
		m.err = errors.New("comment storage is unavailable")
		m.edit = appCommentEdit{index: -1}
		return
	}
	comment := m.editedComment(body)
	saved, err := m.save(comment, m.currentPatch)
	if err != nil {
		m.err = err
		m.edit = appCommentEdit{index: -1}
		return
	}
	if m.edit.index >= 0 {
		m.comments.items[m.edit.index] = saved
	} else {
		m.comments.items = append(m.comments.items, saved)
		m.commentView.Update(m.comments.items)
		m.commentView.Move(len(m.comments.items))
	}
	m.comments.revision++
	m.commentView.Update(m.comments.items)
	m.edit = appCommentEdit{index: -1}
	m.err = nil
	m.diffView.ClearSelection()
}

func (m *appModel) editedComment(body string) commentsComment {
	if m.edit.index >= 0 {
		comment := m.comments.items[m.edit.index]
		comment.Body = body
		return comment
	}
	return commentsComment{Anchor: m.edit.anchor, Body: body}
}

func (m *appModel) applyLoadedComments(msg appCommentsLoadedMsg) {
	if msg.revision != m.comments.revision {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh comments: %w", msg.err)
		return
	}
	m.comments.items = msg.comments
	m.commentView.Update(m.comments.items)
	m.err = nil
}

func (m *appModel) applyRefreshedPatch(msg appRefreshDiffMsg) {
	if msg.kind != m.kind {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh diff: %w", msg.err)
		return
	}
	m.currentPatch = msg.patch
	m.diffView.Update(msg.patch)
	m.err = nil
}

func (m appModel) refreshDiffCommand() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg {
		patch, err := m.refresh(kind)
		return appRefreshDiffMsg{patch: patch, kind: kind, err: err}
	}
}

func (m appModel) updateKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.diffView.EndDrag()
	name := msg.String()
	switch m.mode {
	case appModeComments:
		return m.updateCommentModeKey(name)
	case appModeHelp:
		return m.updateHelpModeKey(name)
	case appModeSearch:
		return m.updateSearchModeKey(msg)
	default:
		return m.updateBrowseModeKey(name)
	}
}

func (m appModel) updateHelpModeKey(name string) (tea.Model, tea.Cmd) {
	if name == "esc" || name == "?" || name == "q" {
		m.mode = appModeBrowse
	}
	return m, nil
}

func (m appModel) updateSearchModeKey(
	msg tea.KeyPressMsg,
) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.diffView.CancelSearch()
		m.mode = appModeBrowse
	case "enter":
		m.diffView.AcceptSearch()
		m.mode = appModeBrowse
	case "backspace":
		m.diffView.BackspaceSearch()
	default:
		if msg.Text != "" {
			m.diffView.InsertSearch(msg.Text)
		}
	}
	return m, nil
}

func (m appModel) updateBrowseModeKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	name = m.keys.Feed(name)
	if m.handleNavigationKey(name) {
		return m, nil
	}
	return m.handleBrowseCommand(name)
}

func (m appModel) handleNavigationKey(name string) bool {
	if motion, ok := appNavigationMotions[name]; ok {
		m.diffView.Move(motion)
		return true
	}
	if alignment, ok := appNavigationAlignments[name]; ok {
		m.diffView.Align(alignment)
		return true
	}
	switch name {
	case "n":
		m.diffView.Find(diffscreenForward)
	case "N":
		m.diffView.Find(diffscreenBackward)
	case "h", "left":
		m.diffView.ScrollHorizontal(-appHorizontalScrollStep)
	case "l", "right":
		m.diffView.ScrollHorizontal(appHorizontalScrollStep)
	case "0":
		m.diffView.ScrollHorizontal(-int(^uint(0) >> 1))
	case "$":
		m.diffView.ScrollHorizontal(int(^uint(0) >> 1))
	default:
		return false
	}
	return true
}

func (m appModel) handleBrowseCommand(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "c":
		return m.startCommentCommand()
	case "e":
		return m.startSourceEditorCommand()
	case "R":
		return m, m.refreshDiffCommand()
	case "tab":
		return m.togglePatchKind()
	default:
		return m.updateBrowseToggle(name)
	}
}

func (m appModel) startCommentCommand() (tea.Model, tea.Cmd) {
	cmd, err := (&m).startCommentFromSelection()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m appModel) startSourceEditorCommand() (tea.Model, tea.Cmd) {
	path, line, err := editedSourceLocation(m)
	if err != nil {
		m.err = err
		return m, nil
	}
	cmd, err := m.startSourceEditor(m.ctx, path, line, func(err error) tea.Msg {
		return appSourceEditorFinishedMsg{err: err}
	})
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m appModel) togglePatchKind() (tea.Model, tea.Cmd) {
	if m.currentPatch.Branch == "" {
		return m, nil
	}
	if m.kind == patchUnstaged {
		m.kind = patchBranch
	} else {
		m.kind = patchUnstaged
	}
	m.diffView.ClearSelection()
	return m, m.refreshDiffCommand()
}

func (m appModel) updateBrowseToggle(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "?":
		m.mode = appModeHelp
	case "/":
		m.mode = appModeSearch
		m.diffView.BeginSearch()
	case "v":
		m.diffView.ToggleSelection()
	case "esc":
		m.diffView.ClearSelection()
	case "C":
		return m.openCommentScreen()
	case "t":
		m = m.toggleSideBySide()
	default:
		return m, nil
	}
	return m, nil
}

func (m appModel) openCommentScreen() (tea.Model, tea.Cmd) {
	m.mode = appModeComments
	m.commentView.Update(m.comments.items)
	if m.load == nil {
		return m, nil
	}
	revision := m.comments.revision
	return m, func() tea.Msg {
		comments, err := m.load()
		return appCommentsLoadedMsg{
			comments: comments,
			revision: revision,
			err:      err,
		}
	}
}

func (m appModel) toggleSideBySide() appModel {
	m.diffOptions.SideBySide = !m.diffOptions.SideBySide
	m.diffView.Configure(m.diffOptions)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.diffOptions.SideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
	return m
}

func (m appModel) handleMouseInput(msg tea.Msg) {
	switch event := msg.(type) {
	case tea.MouseClickMsg:
		m.handleMouseClick(event)
	case tea.MouseMotionMsg:
		m.handleMouseMotion(event)
	case tea.MouseReleaseMsg:
		m.handleMouseRelease(event)
	case tea.MouseWheelMsg:
		m.handleMouseWheel(event)
	default:
		return
	}
}

func (m appModel) handleMouseClick(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft {
		return
	}
	switch m.mode {
	case appModeBrowse:
		m.diffView.BeginDrag(msg.X, msg.Y)
	case appModeComments:
		m.commentView.Click(msg.X, msg.Y)
	default:
		return
	}
}

func (m appModel) handleMouseMotion(msg tea.MouseMotionMsg) {
	if msg.Button == tea.MouseLeft && m.mode == appModeBrowse {
		m.diffView.DragTo(msg.X, msg.Y)
	}
}

func (m appModel) handleMouseRelease(msg tea.MouseReleaseMsg) {
	if msg.Button != tea.MouseLeft && msg.Button != tea.MouseNone {
		return
	}
	if m.mode == appModeBrowse {
		m.diffView.DragTo(msg.X, msg.Y)
	}
	m.diffView.EndDrag()
}

func (m appModel) handleMouseWheel(msg tea.MouseWheelMsg) {
	m.diffView.EndDrag()
	if m.mode != appModeBrowse && m.mode != appModeSearch {
		return
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.diffView.ScrollVertical(-appVerticalScrollStep)
	case tea.MouseWheelDown:
		m.diffView.ScrollVertical(appVerticalScrollStep)
	case tea.MouseWheelLeft:
		m.diffView.ScrollHorizontal(-appHorizontalScrollStep)
	case tea.MouseWheelRight:
		m.diffView.ScrollHorizontal(appHorizontalScrollStep)
	default:
		return
	}
}

func (m appModel) updateCommentModeKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = appModeBrowse
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "j", "down":
		m.commentView.Move(1)
	case "k", "up":
		m.commentView.Move(-1)
	case "enter", "e":
		return m.editSelectedComment()
	case "D":
		m = m.deleteSelectedComment()
	default:
		return m, nil
	}
	return m, nil
}

func (m appModel) editSelectedComment() (tea.Model, tea.Cmd) {
	selected, ok := m.commentView.Selected()
	if !ok {
		return m, nil
	}
	m.edit.index = selectedCommentIndex(m.comments.items, selected)
	if m.edit.index < 0 {
		return m, nil
	}
	m.edit.body = m.comments.items[m.edit.index].Body
	m.edit.anchor = m.comments.items[m.edit.index].Anchor
	cmd, err := newCommentEditorCommand(
		m.ctx,
		m.edit.body,
		m.edit.anchor,
		func(body string, err error) tea.Msg {
			return appCommentEditorFinishedMsg{body: body, err: err}
		},
	)
	if err != nil {
		m.err = err
		m.edit = appCommentEdit{index: -1}
		return m, nil
	}
	return m, cmd
}

func (m appModel) deleteSelectedComment() appModel {
	selected, ok := m.commentView.Selected()
	if !ok {
		return m
	}
	return m.deleteCommentAt(selectedCommentIndex(m.comments.items, selected))
}

func (m appModel) deleteCommentAt(index int) appModel {
	if index < 0 || index >= len(m.comments.items) {
		return m
	}
	if m.delete == nil {
		m.err = errors.New("comment storage is unavailable")
		return m
	}
	if err := m.delete(m.comments.items[index], m.currentPatch); err != nil {
		m.err = err
		return m
	}
	m.comments.items = append(
		m.comments.items[:index],
		m.comments.items[index+1:]...,
	)
	m.commentView.Update(m.comments.items)
	m.comments.revision++
	m.err = nil
	return m
}

func selectedCommentIndex(
	items []commentsComment,
	selected commentsComment,
) int {
	for index, item := range items {
		if selected.ID != "" && item.ID == selected.ID {
			return index
		}
		if selected.ID == "" && item.Body == selected.Body &&
			item.Anchor.FilePath == selected.Anchor.FilePath {
			return index
		}
	}
	return -1
}

func newCommentEditorCommand(
	ctx context.Context,
	body string,
	anchor commentsAnchor,
	onComplete func(string, error) tea.Msg,
) (tea.Cmd, error) {
	command, err := configuredEditorCommand()
	if err != nil {
		return nil, err
	}
	adapter := &editorCommandAdapter{}
	adapter.run = func() error {
		path, err := createCommentDraft(body, anchor)
		if err != nil {
			return err
		}
		defer os.Remove(path)
		if err := runCommentEditor(ctx, command, path, adapter); err != nil {
			return fmt.Errorf("editor: %w", err)
		}
		adapter.body, err = readCommentDraft(path, anchor)
		return err
	}
	complete := func(err error) tea.Msg { return onComplete(adapter.body, err) }
	return tea.Exec(adapter, complete), nil
}

func (m *appModel) startCommentFromSelection() (tea.Cmd, error) {
	file, lines, ok := m.diffView.Selected()
	if !ok {
		return nil, errors.New("select code lines before commenting")
	}
	m.edit.body = ""
	m.edit.index = -1
	m.edit.anchor = selectedCommentAnchor(file, lines)
	cmd, err := newCommentEditorCommand(m.ctx, m.edit.body, m.edit.anchor,
		func(body string, err error) tea.Msg {
			return appCommentEditorFinishedMsg{body: body, err: err}
		})
	if err != nil {
		m.edit = appCommentEdit{index: -1}
	}
	return cmd, err
}

func selectedCommentAnchor(file patchFile, lines []patchLine) commentsAnchor {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := commentsAnchor{FilePath: path}
	for _, line := range lines {
		prefix := " "
		if line.Kind == patchAddition {
			prefix = "+"
		}
		if line.Kind == patchDeletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		includeAnchorLine(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		includeAnchorLine(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor
}

func includeAnchorLine(start, end *int, number int) {
	if number == 0 {
		return
	}
	if *start == 0 || number < *start {
		*start = number
	}
	if number > *end {
		*end = number
	}
}

func configuredEditorCommand() (string, error) {
	command := strings.TrimSpace(os.Getenv("EDITOR"))
	if command == "" {
		return "", errors.New("$EDITOR is not set")
	}
	return command, nil
}

func createCommentDraft(body string, anchor commentsAnchor) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(commentDraftText(body, anchor)); err != nil {
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

func commentDraftText(body string, anchor commentsAnchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := commentSuggestionLines(anchor)
	fence := suggestionFence(lines)
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	_, _ = draft.WriteString(fence + "suggestion\n")
	for _, line := range lines {
		_, _ = draft.WriteString(line)
		_ = draft.WriteByte('\n')
	}
	_, _ = draft.WriteString(fence)
	_ = draft.WriteByte('\n')
	return draft.String()
}

func commentSuggestionLines(anchor commentsAnchor) []string {
	lines := make([]string, 0, len(anchor.QuotedLines))
	for _, line := range anchor.QuotedLines {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func suggestionFence(lines []string) string {
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

func readCommentDraft(path string, anchor commentsAnchor) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	if len(anchor.QuotedLines) == 0 {
		return string(data), nil
	}
	return stripCommentSuggestion(string(data), anchor), nil
}

func stripCommentSuggestion(body string, anchor commentsAnchor) string {
	lines := commentSuggestionLines(anchor)
	fence := suggestionFence(lines)
	suggestion := fence + "suggestion\n" + strings.Join(
		lines,
		"\n",
	) + "\n" + fence
	start := strings.Index(body, suggestion)
	if start < 0 {
		return body
	}
	end := start + len(suggestion)
	before := strings.TrimRight(body[:start], "\n")
	after := body[end:]
	if strings.TrimSpace(after) == "" {
		return before
	}
	return before + "\n" + strings.TrimLeft(after, "\n")
}

func runCommentEditor(
	ctx context.Context,
	command, path string,
	adapter *editorCommandAdapter,
) error {
	quotedPath := "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	cmd := exec.CommandContext(ctx, "sh", "-c", command+" "+quotedPath)
	cmd.Stdin = adapter.stdin
	cmd.Stdout = adapter.stdout
	cmd.Stderr = adapter.stderr
	return cmd.Run()
}

func editedSourceLocation(m appModel) (string, int, error) {
	file, line, ok := m.diffView.Current()
	if !ok {
		return "", 0, errors.New("select a code line to open in $EDITOR")
	}
	path, number := file.NewPath, line.NewNumber
	if path == "" || path == "/dev/null" {
		path = file.OldPath
	}
	if number == 0 {
		number = line.OldNumber
	}
	if path == "" || path == "/dev/null" || number < 1 {
		return "", 0, errors.New(
			"current line has no editable working-tree location",
		)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.currentPatch.Root, filepath.FromSlash(path))
	}
	return path, int(number), nil
}

func sourceEditorCommand(
	ctx context.Context,
	command, path string,
	line int,
	adapter *editorCommandAdapter,
) error {
	quotedPath := "'" + strings.ReplaceAll(path, "'", "'\"'\"'") + "'"
	cmd := exec.CommandContext(ctx, "sh", "-c", command+" +"+strconv.Itoa(
		line,
	)+" "+quotedPath)
	cmd.Stdin = adapter.stdin
	cmd.Stdout = adapter.stdout
	cmd.Stderr = adapter.stderr
	return cmd.Run()
}

func (appModel) startSourceEditor(
	ctx context.Context,
	path string,
	line int,
	onComplete func(error) tea.Msg,
) (tea.Cmd, error) {
	command, err := configuredEditorCommand()
	if err != nil {
		return nil, err
	}
	adapter := &editorCommandAdapter{}
	adapter.run = func() error {
		return sourceEditorCommand(ctx, command, path, line, adapter)
	}
	return tea.Exec(adapter, onComplete), nil
}

func diffscreenPointerRow(y, width, height int) (int, bool) {
	row := y - 1
	return row, width > 0 && height > 0 && row >= 0 && row < max(
		1,
		height-3,
	) && y < height-2
}

func diffscreenPointerPane(view *diffscreenDiffView,
	viewport diffscreenDiffViewport, x int) (diffscreenDiffPane, bool) {
	if !view.split {
		return diffscreenRight, true
	}
	leftWidth := max(20, (viewport.Width-3)/2)
	switch {
	case x < leftWidth:
		return diffscreenLeft, true
	case x < leftWidth+3:
		return diffscreenRight, false
	default:
		return diffscreenRight, true
	}
}

func diffscreenCursorLine(view *diffscreenDiffView,
	cursor diffscreenDiffCursor) int {
	if cursor.row < 0 || cursor.row >= len(view.rows) {
		return -1
	}
	row := view.rows[cursor.row]
	if row.kind != diffscreenLineRow {
		return -1
	}
	if !view.split || cursor.pane == diffscreenRight {
		return row.rightLine
	}
	return row.leftLine
}

func diffscreenPointerCursor(
	view *diffscreenDiffView,
	viewport diffscreenDiffViewport,
	x, y, width, height int,
) (diffscreenDiffCursor, bool) {
	if x < 0 || x >= width {
		return diffscreenDiffCursor{}, false
	}
	row, ok := diffscreenPointerRow(y, width, height)
	if !ok {
		return diffscreenDiffCursor{}, false
	}
	row, ok = diffscreenVisibleContentRow(view, viewport, row)
	if !ok {
		return diffscreenDiffCursor{}, false
	}
	pane, ok := diffscreenPointerPane(view, viewport, x)
	if !ok {
		return diffscreenDiffCursor{}, false
	}
	cursor := diffscreenDiffCursor{row: viewport.top + row, pane: pane}
	return cursor, diffscreenCursorLine(view, cursor) >= 0
}

func diffscreenVisibleContentRow(
	view *diffscreenDiffView,
	viewport diffscreenDiffViewport,
	row int,
) (int, bool) {
	if diffscreenViewportHasFileHeader(view, viewport) {
		row--
	}
	if row < 0 || row >= max(1, diffscreenViewportContentHeight(
		view,
		viewport,
	)) {
		return 0, false
	}
	return row, true
}

func diffscreenViewportHasFileHeader(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) bool {
	return viewport.Height > 1 && viewport.top >= 0 &&
		viewport.top < len(view.rows) &&
		view.rows[viewport.top].kind != diffscreenFileRow
}

func diffscreenViewportContentHeight(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) int {
	height := viewport.Height
	if diffscreenViewportHasFileHeader(view, viewport) {
		height--
	}
	return height
}

func diffscreenSelectionForDrag(view *diffscreenDiffView, first,
	last diffscreenDiffCursor) (diffscreenDiffSelection, bool) {
	if diffscreenCursorLine(
		view,
		first,
	) < 0 || diffscreenCursorLine(
		view,
		last,
	) < 0 {
		return diffscreenDiffSelection{}, false
	}
	firstRow, lastRow := view.rows[first.row], view.rows[last.row]
	if firstRow.file != lastRow.file || firstRow.hunk != lastRow.hunk {
		return diffscreenDiffSelection{}, false
	}
	return diffscreenDiffSelection{First: first, Last: last}, true
}

func diffscreenHighlightedSource(path, source, theme string) []string {
	if source == "" {
		return nil
	}
	var output bytes.Buffer
	err := quick.Highlight(
		&output,
		source,
		path,
		"terminal16m",
		theme,
	)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	rendered, err := io.ReadAll(&output)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}

func diffscreenHighlightPair(file patchFile,
	options diffscreenOptions) diffscreenPair {
	theme := diffscreenLightTheme
	if options.Dark {
		theme = diffscreenDarkTheme
	}
	return diffscreenPair{
		Old: diffscreenHighlightedSource(file.OldPath, file.OldSource, theme),
		New: diffscreenHighlightedSource(file.NewPath, file.NewSource, theme),
	}
}

func diffscreenHunkHeader(header string) string {
	if strings.HasPrefix(header, "@@") {
		return header
	}
	return "@@ " + header
}

func diffscreenHunkLinePairs(lines []patchLine,
	options diffscreenOptions) [][2]int {
	pairs := make([][2]int, 0, len(lines))
	for index := 0; index < len(lines); {
		linePairs, next := diffscreenHunkLinePair(lines, options, index)
		pairs = append(pairs, linePairs...)
		index = next
	}
	return pairs
}

func diffscreenHunkLinePair(lines []patchLine, options diffscreenOptions,
	index int) ([][2]int,
	int) {
	line := lines[index]
	if !options.SideBySide || line.Kind == patchContext {
		return [][2]int{{index, index}}, index + 1
	}
	if line.Kind == patchAddition {
		return [][2]int{{-1, index}}, index + 1
	}
	return diffscreenPairedChangeBlock(lines, index)
}

func diffscreenPairedChangeBlock(lines []patchLine, start int) ([][2]int, int) {
	deletedEnd := diffscreenDeletionEnd(lines, start)
	addedEnd := diffscreenAdditionEnd(lines, deletedEnd)
	return diffscreenChangePairs(start, deletedEnd, addedEnd), addedEnd
}

func diffscreenDeletionEnd(lines []patchLine, start int) int {
	end := start
	for end < len(lines) && lines[end].Kind == patchDeletion {
		end++
	}
	return end
}

func diffscreenAdditionEnd(lines []patchLine, start int) int {
	end := start
	for end < len(lines) && lines[end].Kind == patchAddition {
		end++
	}
	return end
}

func diffscreenChangePairs(start, deletedEnd, addedEnd int) [][2]int {
	count := max(deletedEnd-start, addedEnd-deletedEnd)
	pairs := make([][2]int, 0, count)
	for offset := 0; offset < count; offset++ {
		pairs = append(pairs, diffscreenChangePair(
			start,
			deletedEnd,
			addedEnd,
			offset,
		))
	}
	return pairs
}

func diffscreenChangePair(start, deletedEnd, addedEnd, offset int) [2]int {
	pair := [2]int{-1, -1}
	if start+offset < deletedEnd {
		pair[0] = start + offset
	}
	if deletedEnd+offset < addedEnd {
		pair[1] = deletedEnd + offset
	}
	return pair
}

func diffscreenHighlightedLine(line patchLine,
	highlighted diffscreenPair) string {
	lines := highlighted.New
	number := line.NewNumber
	if line.Kind == patchDeletion {
		lines = highlighted.Old
		number = line.OldNumber
	}
	if number <= 0 || int(number) > len(lines) {
		return line.Text
	}
	return lines[int(number)-1]
}

func diffscreenProjectedLineRow(fileIndex, hunkIndex int, hunk patchHunk,
	pair [2]int, options diffscreenOptions,
	highlighted diffscreenPair) diffscreenEntry {
	row := diffscreenEntry{
		kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex,
		leftLine: pair[0], rightLine: pair[1],
	}
	if !options.SideBySide {
		row.text = diffscreenHighlightedLine(
			hunk.Lines[row.rightLine],
			highlighted,
		)
		return row
	}
	if row.leftLine >= 0 {
		row.left = diffscreenHighlightedLine(
			hunk.Lines[row.leftLine],
			highlighted,
		)
	}
	if row.rightLine >= 0 {
		row.right = diffscreenHighlightedLine(
			hunk.Lines[row.rightLine],
			highlighted,
		)
	}
	return row
}

func diffscreenProjectedFileRows(fileIndex int, file patchFile,
	options diffscreenOptions) []diffscreenEntry {
	rows := []diffscreenEntry{{
		kind: diffscreenFileRow, file: fileIndex, hunk: -1,
		leftLine: -1, rightLine: -1, text: file.DisplayPath,
	}}
	for _, metadata := range file.Metadata {
		rows = append(rows, diffscreenEntry{
			kind: diffscreenMetadataRow, file: fileIndex, hunk: -1,
			leftLine: -1, rightLine: -1, text: metadata,
		})
	}
	highlighted := diffscreenHighlightPair(file, options)
	for hunkIndex, hunk := range file.Hunks {
		rows = append(rows, diffscreenEntry{
			kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex,
			leftLine: -1, rightLine: -1, text: diffscreenHunkHeader(
				hunk.Header,
			),
		})
		for _, pair := range diffscreenHunkLinePairs(hunk.Lines, options) {
			rows = append(rows, diffscreenProjectedLineRow(
				fileIndex,
				hunkIndex,
				hunk,
				pair,
				options,
				highlighted,
			))
		}
	}
	return rows
}

func diffscreenBuildDiffView(patch patchPatch,
	options diffscreenOptions) *diffscreenDiffView {
	view := &diffscreenDiffView{
		patch: patch,
		split: options.SideBySide,
		dark:  options.Dark,
	}
	for fileIndex, file := range patch.Files {
		view.rows = append(view.rows, diffscreenProjectedFileRows(
			fileIndex,
			file,
			options,
		)...)
	}
	return view
}

func diffscreenCursorIdentityAt(view *diffscreenDiffView,
	cursor diffscreenDiffCursor) diffscreenCursorIdentity {
	lineIndex := diffscreenCursorLine(view, cursor)
	if lineIndex < 0 {
		return diffscreenCursorIdentity{}
	}
	row := view.rows[cursor.row]
	file := view.patch.Files[row.file]
	return diffscreenCursorIdentity{
		file:   file,
		hunk:   file.Hunks[row.hunk],
		line:   file.Hunks[row.hunk].Lines[lineIndex],
		cursor: cursor,
		valid:  true,
	}
}

func diffscreenInitialViewport(view *diffscreenDiffView, width,
	height int) diffscreenDiffViewport {
	viewport := diffscreenDiffViewport{Width: max(
		1,
		width,
	), Height: max(
		1,
		height,
	)}
	return diffscreenClampViewport(view, viewport)
}

func diffscreenPreservedViewport(view *diffscreenDiffView,
	viewport diffscreenDiffViewport, leftColumn int) diffscreenDiffViewport {
	maxColumn := diffscreenViewportLeft(view, viewport)
	columns := max(-viewport.LeftColumn, min(
		leftColumn,
		maxColumn-viewport.LeftColumn,
	))
	viewport.LeftColumn += columns
	return diffscreenClampViewport(view, viewport)
}

func diffscreenCursorMatchRank(candidate, original patchLine) int {
	if candidate.Kind != original.Kind {
		return 1
	}
	if candidate.OldNumber == original.OldNumber &&
		candidate.NewNumber == original.NewNumber {
		return 3
	}
	if candidate.Text == original.Text {
		return 2
	}
	return 1
}

func diffscreenAbsoluteDistance(distance int) int {
	if distance < 0 {
		return -distance
	}
	return distance
}

func diffscreenCursorMatchDistance(candidate, original patchLine,
	rowDistance int) int {
	if candidate.NewNumber > 0 && original.NewNumber > 0 {
		return diffscreenAbsoluteDistance(int(
			candidate.NewNumber - original.NewNumber,
		))
	}
	if candidate.OldNumber > 0 && original.OldNumber > 0 {
		return diffscreenAbsoluteDistance(int(
			candidate.OldNumber - original.OldNumber,
		))
	}
	return diffscreenAbsoluteDistance(rowDistance)
}

func diffscreenBetterCursorMatch(candidate,
	best diffscreenCursorMatch) diffscreenCursorMatch {
	if candidate.rank > best.rank ||
		candidate.rank == best.rank && candidate.distance < best.distance {
		return candidate
	}
	return best
}

func diffscreenFileMatchesIdentity(file patchFile, identity patchFile) bool {
	oldPathMatches := file.OldPath != "" && file.OldPath == identity.OldPath
	newPathMatches := file.NewPath != "" && file.NewPath == identity.NewPath
	return oldPathMatches || newPathMatches
}

func diffscreenCursorMatchAt(view *diffscreenDiffView,
	identity diffscreenCursorIdentity, row int,
	pane diffscreenDiffPane) (diffscreenCursorMatch, bool) {
	cursor := diffscreenDiffCursor{row: row, pane: pane}
	lineIndex := diffscreenCursorLine(view, cursor)
	if lineIndex < 0 {
		return diffscreenCursorMatch{}, false
	}
	entry := view.rows[row]
	file := view.patch.Files[entry.file]
	line := file.Hunks[entry.hunk].Lines[lineIndex]
	return diffscreenCursorMatch{
		cursor: cursor,
		rank:   diffscreenCursorMatchRank(line, identity.line),
		distance: diffscreenCursorMatchDistance(
			line,
			identity.line,
			row-identity.cursor.row,
		),
	}, true
}

func diffscreenFindCursorMatch(view *diffscreenDiffView,
	identity diffscreenCursorIdentity) (diffscreenDiffCursor, bool) {
	if !identity.valid {
		return diffscreenDiffCursor{}, false
	}
	best, exact := diffscreenSearchCursorRows(view, identity)
	if exact {
		return best.cursor, true
	}
	return best.cursor, best.rank > 0
}

func diffscreenSearchCursorRows(view *diffscreenDiffView,
	identity diffscreenCursorIdentity) (diffscreenCursorMatch, bool) {
	best := diffscreenCursorMatch{}
	for row, entry := range view.rows {
		if !diffscreenRowMatchesIdentity(view, entry, identity) {
			continue
		}
		candidate, exact, ok := diffscreenBestCursorMatchAtRow(
			view, identity, row,
		)
		if exact {
			return candidate, true
		}
		if ok {
			best = diffscreenBetterCursorMatch(candidate, best)
		}
	}
	return best, false
}

func diffscreenRowMatchesIdentity(view *diffscreenDiffView,
	entry diffscreenEntry, identity diffscreenCursorIdentity) bool {
	return entry.hunk >= 0 && diffscreenFileMatchesIdentity(
		view.patch.Files[entry.file], identity.file,
	)
}

func diffscreenBestCursorMatchAtRow(
	view *diffscreenDiffView,
	identity diffscreenCursorIdentity,
	row int,
) (match diffscreenCursorMatch, exact, found bool) {
	best := diffscreenCursorMatch{}
	for _, pane := range diffscreenSearchPanes(view, identity.cursor.pane) {
		candidate, ok := diffscreenCursorMatchAt(view, identity, row, pane)
		if !ok {
			continue
		}
		if candidate.rank == 3 {
			return candidate, true, true
		}
		best = diffscreenBetterCursorMatch(candidate, best)
	}
	return best, false, best.rank > 0
}

func diffscreenTranslatedSelection(next, old *diffscreenDiffView,
	screen *diffscreenView) *diffscreenDiffSelection {
	selection := screen.selection
	if selection == nil {
		return nil
	}
	firstIdentity := diffscreenCursorIdentityAt(old, selection.First)
	lastIdentity := diffscreenCursorIdentityAt(old, selection.Last)
	if !firstIdentity.valid || !lastIdentity.valid ||
		!diffscreenFileMatchesIdentity(firstIdentity.file, lastIdentity.file) ||
		firstIdentity.hunk.Header != lastIdentity.hunk.Header {
		return nil
	}
	first, firstOK := diffscreenFindCursorMatch(next, firstIdentity)
	last, lastOK := diffscreenFindCursorMatch(next, lastIdentity)
	if !firstOK || !lastOK {
		return nil
	}
	firstRow := next.rows[first.row]
	lastRow := next.rows[last.row]
	if firstRow.file != lastRow.file || firstRow.hunk != lastRow.hunk {
		return nil
	}
	return &diffscreenDiffSelection{First: first, Last: last}
}

func diffscreenPreserveSearch(screen *diffscreenView, next *diffscreenDiffView,
	origin diffscreenCursorIdentity) {
	if !screen.search.active {
		return
	}
	screen.search.from = screen.cursor
	if translated, ok := diffscreenTranslateCursorIdentity(next, origin); ok {
		screen.search.from = translated
	}
	if screen.search.query == "" {
		screen.cursor = screen.search.from
		screen.viewport = diffscreenCursorViewport(
			next,
			screen.viewport,
			screen.cursor,
		)
		screen.search.miss = false
		return
	}
	cursor, ok := diffscreenFindSearchCursor(
		next,
		screen.search.query,
		screen.search.from,
	)
	screen.search.miss = !ok
	if ok {
		screen.cursor = cursor
		screen.viewport = diffscreenCursorViewport(
			next,
			screen.viewport,
			cursor,
		)
	}
}

func diffscreenViewportBottom(view *diffscreenDiffView, height int) int {
	bottom := max(0, len(view.rows)-height)
	if height > 1 && bottom < len(
		view.rows,
	) && view.rows[bottom].kind != diffscreenFileRow {
		bottom++
	}
	return bottom
}

func diffscreenViewportLeft(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) int {
	contentWidth := max(1, viewport.Width-14)
	extraWidth := 0
	if view.split {
		contentWidth, extraWidth = max(1, (viewport.Width-3)/2-6), 2
	}
	longest := 0
	for _, row := range view.rows {
		if row.kind != diffscreenLineRow {
			continue
		}
		text := ansi.Strip(row.text + row.left + row.right)
		text = strings.ReplaceAll(text, "\t", "    ")
		longest = max(longest, lipgloss.Width(text)+extraWidth)
	}
	return max(0, longest-contentWidth)
}

func diffscreenClampViewport(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) diffscreenDiffViewport {
	viewport.top = max(0, min(viewport.top, diffscreenViewportBottom(
		view,
		viewport.Height,
	)))
	viewport.LeftColumn = max(0, min(
		viewport.LeftColumn,
		diffscreenViewportLeft(view, viewport),
	))
	return viewport
}

func diffscreenCursorViewport(
	view *diffscreenDiffView,
	viewport diffscreenDiffViewport,
	cursor diffscreenDiffCursor,
) diffscreenDiffViewport {
	viewport = diffscreenClampViewport(view, viewport)
	if diffscreenCursorLine(view, cursor) < 0 {
		return viewport
	}
	for range 2 {
		height := max(1, diffscreenViewportContentHeight(view, viewport))
		viewport.top = diffscreenCursorViewportTop(cursor, viewport, height)
		viewport = diffscreenClampViewport(view, viewport)
	}
	return viewport
}

func diffscreenCursorViewportTop(
	cursor diffscreenDiffCursor,
	viewport diffscreenDiffViewport,
	height int,
) int {
	if cursor.row < viewport.top {
		return cursor.row
	}
	if cursor.row >= viewport.top+height {
		return cursor.row - height + 1
	}
	return viewport.top
}

func diffscreenMoveFileOrPane(screen *diffscreenView, motion diffscreenMotion) {
	switch motion {
	case diffscreenPreviousFile:
		screen.ClearSelection()
		diffscreenMoveFile(screen, diffscreenPrivateBackward)
	case diffscreenNextFile:
		screen.ClearSelection()
		diffscreenMoveFile(screen, diffscreenPrivateForward)
	case diffscreenOldPane:
		diffscreenSwitchPane(screen, diffscreenLeft)
	case diffscreenNewPane:
		diffscreenSwitchPane(screen, diffscreenRight)
	case diffscreenOtherPane:
		diffscreenSwitchPane(screen, diffscreenOppositePane(screen.cursor.pane))
	default:
		return
	}
}

func diffscreenMoveFile(screen *diffscreenView,
	direction diffscreenPrivateDirection) {
	cursor, ok := diffscreenJumpFile(
		screen.view,
		screen.cursor,
		direction,
	)
	if ok {
		screen.cursor = cursor
		screen.viewport = diffscreenCursorVisibleViewport(
			cursor,
			screen.view,
			screen.viewport,
		)
	}
}

func diffscreenLastCursor(view *diffscreenDiffView) (diffscreenDiffCursor,
	bool) {
	for _, pane := range []diffscreenDiffPane{diffscreenRight, diffscreenLeft} {
		for row := len(view.rows) - 1; row >= 0; row-- {
			cursor := diffscreenDiffCursor{row: row, pane: pane}
			if diffscreenCursorLine(view, cursor) >= 0 {
				return cursor, true
			}
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenMovePage(screen *diffscreenView,
	direction diffscreenPrivateDirection) {
	viewport, cursor := diffscreenPageTarget(
		screen.view,
		screen.viewport,
		screen.cursor,
		direction,
	)
	if screen.selection != nil {
		selection, ok := diffscreenSelectionForDrag(
			screen.view,
			screen.selection.First,
			cursor,
		)
		if !ok {
			return
		}
		screen.selection = &selection
	}
	screen.viewport, screen.cursor = viewport, cursor
}

func diffscreenPageTarget(view *diffscreenDiffView,
	viewport diffscreenDiffViewport, cursor diffscreenDiffCursor,
	direction diffscreenPrivateDirection) (diffscreenDiffViewport,
	diffscreenDiffCursor) {
	if diffscreenCursorLine(view, cursor) < 0 {
		return viewport, cursor
	}
	distance := int(direction) * max(1, viewport.Height/2)
	viewport.top += distance
	viewport = diffscreenClampViewport(view, viewport)
	target := min(len(view.rows)-1, max(0, cursor.row+distance))
	candidate, ok := diffscreenNearestCursor(
		view,
		viewport,
		cursor.pane,
		target,
		direction,
	)
	if ok {
		cursor = candidate
	}
	return viewport, cursor
}

func diffscreenMoveLine(screen *diffscreenView,
	direction diffscreenPrivateDirection) {
	cursor, ok := diffscreenScanCursor(screen.view, screen.cursor, direction)
	if !ok {
		return
	}
	if screen.selection != nil {
		selection, ok := diffscreenSelectionForDrag(
			screen.view,
			screen.selection.First,
			cursor,
		)
		if !ok {
			return
		}
		screen.selection = &selection
	}
	screen.cursor = cursor
	screen.viewport = diffscreenCursorViewport(
		screen.view,
		screen.viewport,
		cursor,
	)
}

func diffscreenSwitchPane(screen *diffscreenView, pane diffscreenDiffPane) {
	cursor, ok := diffscreenFindCursorInPane(pane, screen.view, screen.cursor)
	if !ok {
		return
	}
	var selection *diffscreenDiffSelection
	if screen.selection != nil {
		mapped, ok := diffscreenSelectionInPane(
			pane,
			screen.view,
			*screen.selection,
		)
		if !ok {
			return
		}
		selection = &mapped
	}
	screen.cursor = cursor
	screen.selection = selection
	screen.viewport = diffscreenCursorVisibleViewport(
		cursor,
		screen.view,
		screen.viewport,
	)
}

func diffscreenSelectionInPane(pane diffscreenDiffPane,
	view *diffscreenDiffView,
	selection diffscreenDiffSelection) (diffscreenDiffSelection, bool) {
	first, firstOK := diffscreenFindCursorInPane(pane, view, selection.First)
	last, lastOK := diffscreenFindCursorInPane(pane, view, selection.Last)
	if !firstOK || !lastOK {
		return diffscreenDiffSelection{}, false
	}
	firstRow := view.rows[first.row]
	lastRow := view.rows[last.row]
	if firstRow.file != lastRow.file || firstRow.hunk != lastRow.hunk {
		return diffscreenDiffSelection{}, false
	}
	return diffscreenDiffSelection{First: first, Last: last}, true
}

func diffscreenAlignmentOffset(alignment diffscreenAlignment, height int) int {
	switch alignment {
	case diffscreenCenter:
		return height / 2
	case diffscreenBottom:
		return height - 1
	default:
		return 0
	}
}

func diffscreenViewportHeight(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) int {
	height := viewport.Height
	if height > 1 && viewport.top >= 0 && viewport.top < len(
		view.rows,
	) && view.rows[viewport.top].kind != diffscreenFileRow {
		height--
	}
	return max(1, height)
}

func diffscreenCursorVisible(view *diffscreenDiffView,
	viewport diffscreenDiffViewport, cursor diffscreenDiffCursor) bool {
	if diffscreenCursorLine(view, cursor) < 0 {
		return false
	}
	return cursor.row >= viewport.top &&
		cursor.row < viewport.top+diffscreenViewportHeight(
			view,
			viewport,
		)
}

func diffscreenNearestCursor(view *diffscreenDiffView,
	viewport diffscreenDiffViewport, pane diffscreenDiffPane, target int,
	direction diffscreenPrivateDirection) (diffscreenDiffCursor, bool) {
	height := diffscreenViewportHeight(view, viewport)
	for distance := 0; distance < height; distance++ {
		for _, row := range []int{target + int(
			direction,
		)*distance, target - int(direction)*distance} {
			cursor := diffscreenDiffCursor{row: row, pane: pane}
			if diffscreenCursorVisible(view, viewport, cursor) {
				return cursor, true
			}
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenScrollDirection(row, target int) diffscreenPrivateDirection {
	if row > target {
		return diffscreenPrivateBackward
	}
	return diffscreenPrivateForward
}

func diffscreenNearestScrollCursor(
	view *diffscreenDiffView,
	viewport diffscreenDiffViewport,
	cursor diffscreenDiffCursor,
	target int,
	direction diffscreenPrivateDirection,
) (diffscreenDiffCursor, bool) {
	candidate, ok := diffscreenNearestCursor(
		view, viewport, cursor.pane, target, direction,
	)
	if ok || !view.split {
		return candidate, ok
	}
	return diffscreenNearestCursor(
		view, viewport, diffscreenOppositePane(cursor.pane), target, direction,
	)
}

func diffscreenScrollSelection(
	view *diffscreenDiffView,
	selection *diffscreenDiffSelection,
	cursor diffscreenDiffCursor,
) (*diffscreenDiffSelection, bool) {
	if selection == nil {
		return nil, true
	}
	updated, ok := diffscreenSelectionForDrag(view, selection.First, cursor)
	if !ok {
		return selection, false
	}
	return &updated, true
}

func diffscreenPatchLineAtCursor(view *diffscreenDiffView,
	cursor diffscreenDiffCursor) (patchFile, patchLine, bool) {
	lineIndex := diffscreenCursorLine(view, cursor)
	if lineIndex < 0 {
		return patchFile{}, patchLine{}, false
	}
	row := view.rows[cursor.row]
	file := view.patch.Files[row.file]
	line := file.Hunks[row.hunk].Lines[lineIndex]
	return file, line, true
}

func diffscreenSelectionFile(view *diffscreenDiffView,
	selection diffscreenDiffSelection) (patchFile, bool) {
	file, _, firstOK := diffscreenPatchLineAtCursor(view, selection.First)
	_, _, lastOK := diffscreenPatchLineAtCursor(view, selection.Last)
	if !firstOK || !lastOK {
		return file, false
	}
	firstRow := view.rows[selection.First.row]
	lastRow := view.rows[selection.Last.row]
	return file, firstRow.file == lastRow.file && firstRow.hunk == lastRow.hunk
}

func diffscreenLinesInSelection(view *diffscreenDiffView,
	selection diffscreenDiffSelection) []patchLine {
	_, first, firstOK := diffscreenPatchLineAtCursor(view, selection.First)
	_, last, lastOK := diffscreenPatchLineAtCursor(view, selection.Last)
	if !firstOK || !lastOK {
		return nil
	}
	if diffscreenSelectionCrossesPanes(selection) {
		return diffscreenLinesAcrossPanes(first, last)
	}
	return diffscreenLinesAcrossRows(view, selection)
}

func diffscreenSelectionCrossesPanes(selection diffscreenDiffSelection) bool {
	return selection.First.row == selection.Last.row &&
		selection.First.pane != selection.Last.pane
}

func diffscreenLinesAcrossPanes(first, last patchLine) []patchLine {
	if first == last {
		return []patchLine{first}
	}
	return []patchLine{first, last}
}

func diffscreenLinesAcrossRows(view *diffscreenDiffView,
	selection diffscreenDiffSelection) []patchLine {
	start := min(selection.First.row, selection.Last.row)
	end := max(selection.First.row, selection.Last.row)
	lines := make([]patchLine, 0, end-start+1)
	for row := start; row <= end; row++ {
		cursor := diffscreenDiffCursor{
			row:  row,
			pane: diffscreenSelectionPaneAtRow(selection, row),
		}
		if _, line, ok := diffscreenPatchLineAtCursor(view, cursor); ok {
			lines = append(lines, line)
		}
	}
	return lines
}

func diffscreenSelectionPaneAtRow(selection diffscreenDiffSelection,
	row int) diffscreenDiffPane {
	if row == selection.Last.row {
		return selection.Last.pane
	}
	return selection.First.pane
}

func diffscreenPatchChangeCounts(patch patchPatch) (added, removed int) {
	for _, file := range patch.Files {
		fileAdded, fileRemoved := diffscreenFileChangeCounts(file)
		added += fileAdded
		removed += fileRemoved
	}
	return added, removed
}

func diffscreenFileChangeCounts(file patchFile) (added, removed int) {
	for _, hunk := range file.Hunks {
		hunkAdded, hunkRemoved := diffscreenHunkChangeCounts(hunk.Lines)
		added += hunkAdded
		removed += hunkRemoved
	}
	return added, removed
}

func diffscreenHunkChangeCounts(lines []patchLine) (added, removed int) {
	for _, line := range lines {
		switch line.Kind {
		case patchAddition:
			added++
		case patchDeletion:
			removed++
		default:
			continue
		}
	}
	return added, removed
}

func diffscreenRenderBody(screen *diffscreenView) []string {
	if len(screen.view.patch.Files) != 0 {
		return strings.Split(diffscreenRenderPatch(
			screen.view,
			screen.viewport,
			screen.cursor,
			screen.selection,
		), "\n")
	}
	empty := "No unstaged or untracked changes."
	if screen.view.patch.Kind == patchBranch {
		empty = "No branch or worktree changes."
	}
	body := make([]string, max(1, screen.height-3))
	body[min(1, len(body)-1)] = diffscreenMutedStyle.Render(empty)
	return body
}

func diffscreenStatus(screen *diffscreenView) string {
	if screen.search.active {
		status := "/" + screen.search.query + diffscreenCursorStyle.Render(" ")
		if screen.search.miss {
			status += diffscreenErrorStyle.Render("  no matches")
		}
		return status
	}
	if screen.selection != nil {
		return "visual selection  j/k extend  c comment  Esc cancel"
	}
	return "j/k/h/l move  c comment  ? help  q quit"
}

func diffscreenRenderFooter(screen *diffscreenView, err error) string {
	footer := diffscreenMutedStyle.Render(diffscreenStatus(screen))
	if screen.search.repeatMiss {
		footer = diffscreenErrorStyle.Render(fmt.Sprintf(
			"no matches for %q",
			screen.search.term,
		))
	}
	if err != nil {
		footer = diffscreenErrorStyle.Render(strings.ReplaceAll(
			err.Error(),
			"\n",
			" ",
		))
	}
	return diffscreenFooterWithLabel(screen, footer)
}

func diffscreenFooterWithLabel(screen *diffscreenView, footer string) string {
	label := diffscreenFooterLabel(screen.view.patch)
	if screen.viewport.top > 0 {
		label += fmt.Sprintf(" (%d%%)", diffscreenViewportPercent(
			screen.view,
			screen.viewport,
		))
	}
	right := diffscreenMutedStyle.Render(label)
	width := max(20, screen.width)
	footer = ansi.Truncate(footer, max(0, width-lipgloss.Width(right)-1), "")
	padding := strings.Repeat(" ", max(1, width-lipgloss.Width(
		footer,
	)-lipgloss.Width(right)))
	return footer + padding + right
}

func diffscreenFooterLabel(patch patchPatch) string {
	if patch.Kind == patchBranch {
		return "branch changes from " + patch.Branch
	}
	return "local changes"
}

func diffscreenViewportPercent(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) int {
	if len(view.rows) == 0 {
		return 0
	}
	bottom := min(len(
		view.rows,
	), viewport.top+diffscreenViewportHeight(
		view,
		viewport,
	))
	return bottom * 100 / len(view.rows)
}

func diffscreenRenderFrame(header string, body []string, footer string,
	height int) string {
	bodyHeight := max(1, height-3)
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, header)
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+1 {
		lines = append(lines, "")
	}
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}

func diffscreenFindSearchCursor(view *diffscreenDiffView, query string,
	cursor diffscreenDiffCursor) (diffscreenDiffCursor, bool) {
	return diffscreenFindSearchCursorDirection(
		view,
		query,
		cursor,
		diffscreenPrivateForward,
	)
}

func diffscreenFindSearchCursorDirection(view *diffscreenDiffView, query string,
	cursor diffscreenDiffCursor,
	direction diffscreenPrivateDirection) (diffscreenDiffCursor, bool) {
	if query == "" || diffscreenCursorLine(view, cursor) < 0 {
		return diffscreenDiffCursor{}, false
	}
	query = strings.ToLower(query)
	step := int(direction)
	for count := 0; count < len(view.rows)-1; count++ {
		row := (cursor.row + step + len(view.rows)) % len(view.rows)
		cursor.row = row
		candidate, ok := diffscreenSearchRow(
			view,
			query,
			cursor,
			row,
			direction,
		)
		if ok {
			return candidate, true
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenSearchRow(
	view *diffscreenDiffView,
	query string,
	cursor diffscreenDiffCursor,
	row int,
	direction diffscreenPrivateDirection,
) (diffscreenDiffCursor, bool) {
	entry := view.rows[row]
	if entry.kind == diffscreenLineRow {
		return diffscreenSearchLineRow(view, query, cursor, row)
	}
	text := strings.ToLower(ansi.Strip(entry.text))
	if !strings.Contains(text, query) {
		return diffscreenDiffCursor{}, false
	}
	return diffscreenSearchHeaderCursor(view, row, cursor.pane, direction)
}

func diffscreenSearchLineRow(
	view *diffscreenDiffView,
	query string,
	cursor diffscreenDiffCursor,
	row int,
) (diffscreenDiffCursor, bool) {
	for _, pane := range diffscreenSearchPanes(view, cursor.pane) {
		candidate, ok := diffscreenCursorAtRow(view, pane, row)
		if !ok {
			continue
		}
		if diffscreenCursorMatchesQuery(view, candidate, query) {
			return candidate, true
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenSearchPanes(view *diffscreenDiffView,
	pane diffscreenDiffPane) []diffscreenDiffPane {
	panes := []diffscreenDiffPane{pane}
	if view.split {
		panes = append(panes, diffscreenOppositePane(pane))
	}
	return panes
}

func diffscreenCursorMatchesQuery(view *diffscreenDiffView,
	cursor diffscreenDiffCursor, query string) bool {
	_, line, ok := diffscreenPatchLineAtCursor(view, cursor)
	return ok && strings.Contains(strings.ToLower(line.Text), query)
}

func diffscreenSearchHeaderCursor(
	view *diffscreenDiffView,
	row int,
	pane diffscreenDiffPane,
	direction diffscreenPrivateDirection,
) (diffscreenDiffCursor, bool) {
	step := int(direction)
	for distance := 1; distance <= len(view.rows); distance++ {
		for _, candidateRow := range []int{
			row + step*distance,
			row - step*distance,
		} {
			candidate, ok := diffscreenSearchHeaderCandidate(
				view, row, candidateRow, pane,
			)
			if ok {
				return candidate, true
			}
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenSearchHeaderCandidate(
	view *diffscreenDiffView,
	row, candidateRow int,
	pane diffscreenDiffPane,
) (diffscreenDiffCursor, bool) {
	if candidateRow < 0 || candidateRow >= len(view.rows) ||
		view.rows[candidateRow].file != view.rows[row].file {
		return diffscreenDiffCursor{}, false
	}
	if candidate, ok := diffscreenCursorAtRow(view, pane, candidateRow); ok {
		return candidate, true
	}
	if view.split {
		return diffscreenCursorAtRow(
			view, diffscreenOppositePane(pane), candidateRow,
		)
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenOppositePane(pane diffscreenDiffPane) diffscreenDiffPane {
	if pane == diffscreenLeft {
		return diffscreenRight
	}
	return diffscreenLeft
}

func diffscreenRenderPatch(
	view *diffscreenDiffView,
	viewport diffscreenDiffViewport,
	cursor diffscreenDiffCursor,
	selection *diffscreenDiffSelection,
) string {
	viewport = diffscreenViewportClamp(view, &viewport)
	lines := diffscreenRenderViewportHeader(view, viewport)
	lines = append(lines, diffscreenRenderVisibleRows(
		view, viewport, cursor, selection,
	)...)
	for len(lines) < viewport.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func diffscreenRenderViewportHeader(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) []string {
	if !diffscreenViewportHasFileHeader(view, viewport) {
		return nil
	}
	entry := view.rows[viewport.top]
	path := view.patch.Files[entry.file].DisplayPath
	header := diffscreenFileStyle.Width(max(20, viewport.Width)).Render(path)
	return []string{header}
}

func diffscreenRenderVisibleRows(
	view *diffscreenDiffView,
	viewport diffscreenDiffViewport,
	cursor diffscreenDiffCursor,
	selection *diffscreenDiffSelection,
) []string {
	end := diffscreenViewportRowsEnd(view, viewport)
	lines := make([]string, 0, max(0, end-viewport.top))
	for row := viewport.top; row < end; row++ {
		entry := view.rows[row]
		lines = append(lines, diffscreenRenderViewportRow(
			view, entry, row, viewport, cursor, selection,
		))
	}
	return lines
}

func diffscreenViewportRowsEnd(view *diffscreenDiffView,
	viewport diffscreenDiffViewport) int {
	height := diffscreenViewportContentHeight(view, viewport)
	return min(len(view.rows), viewport.top+max(1, height))
}

func diffscreenRenderViewportRow(
	view *diffscreenDiffView,
	entry diffscreenEntry,
	row int,
	viewport diffscreenDiffViewport,
	cursor diffscreenDiffCursor,
	selection *diffscreenDiffSelection,
) string {
	if view.split && entry.kind == diffscreenLineRow {
		return diffscreenRenderSplitRow(
			view,
			entry,
			row,
			viewport,
			cursor,
			selection,
		)
	}
	return diffscreenRenderUnifiedRow(
		entry,
		cursor,
		selection,
		view,
		viewport,
		row,
	)
}

func diffscreenRenderDiffCell(
	view *diffscreenDiffView,
	entry diffscreenEntry,
	row int,
	pane diffscreenDiffPane,
	width, offset int,
	cursor diffscreenDiffCursor,
	selection *diffscreenDiffSelection,
) string {
	lineIndex := diffscreenEntryLineIndex(entry, pane)
	if lineIndex < 0 {
		return strings.Repeat(" ", width)
	}
	line := view.patch.Files[entry.file].Hunks[entry.hunk].Lines[lineIndex]
	text, number := diffscreenEntryLineText(entry, line, pane)
	gutter := diffscreenNumberGutter(number)
	value := gutter + diffscreenFitText(
		diffscreenLinePrefix(line.Kind)+text,
		width-lipgloss.Width(gutter),
		offset,
	)
	candidate := diffscreenDiffCursor{row: row, pane: pane}
	style := diffscreenLineStyle(line.Kind, view.dark)
	colors := diffscreenPreserveForeground
	if diffscreenSelectionContainsCursor(selection, candidate) {
		style = diffscreenSelectionStyle(view.dark)
		colors = diffscreenDiscardForeground
	}
	if cursor == candidate {
		style = diffscreenCursorStyle
		colors = diffscreenDiscardForeground
	}
	return diffscreenRenderStyledCell(value, width, 0, style, colors)
}

func diffscreenEntryLineIndex(entry diffscreenEntry,
	pane diffscreenDiffPane) int {
	if entry.kind != diffscreenLineRow {
		return -1
	}
	if pane == diffscreenRight {
		return entry.rightLine
	}
	return entry.leftLine
}

func diffscreenEntryLineText(entry diffscreenEntry, line patchLine,
	pane diffscreenDiffPane) (string, patchLineNumber) {
	if pane == diffscreenLeft {
		return entry.left, line.OldNumber
	}
	return entry.right, line.NewNumber
}

func diffscreenNumberGutter(number patchLineNumber) string {
	value := ""
	if number != 0 {
		value = strconv.Itoa(int(number))
	}
	return fmt.Sprintf("%5s ", value)
}

func diffscreenLinePrefix(kind patchLineKind) string {
	switch kind {
	case patchAddition:
		return diffscreenAddedStyle.Render("+") + " "
	case patchDeletion:
		return diffscreenRemovedStyle.Render("-") + " "
	default:
		return "  "
	}
}

func diffscreenRenderSplitRow(
	view *diffscreenDiffView,
	entry diffscreenEntry,
	row int,
	viewport diffscreenDiffViewport,
	cursor diffscreenDiffCursor,
	selection *diffscreenDiffSelection,
) string {
	leftWidth := max(20, (viewport.Width-3)/2)
	rightWidth := max(20, viewport.Width-3-leftWidth)
	left := diffscreenRenderDiffCell(
		view, entry, row, diffscreenLeft, leftWidth,
		viewport.LeftColumn, cursor, selection,
	)
	right := diffscreenRenderDiffCell(
		view, entry, row, diffscreenRight, rightWidth,
		viewport.LeftColumn, cursor, selection,
	)
	return left + " │ " + right
}

func diffscreenLineNumberAtCursor(cursor diffscreenDiffCursor,
	v *diffscreenDiffView,

) int {
	var current = v.rows[cursor.row]
	if current.kind != diffscreenLineRow {
		return -1
	}
	if !v.split || cursor.pane == diffscreenRight {
		return current.rightLine
	}
	return current.leftLine
}

func diffscreenTranslateCursorIdentity(
	next *diffscreenDiffView,
	origin diffscreenCursorIdentity,
) (diffscreenDiffCursor, bool) {
	return diffscreenFindCursorMatch(next, origin)
}

func diffscreenLineForCursor(candidate diffscreenDiffCursor,
	next *diffscreenDiffView) (patchLine, bool) {
	_, line, ok := diffscreenPatchLineAtCursor(next, candidate)
	return line, ok
}

func diffscreenFirstCursor(next *diffscreenDiffView) (diffscreenDiffCursor,
	bool) {
	for _, pane := range []diffscreenDiffPane{diffscreenRight, diffscreenLeft} {
		for row := range next.rows {
			cursor := diffscreenDiffCursor{row: row, pane: pane}
			if diffscreenCursorLine(next, cursor) >= 0 {
				return cursor, true
			}
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenCursorVisibleViewport(cursor diffscreenDiffCursor,
	v *diffscreenDiffView,
	viewport diffscreenDiffViewport) diffscreenDiffViewport {
	return diffscreenCursorViewport(v, viewport, cursor)
}

func diffscreenCursorAtRow(next *diffscreenDiffView, pane diffscreenDiffPane,
	y int) (diffscreenDiffCursor, bool) {
	cursor := diffscreenDiffCursor{row: y, pane: pane}
	return cursor, diffscreenCursorLine(next, cursor) >= 0
}

func diffscreenScanCursor(view *diffscreenDiffView, start diffscreenDiffCursor,
	direction diffscreenPrivateDirection) (diffscreenDiffCursor, bool) {
	if diffscreenCursorLine(view, start) < 0 || direction == 0 {
		return diffscreenDiffCursor{}, false
	}
	step := int(direction)
	for row := start.row + step; row >= 0 && row < len(view.rows); row += step {
		if cursor, ok := diffscreenCursorAtRow(view, start.pane, row); ok {
			return cursor, true
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenJumpFile(
	view *diffscreenDiffView,
	cursor diffscreenDiffCursor,
	direction diffscreenPrivateDirection,
) (diffscreenDiffCursor, bool) {
	file, _, ok := diffscreenPatchLineAtCursor(view, cursor)
	if !ok {
		return diffscreenDiffCursor{}, false
	}
	for {
		next, changed, ok := diffscreenNextFileCursor(
			view, cursor, direction, file,
		)
		if !ok {
			return diffscreenDiffCursor{}, false
		}
		if changed {
			return next, true
		}
		cursor = next
	}
}

func diffscreenNextFileCursor(
	view *diffscreenDiffView,
	cursor diffscreenDiffCursor,
	direction diffscreenPrivateDirection,
	file patchFile,
) (nextCursor diffscreenDiffCursor, changed, valid bool) {
	next, ok := diffscreenScanCursor(view, cursor, direction)
	if !ok {
		return diffscreenDiffCursor{}, false, false
	}
	changed, valid = diffscreenCursorMovedToFile(view, next, file)
	return next, changed, valid
}

func diffscreenCursorMovedToFile(
	view *diffscreenDiffView,
	cursor diffscreenDiffCursor,
	file patchFile,
) (changed, valid bool) {
	nextFile, _, valid := diffscreenPatchLineAtCursor(view, cursor)
	if !valid {
		return false, false
	}
	return nextFile.OldPath != file.OldPath || nextFile.NewPath != file.NewPath,
		true
}

func diffscreenViewportClamp(v *diffscreenDiffView,
	viewport *diffscreenDiffViewport) diffscreenDiffViewport {
	return diffscreenClampViewport(v, *viewport)
}

func diffscreenContentWidth(v *diffscreenDiffView,
	viewport *diffscreenDiffViewport) int {
	return diffscreenViewportLeft(v, *viewport)
}

func diffscreenFindCursorInPane(
	pane diffscreenDiffPane,
	view *diffscreenDiffView,
	cursor diffscreenDiffCursor,
) (diffscreenDiffCursor, bool) {
	if !view.split || diffscreenCursorLine(view, cursor) < 0 {
		return diffscreenDiffCursor{}, false
	}
	if candidate, ok := diffscreenCursorAtRow(view, pane, cursor.row); ok {
		return candidate, true
	}
	if candidate, ok := diffscreenPreviousCursorInFile(view, pane, cursor); ok {
		return candidate, true
	}
	return diffscreenNextCursor(view, pane, cursor.row)
}

func diffscreenPreviousCursorInFile(view *diffscreenDiffView,
	pane diffscreenDiffPane, cursor diffscreenDiffCursor) (diffscreenDiffCursor,
	bool) {
	file := view.rows[cursor.row].file
	for row := cursor.row - 1; row >= 0; row-- {
		if view.rows[row].file != file {
			return diffscreenDiffCursor{}, false
		}
		if candidate, ok := diffscreenCursorAtRow(view, pane, row); ok {
			return candidate, true
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenNextCursor(view *diffscreenDiffView, pane diffscreenDiffPane,
	row int) (diffscreenDiffCursor, bool) {
	for row++; row < len(view.rows); row++ {
		if candidate, ok := diffscreenCursorAtRow(view, pane, row); ok {
			return candidate, true
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffscreenRenderUnifiedRow(
	current diffscreenEntry,
	cursor diffscreenDiffCursor,
	selection *diffscreenDiffSelection,
	view *diffscreenDiffView,
	viewport diffscreenDiffViewport,
	row int,
) string {
	width := max(20, viewport.Width)
	switch current.kind {
	case diffscreenFileRow:
		return diffscreenFileStyle.Width(width).Render(current.text)
	case diffscreenMetadataRow:
		return diffscreenMetadataStyle.Render("  " + current.text)
	case diffscreenHunkRow:
		return diffscreenHunkStyle.Render(current.text)
	case diffscreenLineRow:
		return diffscreenRenderUnifiedLine(
			view, current, row, viewport, cursor, selection,
		)
	default:
		return ""
	}
}

func diffscreenRenderUnifiedLine(
	view *diffscreenDiffView,
	entry diffscreenEntry,
	row int,
	viewport diffscreenDiffViewport,
	cursor diffscreenDiffCursor,
	selection *diffscreenDiffSelection,
) string {
	file := view.patch.Files[entry.file]
	line := file.Hunks[entry.hunk].Lines[entry.rightLine]
	gutter := diffscreenLineGutter(line)
	width := max(20, viewport.Width)
	textWidth := width - lipgloss.Width(gutter)
	value := gutter + diffscreenFitText(
		entry.text,
		textWidth,
		viewport.LeftColumn,
	)
	style := diffscreenLineStyle(line.Kind, view.dark)
	colors := diffscreenPreserveForeground
	candidate := diffscreenDiffCursor{row: row, pane: cursor.pane}
	if diffscreenSelectionContainsCursor(selection, candidate) {
		style = diffscreenSelectionStyle(view.dark)
		colors = diffscreenDiscardForeground
	}
	if cursor.row == row {
		style = diffscreenCursorStyle
		colors = diffscreenDiscardForeground
	}
	return diffscreenRenderStyledCell(value, width, 0, style, colors)
}

func diffscreenLineGutter(line patchLine) string {
	prefix := " "
	if line.Kind == patchAddition {
		prefix = diffscreenAddedStyle.Render("+")
	}
	if line.Kind == patchDeletion {
		prefix = diffscreenRemovedStyle.Render("-")
	}
	oldNumber, newNumber := "", ""
	if line.OldNumber != 0 {
		oldNumber = strconv.Itoa(int(line.OldNumber))
	}
	if line.NewNumber != 0 {
		newNumber = strconv.Itoa(int(line.NewNumber))
	}
	return fmt.Sprintf("%5s %5s %s ", oldNumber, newNumber, prefix)
}

func diffscreenLineStyle(kind patchLineKind, dark bool) lipgloss.Style {
	lightDark := lipgloss.LightDark(dark)
	switch kind {
	case patchAddition:
		return lipgloss.NewStyle().Background(lightDark(
			lipgloss.Color("#dafbe1"), lipgloss.Color("#1b3823"),
		))
	case patchDeletion:
		return lipgloss.NewStyle().Background(lightDark(
			lipgloss.Color("#ffebe9"), lipgloss.Color("#402222"),
		))
	default:
		return diffscreenContextStyle
	}
}

func diffscreenSelectionContainsCursor(selection *diffscreenDiffSelection,
	cursor diffscreenDiffCursor) bool {
	if selection == nil {
		return false
	}
	first, last := selection.First, selection.Last
	if first.row == last.row && first.pane != last.pane {
		return cursor.row == first.row &&
			(cursor.pane == first.pane || cursor.pane == last.pane)
	}
	if first.pane != cursor.pane {
		return false
	}
	return cursor.row >= min(
		first.row,
		last.row,
	) && cursor.row <= max(
		first.row,
		last.row,
	)
}

func diffscreenSelectionStyle(dark bool) lipgloss.Style {
	return lipgloss.NewStyle().Background(lipgloss.LightDark(dark)(
		lipgloss.Color("#dbeafe"), lipgloss.Color("#1e3a5f"),
	))
}

func diffscreenRenderStyledCell(value string, width, offset int,
	style lipgloss.Style, colors diffscreenForegroundPolicy) string {
	value = diffscreenStripSGRColors(value, colors)
	fitted := diffscreenFitText(value, width, offset)
	prefix := diffscreenStylePrefix(style)
	if prefix != "" {
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func diffscreenFitText(value string, width, offset int) string {
	if width <= 0 {
		return ""
	}
	value = strings.ReplaceAll(value, "\t", "    ")
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func diffscreenStripSGRColors(value string,
	policy diffscreenForegroundPolicy) string {
	return diffscreenAnsiSGRPattern.ReplaceAllStringFunc(
		value,
		func(sequence string) string {
			parameters := sequence[2 : len(sequence)-1]
			if parameters == "" {
				return sequence
			}
			filtered := diffscreenFilteredSGR(parameters, policy)
			if filtered == "" {
				return ""
			}
			return "\x1b[" + filtered + "m"
		},
	)
}

func diffscreenFilteredSGR(parameters string,
	policy diffscreenForegroundPolicy) string {
	parts := strings.Split(parameters, ";")
	filtered := make([]string, 0, len(parts))
	for index := 0; index < len(parts); index++ {
		parameter, next, keep := diffscreenFilteredSGRParameter(
			parts,
			index,
			policy,
		)
		if keep {
			filtered = append(filtered, parameter)
		}
		index = next
	}
	return strings.Join(filtered, ";")
}

func diffscreenFilteredSGRParameter(
	parts []string,
	index int,
	policy diffscreenForegroundPolicy,
) (string, int, bool) {
	parameter := parts[index]
	code, err := strconv.Atoi(parameter)
	if err != nil {
		return parameter, index, true
	}
	if diffscreenSGRHasColorParameters(code, policy) {
		return parameter, diffscreenSGRColorEnd(parts, index), false
	}
	keep := !diffscreenSGRColorCode(code) &&
		!(policy == diffscreenDiscardForeground && diffscreenSGRForegroundCode(
			code,
		))
	return parameter, index, keep
}

func diffscreenSGRHasColorParameters(code int,
	policy diffscreenForegroundPolicy) bool {
	return code == 48 || policy == diffscreenDiscardForeground && code == 38
}

func diffscreenSGRColorEnd(parts []string, index int) int {
	if index+1 >= len(parts) {
		return index
	}
	switch parts[index+1] {
	case "2":
		return min(index+4, len(parts)-1)
	case "5":
		return min(index+2, len(parts)-1)
	default:
		return index
	}
}

func diffscreenSGRColorCode(code int) bool {
	return code >= 40 && code <= 49 || code >= 100 && code <= 107
}

func diffscreenSGRForegroundCode(code int) bool {
	return code >= 30 && code <= 39 || code >= 90 && code <= 97
}

func diffscreenStylePrefix(style lipgloss.Style) string {
	const marker = "\x00"
	rendered := style.Render(marker)
	index := strings.Index(rendered, marker)
	if index < 0 {
		return ""
	}
	return rendered[:index]
}

func addStoredComment(tx *bolt.Tx, comment commentsComment, data []byte) error {
	bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
	if err != nil {
		return err
	}
	total := 0
	cursor := bucket.Cursor()
	for _, value := cursor.First(); value != nil; _, value = cursor.Next() {
		total += len(value)
	}
	if total+len(data) > commentsMaxPendingBytes {
		return fmt.Errorf(
			"pending feedback exceeds %d bytes",
			commentsMaxPendingBytes,
		)
	}
	if bucket.Get([]byte(comment.ID)) != nil {
		return errors.New("comment ID already exists")
	}
	return bucket.Put([]byte(comment.ID), data)
}

func repositoryComments(
	tx *bolt.Tx,
	repository string,
) ([]commentsComment, error) {
	bucket := tx.Bucket(commentsMessagesBucket)
	if bucket == nil {
		return nil, nil
	}
	var comments []commentsComment
	err := bucket.ForEach(func(_, value []byte) error {
		comment, err := decodeStoredComment(value)
		if err != nil {
			return err
		}
		if comment.Repository == repository {
			comments = append(comments, comment)
		}
		return nil
	})
	return comments, err
}

func updateStoredComment(
	tx *bolt.Tx,
	comment commentsComment,
	data []byte,
) error {
	bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
	if err != nil {
		return err
	}
	key, err := findStoredCommentKey(bucket, comment.Repository, comment.ID)
	if err != nil {
		return err
	}
	if key == nil {
		return errors.New("comment is no longer in the comments")
	}
	if !bytes.Equal(key, []byte(comment.ID)) {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return bucket.Put([]byte(comment.ID), data)
}

func deleteStoredComment(tx *bolt.Tx, repository, id string) error {
	bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
	if err != nil {
		return err
	}
	key, err := findStoredCommentKey(bucket, repository, id)
	if err != nil {
		return err
	}
	if key == nil {
		return errors.New("comment is no longer in the comments")
	}
	return bucket.Delete(key)
}

func acknowledgeStoredComments(
	tx *bolt.Tx,
	repository string,
	wanted map[string]struct{},
) error {
	bucket, err := tx.CreateBucketIfNotExists(commentsMessagesBucket)
	if err != nil {
		return err
	}
	keys, err := acknowledgedCommentKeys(bucket, repository, wanted)
	if err != nil {
		return err
	}
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func acknowledgedCommentKeys(
	bucket *bolt.Bucket,
	repository string,
	wanted map[string]struct{},
) ([][]byte, error) {
	var keys [][]byte
	err := bucket.ForEach(func(key, value []byte) error {
		comment, err := decodeStoredComment(value)
		if err != nil {
			return err
		}
		if comment.Repository != repository {
			return nil
		}
		if _, ok := wanted[comment.ID]; !ok {
			return nil
		}
		keys = append(keys, append([]byte(nil), key...))
		return nil
	})
	return keys, err
}

func (s commentsStore) open() (*bolt.DB, error) {
	if s.Path == "" {
		return nil, errors.New("comments path is empty")
	}
	directory := filepath.Dir(s.Path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create comments directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure comments directory: %w", err)
	}
	db, err := bolt.Open(s.Path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open comments: %w", err)
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("secure comments database: %w", err)
	}
	return db, nil
}

func validateCommentBody(body string) error {
	if len(body) == 0 {
		return errors.New("comment body is empty")
	}
	if len(body) > commentsMaxCommentBytes {
		return fmt.Errorf("comment exceeds %d bytes", commentsMaxCommentBytes)
	}
	return nil
}

func decodeStoredComment(value []byte) (commentsComment, error) {
	var legacy commentsLegacyRecord
	if err := json.Unmarshal(value, &legacy); err != nil {
		return commentsComment{}, fmt.Errorf("decode comment: %w", err)
	}
	if legacy.Comment != nil {
		return commentsComment{
			ID:         legacy.ID,
			Repository: legacy.Repository,
			CreatedAt:  legacy.CreatedAt,
			Anchor:     legacy.Comment.Anchor,
			Body:       legacy.Comment.Body,
		}, nil
	}
	var comment commentsComment
	if err := json.Unmarshal(value, &comment); err != nil {
		return commentsComment{}, fmt.Errorf("decode comment: %w", err)
	}
	return comment, nil
}

func findStoredCommentKey(
	bucket *bolt.Bucket,
	repository, id string,
) ([]byte, error) {
	cursor := bucket.Cursor()
	for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
		comment, err := decodeStoredComment(value)
		if err != nil {
			return nil, err
		}
		if matchesStoredComment(comment, repository, id) {
			return append([]byte(nil), key...), nil
		}
	}
	return nil, nil
}

func commentIDs(comments []commentsComment) []string {
	ids := make([]string, len(comments))
	for i, comment := range comments {
		ids[i] = comment.ID
	}
	return ids
}

func writePendingComments(w io.Writer, comments []commentsComment) error {
	if len(comments) == 0 {
		_, err := fmt.Fprintln(w, "No pending review comments.")
		return err
	}
	if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range comments {
		if err := writePendingComment(w, index+1, comment); err != nil {
			return err
		}
	}
	return nil
}

func writePendingComment(
	w io.Writer,
	number int,
	comment commentsComment,
) error {
	anchor := comment.Anchor
	if err := writePendingCommentHeading(w, number, anchor); err != nil {
		return err
	}
	if err := writeQuotedAnchor(w, anchor.QuotedLines); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
	return err
}

func writePendingCommentHeading(
	w io.Writer,
	number int,
	anchor commentsAnchor,
) error {
	_, err := fmt.Fprintf(
		w,
		"\n### %d. `%s` (%s)\n\n",
		number,
		anchor.FilePath,
		commentAnchorLocation(anchor),
	)
	return err
}

func writeQuotedAnchor(w io.Writer, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "```diff"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "```")
	return err
}

func commentAnchorLocation(anchor commentsAnchor) string {
	sides := anchorSides(anchor)
	if len(sides) == 0 {
		return "diff lines"
	}
	return strings.Join(sides, ", ")
}

func anchorSides(anchor commentsAnchor) []string {
	var sides []string
	if anchor.OldStart > 0 {
		sides = append(sides, formatAnchorSide(
			"old",
			anchor.OldStart,
			anchor.OldEnd,
		))
	}
	if anchor.NewStart > 0 {
		sides = append(sides, formatAnchorSide(
			"new",
			anchor.NewStart,
			anchor.NewEnd,
		))
	}
	return sides
}

func matchesStoredComment(comment commentsComment, repository, id string) bool {
	return comment.Repository == repository && comment.ID == id
}

func formatAnchorSide(side string, start, end int) string {
	if end == 0 || end == start {
		return fmt.Sprintf("%s line %d", side, start)
	}
	return fmt.Sprintf("%s lines %d-%d", side, start, end)
}

func diffHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf(
		"@@ -%d,%d +%d,%d @@",
		h.OrigStartLine,
		h.OrigLines,
		h.NewStartLine,
		h.NewLines,
	)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}

func parseDiffHunkLines(h *diff.Hunk) ([]patchLine, error) {
	oldLine, newLine := h.OrigStartLine, h.NewStartLine
	if len(h.Body) == 0 {
		return []patchLine{}, nil
	}
	rawLines := bytes.Split(
		bytes.TrimSuffix(h.Body, []byte("\n")),
		[]byte("\n"),
	)
	lines := make([]patchLine, 0, len(rawLines))
	for _, raw := range rawLines {
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := escapeDiffLine(string(raw[1:]))
		switch raw[0] {
		case ' ':
			lines = append(lines, patchLine{
				Kind: patchContext, Text: text,
				OldNumber: patchLineNumber(oldLine),
				NewNumber: patchLineNumber(newLine),
			})
			oldLine++
			newLine++
		case '+':
			lines = append(lines, patchLine{
				Kind: patchAddition, Text: text, NewNumber: patchLineNumber(
					newLine,
				),
			})
			newLine++
		case '-':
			lines = append(lines, patchLine{
				Kind: patchDeletion, Text: text, OldNumber: patchLineNumber(
					oldLine,
				),
			})
			oldLine++
		case '\\':
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func escapeDiffLine(text string) string {
	var escaped strings.Builder
	for _, r := range text {
		switch {
		case r == '\n' || r == '\t':
			_, _ = escaped.WriteRune(r)
		case r == '\r':
			_, _ = escaped.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&escaped, `\x%02x`, r)
		default:
			_, _ = escaped.WriteRune(r)
		}
	}
	return strings.ReplaceAll(escaped.String(), "\n", `\n`)
}

func discoverDefaultBranch(
	ctx context.Context,
	root string,
	runner patchRunner,
) (string, error) {
	branchBytes, err := runner.Run(
		ctx,
		root,
		"symbolic-ref",
		"--quiet",
		"--short",
		"refs/remotes/origin/HEAD",
	)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err == nil {
		return strings.TrimSpace(string(branchBytes)), nil
	}
	for _, candidate := range []string{
		"origin/main",
		"main",
		"origin/master",
		"master",
	} {
		_, err := runner.Run(
			ctx,
			root,
			"rev-parse",
			"--verify",
			"--quiet",
			candidate+"^{commit}",
		)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

func userCommentsStore() (commentsStore, error) {
	dataDirectory, err := userDataDirectory()
	if err != nil {
		return commentsStore{}, err
	}
	return commentsStore{Path: filepath.Join(dataDirectory, "comments.db")}, nil
}

func userDataDirectory() (string, error) {
	if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, commentsAppName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", commentsAppName), nil
}

func readIndexSource(
	ctx context.Context,
	run patchRunner,
	root, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > patchMaxFileBytes ||
		bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceText(string(out))
}

func escapeSourceText(source string) string {
	var result strings.Builder
	for _, r := range source {
		switch {
		case r == '\n' || r == '\t':
			_, _ = result.WriteRune(r)
		case r == '\r':
			_, _ = result.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, r)
		default:
			_, _ = result.WriteRune(r)
		}
	}
	return result.String()
}

func loadCurrentPatch(ctx context.Context, kind patchKind) (patchPatch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return patchPatch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	if err := validatePatchLoad(ctx, kind); err != nil {
		return patchPatch{}, err
	}
	root, err := resolveRepositoryRoot(ctx, directory)
	if err != nil {
		return patchPatch{}, err
	}
	branch, err := findDefaultBranch(ctx, root)
	if err != nil {
		return patchPatch{}, fmt.Errorf("discover default branch: %w", err)
	}
	return loadRepositoryPatch(ctx, kind, root, branch)
}

func validatePatchLoad(ctx context.Context, kind patchKind) error {
	if kind != patchUnstaged && kind != patchBranch {
		return fmt.Errorf("unknown patch kind %d", kind)
	}
	return ctx.Err()
}

func loadRepositoryPatch(
	ctx context.Context,
	kind patchKind,
	root, branch string,
) (patchPatch, error) {
	revisions, readOld, err := branchComparisonInputs(ctx, branch, kind, root)
	if err != nil {
		return patchPatch{}, err
	}
	raw, err := runPatchDiff(ctx, revisions, root)
	if err != nil {
		return patchPatch{}, err
	}
	files, err := parsePatchDiff(ctx, raw, readOld, root)
	if err != nil {
		return patchPatch{}, err
	}
	return finishPatchLoad(ctx, kind, root, branch, files)
}

func finishPatchLoad(
	ctx context.Context,
	kind patchKind,
	root, branch string,
	files []patchFile,
) (patchPatch, error) {
	untracked, err := loadUntrackedFiles(ctx, root)
	if err != nil {
		return patchPatch{}, err
	}
	files = append(files, untracked...)
	slices.SortStableFunc(files, comparePatchFilesByPath)
	if err := ctx.Err(); err != nil {
		return patchPatch{}, err
	}
	return patchPatch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
}

func comparePatchFilesByPath(a, b patchFile) int {
	return cmp.Compare(a.DisplayPath, b.DisplayPath)
}

func userSettingsPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve UI settings directory: %w", err)
	}
	return filepath.Join(config, "review-my-slop", "ui.json"), nil
}

func readSettingsPreferences(path string) (settingsPreferences, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settingsPreferences{}, nil
	}
	if err != nil {
		return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
	}
	var settings settingsPreferences
	if err := json.Unmarshal(data, &settings); err != nil {
		return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
	}
	return settings, nil
}

func runCommand(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return runCodeReviewCommand(ctx)
	}
	if len(args) > 1 {
		return errors.New("usage: review-my-slop [code|comments]")
	}
	switch args[0] {
	case "code":
		return runCodeReviewCommand(ctx)
	case "comments":
		return writeCommandPendingComments(ctx, output)
	default:
		return fmt.Errorf(
			"unknown subcommand %q; usage: review-my-slop [code|comments]",
			args[0],
		)
	}
}

func writeCommandPendingComments(ctx context.Context, output io.Writer) error {
	commentStore, err := userCommentsStore()
	if err != nil {
		return err
	}
	currentPatch, err := loadCurrentPatch(ctx, patchUnstaged)
	if err != nil {
		return err
	}
	return commentStore.WritePending(output, currentPatch.Root)
}

func runCodeReviewCommand(ctx context.Context) error {
	commentStore, err := userCommentsStore()
	if err != nil {
		return err
	}
	currentPatch, err := loadCurrentPatch(ctx, patchUnstaged)
	if err != nil {
		return err
	}
	return runReviewApplication(
		ctx,
		currentPatch,
		commentStore,
		loadCurrentPatch,
		currentTerminalSize,
		launchReviewProgram,
	)
}

func currentTerminalSize() appSize {
	if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
		return appSize{Width: width, Height: height}
	}
	if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
		return appSize{Width: width, Height: height}
	}
	return appDefaultSize
}

func launchReviewProgram(m appModel, options ...tea.ProgramOption) error {
	_, err := tea.NewProgram(m, options...).Run()
	return err
}

func writeUserSettingsFile(path string, preferences settingsPreferences) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create UI settings directory: %w", err)
	}
	data, err := json.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("encode UI settings: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), "ui-*.tmp")
	if err != nil {
		return fmt.Errorf("create UI settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure UI settings file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write UI settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close UI settings: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace UI settings: %w", err)
	}
	return nil
}

func runReviewApplication(
	ctx context.Context,
	currentPatch patchPatch,
	store appCommentStore,
	refresh func(context.Context, patchKind) (patchPatch, error),
	terminalSize func() appSize,
	launch func(appModel, ...tea.ProgramOption) error,
) error {
	pending, err := store.List(currentPatch.Root)
	if err != nil {
		return err
	}
	dimensions := terminalSize()
	m, err := buildReviewModel(currentPatch, dimensions, pending, store)
	if err != nil {
		return err
	}
	m.ctx = ctx
	if refresh != nil {
		m.refresh = func(kind patchKind) (patchPatch, error) {
			return refresh(ctx, kind)
		}
	}
	return launch(
		m,
		tea.WithContext(ctx),
		tea.WithWindowSize(dimensions.Width, dimensions.Height),
	)
}

func loadUntrackedFiles(ctx context.Context, root string,

) ([]patchFile, error) {
	out, err := patchRunner(patchExecRunner{}).Run(
		ctx,
		root,
		"ls-files",
		"--others",
		"--exclude-standard",
		"-z",
	)
	if err != nil {
		return nil, err
	}
	var files []patchFile
	for _, path := range splitNULPaths(out) {
		file, include, err := loadUntrackedEntry(ctx, path, root)
		if err != nil {
			return nil, err
		}
		if include {
			files = append(files, file)
		}
	}
	return files, nil
}

func loadUntrackedEntry(
	ctx context.Context,
	path, root string,
) (patchFile, bool, error) {
	if err := ctx.Err(); err != nil {
		return patchFile{}, false, err
	}
	return loadOneUntrackedFile(path, root)
}

func splitNULPaths(output []byte) []string {
	var paths []string
	for path := range bytes.SplitSeq(output, []byte{0}) {
		if len(path) > 0 {
			paths = append(paths, string(path))
		}
	}
	return paths
}

func parsePatchDiff(
	ctx context.Context,
	raw []byte,
	readOld patchSourceReader,
	root string,

) ([]patchFile, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]patchFile, 0, len(parsed))
	for _, fd := range parsed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := parseOneDiffFile(ctx, fd, readOld, root)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func buildReviewModel(
	currentPatch patchPatch,
	dimensions appSize,
	pending []commentsComment,
	store appCommentStore,

) (appModel, error) {
	preferences, err := (func() (settingsPreferences, error) {
		path, err := userSettingsPath()
		if err != nil {
			return settingsPreferences{}, err
		}
		return readSettingsPreferences(path)
	})()
	if err != nil {
		return appModel{}, err
	}
	m := createReviewAppModel(
		currentPatch,
		dimensions,
		pending,
		preferences,
		store,
	)
	m.delete = func(comment commentsComment, current patchPatch) error {
		return store.Delete(current.Root, comment.ID)
	}
	m.load = func() ([]commentsComment, error) {
		return store.List(currentPatch.Root)
	}
	return m, nil
}

func normalizeDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func escapeDiffMetadata(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = escapeDiffDisplayValue(value)
	}
	return result
}

func escapeDiffDisplayValue(value string) string {
	return strings.ReplaceAll(escapeDisplayText(value), "\n", `\n`)
}

func parseOneDiffFile(
	ctx context.Context,
	fd *diff.FileDiff,
	readOld patchSourceReader,
	root string,

) (patchFile, error) {
	oldPath := normalizeDiffPath(fd.OrigName)
	newPath := normalizeDiffPath(fd.NewName)
	display := newPath
	if display == "" {
		display = oldPath
	}
	file := patchFile{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: escapeDiffDisplayValue(display),
		Metadata:    escapeDiffMetadata(fd.Extended),
	}
	file.OldSource = readOld(ctx, patchExecRunner{}, root, oldPath)
	file.NewSource = readPatchFileSource(newPath, root)
	for _, h := range fd.Hunks {
		lines, err := parseOneDiffHunk(h)
		if err != nil {
			return patchFile{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(
			file.Hunks,
			patchHunk{Header: diffHunkHeader(h), Lines: lines},
		)
	}
	return file, nil
}

func createDiffscreenView(
	p patchPatch,
	options diffscreenOptions,
) *diffscreenView {
	v := &diffscreenView{options: options, width: 80, height: 30}
	v.view = buildDiffscreenRows(p, v)
	v.viewport = computeDiffscreenViewport(
		v.view,
		v.width,
		(func(height int) int {
			return max(1, height-3)
		})(v.height),
	)
	v.cursor, _ = findInitialDiffCursor(v)
	return v
}

func highlightDiffSource(filename, source, theme string) []string {
	if source == "" {
		return nil
	}
	var buf bytes.Buffer
	err := quick.Highlight(&buf, source, filename, "terminal16m", theme)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	rendered, err := io.ReadAll(&buf)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}

func buildDiffscreenRows(p patchPatch, v *diffscreenView,

) *diffscreenDiffView {
	options := diffscreenOptions{
		SideBySide: v.options.SideBySide && v.width >= 100,
		Dark:       v.options.Dark,
	}
	view := &diffscreenDiffView{
		patch: p,
		split: options.SideBySide,
		dark:  options.Dark,
	}
	for fileIndex, file := range p.Files {
		appendDiffscreenFileRows(view, fileIndex, file)
	}
	return view
}

func appendDiffscreenFileRows(
	view *diffscreenDiffView,
	fileIndex int,
	file patchFile,
) {
	view.rows = append(view.rows, diffscreenEntry{
		kind: diffscreenFileRow, file: fileIndex, hunk: -1,
		leftLine: -1, rightLine: -1, text: file.DisplayPath,
	})
	for _, metadata := range file.Metadata {
		view.rows = append(view.rows, diffscreenEntry{
			kind: diffscreenMetadataRow, file: fileIndex, hunk: -1,
			leftLine: -1, rightLine: -1, text: metadata,
		})
	}
	highlighted := highlightedDiffPair(
		file,
		diffTheme(diffscreenOptions{Dark: view.dark}),
	)
	for hunkIndex, hunk := range file.Hunks {
		appendDiffscreenHunkRows(view, fileIndex, hunkIndex, hunk, highlighted)
	}
}

func diffTheme(options diffscreenOptions) string {
	if options.Dark {
		return diffscreenDarkTheme
	}
	return diffscreenLightTheme
}

func highlightedDiffPair(file patchFile, theme string) diffscreenPair {
	return diffscreenPair{
		Old: highlightDiffSource(file.OldPath, file.OldSource, theme),
		New: highlightDiffSource(file.NewPath, file.NewSource, theme),
	}
}

func appendDiffscreenHunkRows(
	view *diffscreenDiffView,
	fileIndex, hunkIndex int,
	hunk patchHunk,
	highlighted diffscreenPair,
) {
	header := hunk.Header
	if !strings.HasPrefix(header, "@@") {
		header = "@@ " + header
	}
	view.rows = append(view.rows, diffscreenEntry{
		kind: diffscreenHunkRow, file: fileIndex, hunk: hunkIndex,
		leftLine: -1, rightLine: -1, text: header,
	})
	for _, indices := range pairDiffHunkLines(hunk, view) {
		row := diffscreenEntry{
			kind: diffscreenLineRow, file: fileIndex, hunk: hunkIndex,
			leftLine: indices[0], rightLine: indices[1],
		}
		view.rows = append(view.rows, renderDiffscreenLine(
			row,
			hunk,
			view,
			highlighted,
		))
	}
}

func renderDiffscreenLine(
	row diffscreenEntry,
	hunk patchHunk,
	view *diffscreenDiffView,
	highlighted diffscreenPair,
) diffscreenEntry {
	if !view.split {
		row.text = highlightedDiffLine(highlighted, hunk, &row)
		return row
	}
	if row.leftLine >= 0 {
		row.left = highlightedDiffLineAt(highlighted, hunk, row.leftLine)
	}
	if row.rightLine >= 0 {
		row.right = highlightedDiffLine(highlighted, hunk, &row)
	}
	return row
}

func highlightedDiffLineAt(
	highlighted diffscreenPair,
	hunk patchHunk,
	lineIndex int,
) string {
	return highlightedDiffLine(highlighted, hunk, &diffscreenEntry{
		rightLine: lineIndex,
	})
}

func loadOneUntrackedFile(path string, root string,

) (patchFile, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return patchFile{}, false, fmt.Errorf(
			"stat untracked %q: %w",
			path,
			err,
		)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return patchFile{}, false, fmt.Errorf(
				"read symlink %q: %w",
				path,
				err,
			)
		}
		return makeUntrackedSymlinkFile(path, target), true, nil
	}
	if !info.Mode().IsRegular() {
		return patchFile{}, false, nil
	}
	return loadRegularUntrackedFile(path, full, info)
}

func loadRegularUntrackedFile(
	path, full string,
	info os.FileInfo,
) (patchFile, bool, error) {
	if info.Size() > patchMaxFileBytes {
		return omittedUntrackedFile(path), true, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return patchFile{}, false, fmt.Errorf(
			"read untracked %q: %w",
			path,
			err,
		)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return untrackedBinaryFile(path), true, nil
	}
	return makeUntrackedTextFile(content, path), true, nil
}

func omittedUntrackedFile(path string) patchFile {
	file := untrackedMetadataFile(path, "untracked file")
	file.Metadata = append(file.Metadata, "content omitted: file exceeds 2 MiB")
	return file
}

func untrackedBinaryFile(path string) patchFile {
	return untrackedMetadataFile(path, "untracked binary file")
}

func untrackedMetadataFile(path, description string) patchFile {
	return patchFile{
		NewPath:     path,
		DisplayPath: strings.ReplaceAll(escapeDisplayText(path), "\n", `\n`),
		Metadata:    []string{description},
	}
}

func makeUntrackedSymlinkFile(path string, target string,

) patchFile {
	content := strings.ReplaceAll(escapeSourceText(target), "\n", `\n`)
	return makeUntrackedSourceFile(path, content)
}

func makeUntrackedTextFile(raw []byte, path string,

) patchFile {
	return makeUntrackedSourceFile(path, escapeSourceText(string(raw)))
}

func makeUntrackedSourceFile(path, content string) patchFile {
	lines := untrackedAdditionLines(content)
	return patchFile{
		NewPath:     path,
		DisplayPath: strings.ReplaceAll(escapeDisplayText(path), "\n", `\n`),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []patchHunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func untrackedAdditionLines(content string) []patchLine {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	sourceLines := strings.Split(content, "\n")
	lines := make([]patchLine, 0, len(sourceLines))
	for index, line := range sourceLines {
		lines = append(lines, patchLine{
			Kind: patchAddition, Text: line,
			NewNumber: patchLineNumber(index + 1),
		})
	}
	return lines
}

func parseOneDiffHunk(h *diff.Hunk,

) ([]patchLine, error) {
	if len(h.Body) == 0 {
		return []patchLine{}, nil
	}
	rawBody := bytes.TrimSuffix(h.Body, []byte("\n"))
	rawLines := bytes.Split(rawBody, []byte("\n"))
	lines := make([]patchLine, 0, len(rawLines))
	oldLine := patchLineNumber(h.OrigStartLine)
	newLine := patchLineNumber(h.NewStartLine)
	for _, raw := range rawLines {
		line, include, err := parseDiffLine(raw, oldLine, newLine)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		lines = append(lines, line)
		oldLine, newLine = advancePatchLineNumbers(line.Kind, oldLine, newLine)
	}
	return lines, nil
}

func advancePatchLineNumbers(
	kind patchLineKind,
	oldLine, newLine patchLineNumber,
) (nextOldLine, nextNewLine patchLineNumber) {
	switch kind {
	case patchContext:
		return oldLine + 1, newLine + 1
	case patchAddition:
		return oldLine, newLine + 1
	case patchDeletion:
		return oldLine + 1, newLine
	default:
		return oldLine, newLine
	}
}

func parseDiffLine(
	raw []byte,
	oldLine, newLine patchLineNumber,
) (patchLine, bool, error) {
	if len(raw) == 0 {
		return patchLine{}, false, errors.New("malformed empty diff line")
	}
	line := patchLine{Text: escapeDiffDisplayValue(string(raw[1:]))}
	switch raw[0] {
	case ' ':
		line.Kind = patchContext
		line.OldNumber, line.NewNumber = oldLine, newLine
	case '+':
		line.Kind, line.NewNumber = patchAddition, newLine
	case '-':
		line.Kind, line.OldNumber = patchDeletion, oldLine
	case '\\':
		return patchLine{}, false, nil
	default:
		return patchLine{}, false, fmt.Errorf(
			"unexpected diff prefix %q",
			raw[0],
		)
	}
	return line, true, nil
}

func readPatchFileSource(newPath string, root string,

) string {
	if !validPatchSourcePath(newPath) {
		return ""
	}
	full := filepath.Join(root, filepath.FromSlash(newPath))
	out, ok := readRegularPatchSource(full)
	if !ok {
		return ""
	}
	return escapeSourceText(string(out))
}

func validPatchSourcePath(path string) bool {
	return path != "" && path != "/dev/null"
}

func readRegularPatchSource(path string) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() ||
		info.Size() > patchMaxFileBytes {
		return nil, false
	}
	out, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(out, 0) >= 0 {
		return nil, false
	}
	return out, true
}

func escapeDisplayText(path string,

) string {
	var result strings.Builder
	for _, r := range path {
		switch {
		case r == '\n' || r == '\t':
			_, _ = result.WriteRune(r)
		case r == '\r':
			_, _ = result.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, r)
		default:
			_, _ = result.WriteRune(r)
		}
	}
	return result.String()
}

func branchComparisonInputs(
	ctx context.Context,
	branch string,
	kind patchKind,
	root string,

) ([]string, patchSourceReader, error) {
	if kind != patchBranch {
		return nil, readIndexSource, nil
	}
	if branch == "" {
		return nil, nil, errors.New("default branch is unavailable")
	}
	baseBytes, err := patchRunner(
		patchExecRunner{},
	).Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf(
			"find branch point with %s: %w",
			branch,
			err,
		)
	}
	base := strings.TrimSpace(string(baseBytes))
	readOld := func(
		ctx context.Context,
		run patchRunner,
		root, path string,
	) string {
		return readBranchBaseSource(ctx, run, root, base, path)
	}
	return []string{base}, readOld, nil
}

func readBranchBaseSource(
	ctx context.Context, run patchRunner, root, base, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", base+":"+path)
	if err != nil || len(out) > patchMaxFileBytes ||
		bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceText(string(out))
}

func resolveRepositoryRoot(ctx context.Context, directory string,

) (string, error) {
	rootBytes, err := patchRunner(patchExecRunner{}).Run(
		ctx,
		directory,
		"rev-parse",
		"--show-toplevel",
	)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}

func findDefaultBranch(ctx context.Context, root string,

) (string, error) {
	return findDefaultBranchWithRunner(ctx, patchExecRunner{}, root)
}

func findDefaultBranchWithRunner(
	ctx context.Context,
	runner patchRunner,
	root string,
) (string, error) {
	out, err := runner.Run(
		ctx,
		root,
		"symbolic-ref",
		"--quiet",
		"--short",
		"refs/remotes/origin/HEAD",
	)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	for _, candidate := range []string{
		"origin/main",
		"main",
		"origin/master",
		"master",
	} {
		_, err := runner.Run(
			ctx,
			root,
			"rev-parse",
			"--verify",
			"--quiet",
			candidate+"^{commit}",
		)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

func runPatchDiff(ctx context.Context, revisions []string, root string,

) ([]byte, error) {
	args := []string{
		"-c",
		"core.quotepath=false",
		"-c",
		"diff.external=",
		"--no-pager",
		"diff",
		"--no-ext-diff",
		"--no-color",
		"--find-renames",
		"--src-prefix=a/",
		"--dst-prefix=b/",
		"--unified=3",
	}
	args = append(args, revisions...)
	args = append(args, "--")
	return patchRunner(patchExecRunner{}).Run(ctx, root, args...)
}

func pairDiffHunkLines(hunk patchHunk, v *diffscreenDiffView,

) [][2]int {
	rows := make([][2]int, 0, len(hunk.Lines))
	for index := 0; index < len(hunk.Lines); {
		row, simple := simpleDiffRow(hunk.Lines[index], index, v)
		if simple {
			rows = append(rows, row)
			index++
			continue
		}
		addedStart, end := diffChangeBounds(hunk.Lines, index)
		rows = append(rows, pairedDiffChangeRows(index, addedStart, end)...)
		index = end
	}
	return rows
}

func simpleDiffRow(
	line patchLine,
	index int,
	view *diffscreenDiffView,
) ([2]int, bool) {
	if !view.split || line.Kind == patchContext {
		return [2]int{index, index}, true
	}
	if line.Kind == patchAddition {
		return [2]int{-1, index}, true
	}
	return [2]int{}, false
}

func diffChangeBounds(
	lines []patchLine,
	start int,
) (addedStart, changeEnd int) {
	addedStart = start
	for addedStart < len(lines) && lines[addedStart].Kind == patchDeletion {
		addedStart++
	}
	changeEnd = addedStart
	for changeEnd < len(lines) && lines[changeEnd].Kind == patchAddition {
		changeEnd++
	}
	return addedStart, changeEnd
}

func pairedDiffChangeRows(start, addedStart, end int) [][2]int {
	count := max(addedStart-start, end-addedStart)
	rows := make([][2]int, 0, count)
	for offset := 0; offset < count; offset++ {
		row := [2]int{-1, -1}
		if start+offset < addedStart {
			row[0] = start + offset
		}
		if addedStart+offset < end {
			row[1] = addedStart + offset
		}
		rows = append(rows, row)
	}
	return rows
}

func highlightedDiffLine(
	highlighted diffscreenPair,
	hunk patchHunk,
	row *diffscreenEntry,

) string {
	var line = hunk.Lines[row.rightLine]
	if line.Kind == patchDeletion {
		if line.OldNumber <= 0 || int(line.OldNumber) > len(highlighted.Old) {
			return line.Text
		}
		return highlighted.Old[int(line.OldNumber)-1]
	}
	if line.NewNumber <= 0 || int(line.NewNumber) > len(highlighted.New) {
		return line.Text
	}
	return highlighted.New[int(line.NewNumber)-1]
}

func initialLayout(
	preferences settingsPreferences,
	dimensions appSize,
) appInitialLayout {
	return appInitialLayout{
		SideBySide: preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error {
			return saveLayoutPreference(enabled)
		},
		size: dimensions,
	}
}

func saveLayoutPreference(enabled bool) error {
	path, err := userSettingsPath()
	if err != nil {
		return err
	}
	return writeUserSettingsFile(path, settingsPreferences{SideBySide: enabled})
}

func newCommentView(items []commentsComment) *commentscreenView {
	v := &commentscreenView{width: 80, height: 30}
	v.Update(items)
	return v
}

func saveReviewComment(
	store appCommentStore,
) func(commentsComment, patchPatch) (commentsComment, error) {
	return func(
		comment commentsComment,
		current patchPatch,
	) (commentsComment, error) {
		comment.Repository = current.Root
		if comment.ID != "" {
			if err := store.Update(comment); err != nil {
				return commentsComment{}, err
			}
			return comment, nil
		}
		return store.Add(comment)
	}
}

func createReviewAppModel(
	currentPatch patchPatch,
	dimensions appSize,
	pending []commentsComment,
	preferences settingsPreferences,
	store appCommentStore,

) appModel {
	layout := initialLayout(preferences, dimensions)
	size := layout.size
	if size.Width <= 0 || size.Height <= 0 {
		size = appDefaultSize
	}
	m := appModel{
		ctx:          context.Background(),
		currentPatch: currentPatch,
		comments:     appCommentState{items: pending},
		commentView:  newCommentView(pending),
		edit:         appCommentEdit{index: -1},
		width:        size.Width,
		height:       size.Height,
		save:         saveReviewComment(store),
		saveLayout:   layout.SaveSideBySide,
		diffOptions: diffscreenOptions{
			SideBySide: layout.SideBySide,
			Dark:       true,
		},
		kind: currentPatch.Kind,
		keys: newKeymapMatcher(
			keymapSequence{
				Prefix:         "g",
				Keys:           []string{"g"},
				RetryUnmatched: true,
			},
			keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
			keymapSequence{Prefix: "[", Keys: []string{"f"}},
			keymapSequence{Prefix: "]", Keys: []string{"f"}},
			keymapSequence{
				Prefix: "ctrl+w",
				Keys:   []string{"h", "l", "ctrl+w"},
			},
		),
	}
	m.diffView = createDiffscreenView(currentPatch, m.diffOptions)
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	return m
}

func newKeymapMatcher(sequences ...keymapSequence) keymapMatcher {
	m := keymapMatcher{sequences: make(
		map[string]keymapSequence,
		len(sequences),
	)}
	for _, sequence := range sequences {
		sequence.Keys = append([]string(nil), sequence.Keys...)
		m.sequences[sequence.Prefix] = sequence
	}
	return m
}

func computeDiffscreenViewport(
	v *diffscreenDiffView,
	width, height int,
) diffscreenDiffViewport {
	viewport := diffscreenDiffViewport{
		Width:  max(1, width),
		Height: max(1, height),
	}
	viewport.top = min(viewport.top, maximumDiffscreenTop(v, viewport.Height))
	viewport.LeftColumn = min(viewport.LeftColumn, maximumDiffscreenLeft(
		v,
		viewport.Width,
	))
	return viewport
}

func maximumDiffscreenTop(view *diffscreenDiffView, height int) int {
	end := max(0, len(view.rows)-height)
	if height > 1 && end < len(
		view.rows,
	) && view.rows[end].kind != diffscreenFileRow {
		end++
	}
	return end
}

func maximumDiffscreenLeft(view *diffscreenDiffView, width int) int {
	contentWidth, extra := max(1, width-14), 0
	if view.split {
		contentWidth, extra = max(1, (width-3)/2-6), 2
	}
	longest := 0
	for _, row := range view.rows {
		if row.kind == diffscreenLineRow {
			longest = max(longest, diffRowDisplayWidth(row)+extra)
		}
	}
	return max(0, longest-contentWidth)
}

func diffRowDisplayWidth(row diffscreenEntry) int {
	text := ansi.Strip(row.text + row.left + row.right)
	return lipgloss.Width(strings.ReplaceAll(text, "\t", "    "))
}

func findInitialDiffCursor(view *diffscreenView) (diffscreenDiffCursor, bool) {
	if cursor, ok := findCursorInPane(view.view, diffscreenRight); ok {
		return cursor, true
	}
	return findCursorInPane(view.view, diffscreenLeft)
}

func findCursorInPane(
	view *diffscreenDiffView,
	pane diffscreenDiffPane,
) (diffscreenDiffCursor, bool) {
	for row := range view.rows {
		cursor := diffscreenDiffCursor{row: row, pane: pane}
		if diffCursorHasLine(view, cursor) {
			return cursor, true
		}
	}
	return diffscreenDiffCursor{}, false
}

func diffCursorHasLine(
	view *diffscreenDiffView,
	cursor diffscreenDiffCursor,
) bool {
	if cursor.row < 0 || cursor.row >= len(view.rows) {
		return false
	}
	entry := view.rows[cursor.row]
	if entry.kind != diffscreenLineRow {
		return false
	}
	if !view.split || cursor.pane == diffscreenRight {
		return entry.rightLine >= 0
	}
	return entry.leftLine >= 0
}
