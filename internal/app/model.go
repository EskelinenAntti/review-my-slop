package app

import (
	"context"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/ui/commentscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
	"github.com/eskelinenantti/review-my-slop/internal/ui/keymap"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type saveCommentFunc func(
	comment comments.Comment,
) (comments.Comment, error)
type deleteCommentFunc func(repository, id string) error
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
	Width  int
	Height int
}

type initialLayout struct {
	SideBySide     bool
	SaveSideBySide saveSideBySideFunc
	size           size
}

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
	if size.Width <= 0 || size.Height <= 0 {
		size = defaultSize
	}
	m := model{
		ctx:          context.Background(),
		currentPatch: p,
		comments:     commentState{items: items},
		commentView:  commentscreen.New(items),
		edit:         commentEdit{index: -1},
		width:        size.Width,
		height:       size.Height,
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
	saveComment := func(
		comment comments.Comment,
	) (comments.Comment, error) {
		if comment.ID == "" {
			return store.Add(comment)
		}
		if err := store.Update(comment); err != nil {
			return comments.Comment{}, err
		}
		return comment, nil
	}
	m := newModel(p, items, saveComment, initialLayout{
		SideBySide: preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error {
			return settings.Save(settings.Preferences{SideBySide: enabled})
		},
		size: size,
	})
	m.delete = store.Delete
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
	return m.updateBrowseKey(name)
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
		return m.startCommentEditor()
	case "e":
		return m.startSourceEditor()
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

func (m model) startCommentEditor() (tea.Model, tea.Cmd) {
	cmd, err := m.beginComment()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m model) startSourceEditor() (tea.Model, tea.Cmd) {
	cmd, err := m.openCurrentLine()
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
