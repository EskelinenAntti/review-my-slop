package app

import (
	"context"
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/settings"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
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
type saveSideBySideFunc func(bool) error

// commentStore supplies the persistence operations used by the terminal client.
type commentStore interface {
	Add(comments.Comment) (comments.Comment, error)
	List(string) ([]comments.Comment, error)
	Update(comments.Comment) error
	Delete(string, string) error
}

type terminalDimensions struct {
	width  int
	height int
}

type initialLayout struct {
	sideBySide     bool
	saveSideBySide saveSideBySideFunc
	dimensions     terminalDimensions
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

var defaultDimensions = terminalDimensions{
	width:  ui.DefaultScreenWidth,
	height: ui.DefaultScreenHeight,
}

// commentState pairs stored comments with the revision used to reject stale
// loads.
type commentState struct {
	items    []comments.Comment
	revision uint64
}

type commentEdit struct {
	index  int
	anchor comments.Anchor
}

type model struct {
	ctx context.Context

	diffView    *diffscreen.View
	commentView *commentscreen.View
	dimensions  terminalDimensions
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
	ctx context.Context,
	currentPatch patch.Patch,
	pendingComments []comments.Comment,
	save saveCommentFunc,
	layout initialLayout,
) model {
	dimensions := layout.dimensions
	if dimensions.width <= 0 || dimensions.height <= 0 {
		dimensions = defaultDimensions
	}
	m := model{
		ctx:          ctx,
		currentPatch: currentPatch,
		comments:     commentState{items: pendingComments},
		commentView:  commentscreen.New(pendingComments),
		edit:         commentEdit{index: -1},
		dimensions:   dimensions,
		save:         save,
		saveLayout:   layout.saveSideBySide,
		diffOptions: diffscreen.Options{
			SideBySide: layout.sideBySide,
			Dark:       true,
		},
		kind: currentPatch.Kind,
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
	m.diffView = diffscreen.New(currentPatch, m.diffOptions)
	m.resizeScreens()
	return m
}

func newWithStore(
	ctx context.Context,
	store commentStore,
	currentPatch patch.Patch,
	pendingComments []comments.Comment,
	dimensions terminalDimensions,
) (model, error) {
	preferences, err := settings.Load()
	if err != nil {
		return model{}, err
	}
	m := newModel(ctx, currentPatch, pendingComments, func(
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
		sideBySide: preferences.SideBySide,
		saveSideBySide: func(enabled bool) error {
			return settings.Save(settings.Preferences{SideBySide: enabled})
		},
		dimensions: dimensions,
	})
	m.delete = func(comment comments.Comment, current patch.Patch) error {
		return store.Delete(current.Root, comment.ID)
	}
	m.load = func() ([]comments.Comment, error) {
		return store.List(currentPatch.Root)
	}
	return m, nil
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
		m.dimensions = terminalDimensions{width: msg.Width, height: msg.Height}
		m.resizeScreens()

	case tea.KeyPressMsg:
		m.diffView.EndDrag()
		return m.updateKey(msg)
	case tea.MouseClickMsg:
		m.click(msg)
	case tea.MouseMotionMsg:
		m.drag(msg)
	case tea.MouseReleaseMsg:
		m.release(msg)
	case tea.MouseWheelMsg:
		m.wheel(msg)
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
		if msg.err == nil {
			return m, m.loadRefresh()
		}
		m.err = fmt.Errorf("editor: %w", msg.err)
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		m.finishDiffRefresh(msg)
	default:
		return m, nil
	}
	return m, nil
}

func (m *model) resizeScreens() {
	m.diffView.Resize(m.dimensions.width, m.dimensions.height)
	m.commentView.Resize(m.dimensions.width, m.dimensions.height)
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
		return m.startEditor(m.beginComment)
	case "e":
		return m.startEditor(m.openCurrentLine)
	case "R":
		return m, m.loadRefresh()
	case "tab":
		return m.switchPatchKind()
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

func (m *model) startEditor(
	start func() (tea.Cmd, error),
) (tea.Model, tea.Cmd) {
	cmd, err := start()
	if err != nil {
		m.err = err
		return *m, nil
	}
	return *m, cmd
}
