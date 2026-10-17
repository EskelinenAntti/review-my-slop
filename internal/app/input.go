package app

import (
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func (m reviewModel) updateKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	m.diffView.EndDrag()
	name := msg.String()
	switch m.mode {
	case modeComments:
		return m.updateCommentModeKey(name)
	case modeHelp:
		return m.updateHelpModeKey(name)
	case modeSearch:
		return m.updateSearchModeKey(msg)
	default:
		return m.updateBrowseModeKey(name)
	}
}

func (m reviewModel) updateHelpModeKey(name string) (tea.Model, tea.Cmd) {
	if name == "esc" || name == "?" || name == "q" {
		m.mode = modeBrowse
	}
	return m, nil
}

func (m reviewModel) updateSearchModeKey(
	msg tea.KeyPressMsg,
) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.diffView.CancelSearch()
		m.mode = modeBrowse
	case "enter":
		m.diffView.AcceptSearch()
		m.mode = modeBrowse
	case "backspace":
		m.diffView.BackspaceSearch()
	default:
		if msg.Text != "" {
			m.diffView.InsertSearch(msg.Text)
		}
	}
	return m, nil
}

func (m reviewModel) updateBrowseModeKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	name = m.keys.Feed(name)
	if m.handleNavigationKey(name) {
		return m, nil
	}
	return m.handleBrowseCommand(name)
}

func (m reviewModel) handleNavigationKey(name string) bool {
	if motion, ok := navigationMotions[name]; ok {
		m.diffView.Move(motion)
		return true
	}
	if alignment, ok := navigationAlignments[name]; ok {
		m.diffView.Align(alignment)
		return true
	}
	switch name {
	case "n":
		m.diffView.Find(ui.Forward)
	case "N":
		m.diffView.Find(ui.Backward)
	case "h", "left":
		m.diffView.ScrollHorizontal(-horizontalScrollStep)
	case "l", "right":
		m.diffView.ScrollHorizontal(horizontalScrollStep)
	case "0":
		m.diffView.ScrollHorizontal(-int(^uint(0) >> 1))
	case "$":
		m.diffView.ScrollHorizontal(int(^uint(0) >> 1))
	default:
		return false
	}
	return true
}

func (m reviewModel) handleBrowseCommand(name string) (tea.Model, tea.Cmd) {
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

func (m reviewModel) startCommentCommand() (tea.Model, tea.Cmd) {
	cmd, err := (&m).startCommentFromSelection()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m reviewModel) startSourceEditorCommand() (tea.Model, tea.Cmd) {
	path, line, err := editedSourceLocation(m)
	if err != nil {
		m.err = err
		return m, nil
	}
	cmd, err := m.startSourceEditor(m.ctx, path, line, func(err error) tea.Msg {
		return sourceEditorFinishedMsg{err: err}
	})
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m reviewModel) togglePatchKind() (tea.Model, tea.Cmd) {
	if m.currentPatch.Branch == "" {
		return m, nil
	}
	if m.kind == patches.Unstaged {
		m.kind = patches.Branch
	} else {
		m.kind = patches.Unstaged
	}
	m.diffView.ClearSelection()
	return m, m.refreshDiffCommand()
}

func (m reviewModel) updateBrowseToggle(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "?":
		m.mode = modeHelp
	case "/":
		m.mode = modeSearch
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

func (m reviewModel) openCommentScreen() (tea.Model, tea.Cmd) {
	m.mode = modeComments
	m.commentView.Update(m.comments.items)
	if m.load == nil {
		return m, nil
	}
	revision := m.comments.revision
	return m, func() tea.Msg {
		comments, err := m.load()
		return commentsLoadedMsg{
			comments: comments,
			revision: revision,
			err:      err,
		}
	}
}

func (m reviewModel) toggleSideBySide() reviewModel {
	m.diffOptions.SideBySide = !m.diffOptions.SideBySide
	m.diffView.Configure(m.diffOptions)
	if m.saveLayout != nil {
		if err := m.saveLayout(m.diffOptions.SideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
	return m
}

func (m reviewModel) handleMouseInput(msg tea.Msg) {
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

func (m reviewModel) handleMouseClick(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft {
		return
	}
	switch m.mode {
	case modeBrowse:
		m.diffView.BeginDrag(msg.X, msg.Y)
	case modeComments:
		m.commentView.Click(msg.X, msg.Y)
	default:
		return
	}
}

func (m reviewModel) handleMouseMotion(msg tea.MouseMotionMsg) {
	if msg.Button == tea.MouseLeft && m.mode == modeBrowse {
		m.diffView.DragTo(msg.X, msg.Y)
	}
}

func (m reviewModel) handleMouseRelease(msg tea.MouseReleaseMsg) {
	if msg.Button != tea.MouseLeft && msg.Button != tea.MouseNone {
		return
	}
	if m.mode == modeBrowse {
		m.diffView.DragTo(msg.X, msg.Y)
	}
	m.diffView.EndDrag()
}

func (m reviewModel) handleMouseWheel(msg tea.MouseWheelMsg) {
	m.diffView.EndDrag()
	if m.mode != modeBrowse && m.mode != modeSearch {
		return
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		m.diffView.ScrollVertical(-verticalScrollStep)
	case tea.MouseWheelDown:
		m.diffView.ScrollVertical(verticalScrollStep)
	case tea.MouseWheelLeft:
		m.diffView.ScrollHorizontal(-horizontalScrollStep)
	case tea.MouseWheelRight:
		m.diffView.ScrollHorizontal(horizontalScrollStep)
	default:
		return
	}
}

func (m reviewModel) updateCommentModeKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	switch name {
	case "esc", "C", "q":
		m.mode = modeBrowse
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

func (m reviewModel) editSelectedComment() (tea.Model, tea.Cmd) {
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
			return commentEditorFinishedMsg{body: body, err: err}
		},
	)
	if err != nil {
		m.err = err
		m.edit = commentEdit{index: -1}
		return m, nil
	}
	return m, cmd
}

func (m reviewModel) deleteSelectedComment() reviewModel {
	selected, ok := m.commentView.Selected()
	if !ok {
		return m
	}
	return m.deleteCommentAt(selectedCommentIndex(m.comments.items, selected))
}

func (m reviewModel) deleteCommentAt(index int) reviewModel {
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
	items []feedback.Comment,
	selected feedback.Comment,
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
