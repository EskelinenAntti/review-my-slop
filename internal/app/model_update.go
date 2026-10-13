package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func (model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.diffOptions.Dark {
			m.diffOptions.Dark = dark
			m.diffView.Configure(m.diffOptions)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeScreens()

	case tea.KeyPressMsg:
		m.diffView.EndDrag()
		return m.updateKey(msg)
	case tea.MouseClickMsg, tea.MouseMotionMsg,
		tea.MouseReleaseMsg, tea.MouseWheelMsg:
		m.updateMouse(msg)
	default:
		return m.updateWorkflow(msg)
	}
	return m, nil
}

func (m model) updateWorkflow(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case commentEditorFinishedMsg:
		m.finishCommentEditor(msg)
	case commentsLoadedMsg:
		m.finishCommentsLoad(msg)
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			break
		}
		return m, m.loadRefresh()
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		m.finishDiffRefresh(msg)
	default:
		return m, nil
	}
	return m, nil
}

func (m *model) finishCommentEditor(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.clearCommentEdit()
		return
	}
	m.edit.body = msg.body
	m.finishCommentEdit()
}

func (m *model) finishCommentsLoad(msg commentsLoadedMsg) {
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

func (m *model) finishDiffRefresh(msg refreshDiffMsg) {
	if msg.kind != m.kind {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh diff: %w", msg.err)
		return
	}
	m.updatePatch(msg.patch)
	m.err = nil
}

func (m model) loadRefresh() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg {
		p, err := m.refresh(kind)
		return refreshDiffMsg{patch: p, kind: kind, err: err}
	}
}

func (m model) loadComments() tea.Cmd {
	if m.load == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		comments, err := m.load()
		return commentsLoadedMsg{
			comments: comments,
			revision: revision,
			err:      err,
		}
	}
}

func (m *model) updatePatch(p patch.Patch) {
	m.currentPatch = p
	m.diffView.Update(p)
}

func (m *model) resizeScreens() {
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
}
