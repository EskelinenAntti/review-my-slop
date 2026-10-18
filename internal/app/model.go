package app

import (
	"context"
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type saveCommentFunc func(
	comments.Comment,
	patch.Patch,
) (comments.Comment, error)
type deleteCommentFunc func(comments.Comment, patch.Patch) error
type loadCommentsFunc func() ([]comments.Comment, error)
type refreshDiffFunc func(patch.Kind) (patch.Patch, error)
type saveSideBySideFunc func(bool) error

// commentStore supplies the persistence operations used by the terminal client.
type commentStore interface {
	Add(comments.Comment) (comments.Comment, error)
	List(string) ([]comments.Comment, error)
	Update(comments.Comment) error
	Delete(string, string) error
}

type size struct {
	width  int
	height int
}

type initialLayout struct {
	SideBySide     bool
	SaveSideBySide saveSideBySideFunc
	size           size
}

type refreshDiffMsg struct {
	patch patch.Patch
	kind  patch.Kind
	err   error
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

const horizontalScrollStep = 4
const verticalScrollStep = 3

var defaultSize = size{width: 80, height: 30}

// commentState pairs stored comments with the revision used to reject stale
// loads.
type commentState struct {
	items    []comments.Comment
	revision uint64
}

type commentEdit struct {
	body   string
	index  int
	anchor comments.Anchor
}

type model struct {
	ctx context.Context

	diffView    *diffscreen.View
	commentView *commentscreen.View
	width       int
	height      int
	mode        mode
	keys        keymap.Matcher

	currentPatch patch.Patch
	kind         patch.Kind
	diffOptions  diffscreen.Options
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

func newModel(
	p patch.Patch,
	items []comments.Comment,
	save saveCommentFunc,
	layout initialLayout,
) model {
	size := layout.size
	if size.width <= 0 || size.height <= 0 {
		size = defaultSize
	}
	m := model{
		ctx:          context.Background(),
		currentPatch: p,
		comments:     commentState{items: items},
		commentView:  commentscreen.New(items),
		edit:         commentEdit{index: -1},
		width:        size.width,
		height:       size.height,
		save:         save,
		saveLayout:   layout.SaveSideBySide,
		diffOptions: diffscreen.Options{
			SideBySide: layout.SideBySide,
			Dark:       true,
		},
		kind: p.Kind,
		keys: keymap.New(
			keymap.Sequence{
				Prefix:         "g",
				Keys:           []string{"g"},
				RetryUnmatched: true,
			},
			keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
			keymap.Sequence{Prefix: "[", Keys: []string{"f"}},
			keymap.Sequence{Prefix: "]", Keys: []string{"f"}},
			keymap.Sequence{
				Prefix: "ctrl+w",
				Keys:   []string{"h", "l", "ctrl+w"},
			},
		),
	}
	m.diffView = diffscreen.New(p, m.diffOptions)
	m.resizeScreens()
	return m
}

func newWithStore(
	store commentStore,
	p patch.Patch,
	items []comments.Comment,
	size size,
) (model, error) {
	preferences, err := settings.Load()
	if err != nil {
		return model{}, err
	}
	m := newModel(p, items, func(
		comment comments.Comment,
		current patch.Patch,
	) (comments.Comment, error) {
		comment.Repository = current.Root
		if comment.ID != "" {
			if err := store.Update(comment); err != nil {
				return comments.Comment{}, err
			}
			return comment, nil
		}
		return store.Add(comment)
	}, initialLayout{
		SideBySide: preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error {
			return settings.Save(settings.Preferences{SideBySide: enabled})
		},
		size: size,
	})
	m.delete = func(comment comments.Comment, current patch.Patch) error {
		return store.Delete(current.Root, comment.ID)
	}
	m.load = func() ([]comments.Comment, error) {
		return store.List(p.Root)
	}
	return m, nil
}

func (m *model) configureSideBySide(enabled bool, save saveSideBySideFunc) {
	m.saveLayout = save
	m.setSideBySide(enabled)
}

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

func (m model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	switch m.mode {
	case modeComments:
		return m.updateComments(name)
	case modeHelp:
		if name == "esc" || name == "?" || name == "q" {
			m.mode = modeBrowse
		}
		return m, nil
	case modeSearch:
		return m.updateSearch(name, key)
	default:
		return m.updateBrowseKey(name)
	}
}

func (m model) updateBrowseKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	name = m.keys.Feed(name)
	if m.navigate(name) {
		return m, nil
	}
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "c":
		return m.startEditor((*model).beginComment)
	case "e":
		return m.startEditor((*model).openCurrentLine)
	case "R":
		return m, m.loadRefresh()
	case "tab":
		return m.switchPatchKind()
	default:
		return m.updateBrowseView(name)
	}
}
func (m model) updateBrowseView(name string) (tea.Model, tea.Cmd) {
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
		m.mode = modeComments
		m.commentView.Update(m.comments.items)
		return m, m.loadComments()
	case "t":
		m.toggleSideBySide()
	default:
		return m, nil
	}
	return m, nil
}

func (m model) startEditor(
	start func(*model) (tea.Cmd, error),
) (tea.Model, tea.Cmd) {
	cmd, err := start(&m)
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m model) switchPatchKind() (tea.Model, tea.Cmd) {
	if m.currentPatch.Branch == "" {
		return m, nil
	}
	if m.kind == patch.Unstaged {
		m.kind = patch.Branch
	} else {
		m.kind = patch.Unstaged
	}
	m.diffView.ClearSelection()
	return m, m.loadRefresh()
}
