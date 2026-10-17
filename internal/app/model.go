package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func (m reviewModel) View() tea.View {
	if m.quitting {
		return tea.NewView("")
	}
	result := tea.NewView(func() string {
		switch m.mode {
		case modeHelp:
			return renderAppHelp(m.width, m.height)
		case modeComments:
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

var navigationMotions = map[string]ui.Motion{
	"j":             ui.NextLine,
	"down":          ui.NextLine,
	"k":             ui.PreviousLine,
	"up":            ui.PreviousLine,
	"ctrl+d":        ui.NextPage,
	"ctrl+u":        ui.PreviousPage,
	"g g":           ui.FirstLine,
	"G":             ui.LastLine,
	"] f":           ui.NextFile,
	"[ f":           ui.PreviousFile,
	"ctrl+w h":      ui.OldPane,
	"ctrl+w l":      ui.NewPane,
	"ctrl+w ctrl+w": ui.OtherPane,
}

var navigationAlignments = map[string]ui.Alignment{
	"z z": ui.Center,
	"z t": ui.Top,
	"z b": ui.Bottom,
}

type saveCommentFunc func(feedback.Comment, patches.Patch,

) (feedback.Comment, error)

type deleteCommentFunc func(feedback.Comment, patches.Patch) error

type loadCommentsFunc func() ([]feedback.Comment, error)

type refreshDiffFunc func(patches.Kind) (patches.Patch, error)

type saveSideBySideFunc func(bool) error

type commentStore interface {
	Add(feedback.Comment) (feedback.Comment, error)
	List(string) ([]feedback.Comment, error)
	Update(feedback.Comment) error
	Delete(string, string) error
}

type size struct {
	Width  int
	Height int
}

type initialLayout struct {
	SideBySide     bool
	SaveSideBySide saveSideBySideFunc
	size           size
}

type refreshDiffMsg struct {
	patch patches.Patch
	kind  patches.Kind
	err   error
}

type commentEditorFinishedMsg struct {
	body string
	err  error
}

type commentsLoadedMsg struct {
	comments []feedback.Comment
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

const horizontalScrollStep = 4

const verticalScrollStep = 3

var defaultSize = size{Width: 80, Height: 30}

type commentState struct {
	items    []feedback.Comment
	revision uint64
}

type commentEdit struct {
	body   string
	index  int
	anchor feedback.Anchor
}

type reviewModel struct {
	ctx context.Context

	diffView    *ui.Diff
	commentView *ui.Comments
	width       int
	height      int
	mode        mode
	keys        keymapMatcher

	currentPatch patches.Patch
	kind         patches.Kind
	diffOptions  ui.Options
	comments     commentState
	edit         commentEdit

	save       saveCommentFunc
	delete     deleteCommentFunc
	load       loadCommentsFunc
	refresh    refreshDiffFunc
	saveLayout saveSideBySideFunc
	err        error
	quitting   bool
}

func (reviewModel) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func renderAppHelp(width, height int) string {
	return ui.RenderHelp(helpBindings(), width, height)
}

func helpBindings() []ui.Binding {
	return []ui.Binding{
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

func (m reviewModel) updateOtherMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case commentEditorFinishedMsg:
		m.finishCommentEditor(msg)
	case commentsLoadedMsg:
		m.applyLoadedComments(msg)
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			return m, nil
		}
		return m, m.refreshDiffCommand()
	case tea.FocusMsg:
		return m, m.refreshDiffCommand()
	case refreshDiffMsg:
		m.applyRefreshedPatch(msg)
	default:
		return m, nil
	}
	return m, nil
}

func (m *reviewModel) finishCommentEditor(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.edit = commentEdit{index: -1}
		return
	}
	m.edit.body = msg.body
	body := strings.TrimSpace(m.edit.body)
	if body == "" {
		if m.edit.index >= 0 {
			*m = m.deleteCommentAt(m.edit.index)
		}
		m.edit = commentEdit{index: -1}
		m.diffView.ClearSelection()
		return
	}
	m.saveEditedComment(body)
}

func (m *reviewModel) saveEditedComment(body string) {
	if m.save == nil {
		m.err = errors.New("comment storage is unavailable")
		m.edit = commentEdit{index: -1}
		return
	}
	comment := m.editedComment(body)
	saved, err := m.save(comment, m.currentPatch)
	if err != nil {
		m.err = err
		m.edit = commentEdit{index: -1}
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
	m.edit = commentEdit{index: -1}
	m.err = nil
	m.diffView.ClearSelection()
}

func (m *reviewModel) editedComment(body string) feedback.Comment {
	if m.edit.index >= 0 {
		comment := m.comments.items[m.edit.index]
		comment.Body = body
		return comment
	}
	return feedback.Comment{Anchor: m.edit.anchor, Body: body}
}

func (m *reviewModel) applyLoadedComments(msg commentsLoadedMsg) {
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

func (m *reviewModel) applyRefreshedPatch(msg refreshDiffMsg) {
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

func (m reviewModel) refreshDiffCommand() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg {
		patch, err := m.refresh(kind)
		return refreshDiffMsg{patch: patch, kind: kind, err: err}
	}
}

func createReviewAppModel(
	currentPatch patches.Patch,
	dimensions size,
	pending []feedback.Comment,
	preferences settingsPreferences,
	store commentStore,

) reviewModel {
	layout := makeInitialLayout(preferences, dimensions)
	size := layout.size
	if size.Width <= 0 || size.Height <= 0 {
		size = defaultSize
	}
	m := reviewModel{
		ctx:          context.Background(),
		currentPatch: currentPatch,
		comments:     commentState{items: pending},
		commentView:  ui.NewComments(pending),
		edit:         commentEdit{index: -1},
		width:        size.Width,
		height:       size.Height,
		save:         saveReviewComment(store),
		saveLayout:   layout.SaveSideBySide,
		diffOptions: ui.Options{
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
	m.diffView = ui.NewDiff(currentPatch, m.diffOptions)
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
	return m
}
