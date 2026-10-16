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
	AddComment(comments.Comment) (comments.Comment, error)
	ListComments(string) ([]comments.Comment, error)
	UpdateComment(comments.Comment) error
	DeleteComment(string, string) error
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

func newReviewModel(
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
		commentView:  commentscreen.NewCommentList(items),
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
		keys: keymap.NewKeyMatcher(
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
	m.diffView = diffscreen.NewChangesView(p, m.diffOptions)
	m.resizeViews()
	return m
}

func newModelWithStore(
	store commentStore,
	p patch.Patch,
	items []comments.Comment,
	size size,
) (model, error) {
	preferences, err := settings.LoadPreferences()
	if err != nil {
		return model{}, err
	}
	m := newReviewModel(p, items, func(
		comment comments.Comment,
		current patch.Patch,
	) (comments.Comment, error) {
		comment.Repository = current.Root
		if comment.ID != "" {
			if err := store.UpdateComment(comment); err != nil {
				return comments.Comment{}, err
			}
			return comment, nil
		}
		return store.AddComment(comment)
	}, initialLayout{
		SideBySide: preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error {
			return settings.SavePreferences(
				settings.Preferences{SideBySide: enabled},
			)
		},
		size: size,
	})
	m.setDelete(func(comment comments.Comment, current patch.Patch) error {
		return store.DeleteComment(current.Root, comment.ID)
	})
	m.setLoadComments(func() ([]comments.Comment, error) {
		return store.ListComments(p.Root)
	})
	return m, nil
}

func (m *model) setRefresh(refresh refreshDiffFunc)    { m.refresh = refresh }
func (m *model) setDelete(remove deleteCommentFunc)    { m.delete = remove }
func (m *model) setLoadComments(load loadCommentsFunc) { m.load = load }

func (m *model) setSplitViewAndSaveChoice(
	enabled bool,
	save saveSideBySideFunc,
) {
	m.saveLayout = save
	m.setSplitView(enabled)
}

func (model) Init() tea.Cmd {
	return func() tea.Msg { return tea.RequestBackgroundColor() }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.diffOptions.Dark {
			m.diffOptions.Dark = dark
			m.diffView.SetOptions(m.diffOptions)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeViews()

	case tea.KeyPressMsg:
		m.diffView.EndDrag()
		return m.handleKeyPress(msg)
	case tea.MouseClickMsg, tea.MouseMotionMsg,
		tea.MouseReleaseMsg, tea.MouseWheelMsg:
		m.handleMouseMessage(msg)
	default:
		return m.handleWorkflowMessage(msg)
	}
	return m, nil
}

func (m model) handleWorkflowMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case commentEditorFinishedMsg:
		m.handleCommentEditorResult(msg)
	case commentsLoadedMsg:
		m.handleLoadedComments(msg)
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			break
		}
		return m, m.refreshChanges()
	case tea.FocusMsg:
		return m, m.refreshChanges()
	case refreshDiffMsg:
		m.handleRefreshedDiff(msg)
	default:
		return m, nil
	}
	return m, nil
}

func (m *model) handleCommentEditorResult(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.cancelCommentEdit()
		return
	}
	m.edit.body = msg.body
	m.saveEditedComment()
}

func (m *model) handleLoadedComments(msg commentsLoadedMsg) {
	if msg.revision != m.comments.revision {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh comments: %w", msg.err)
		return
	}
	m.comments.items = msg.comments
	m.commentView.ReplaceComments(m.comments.items)
	m.err = nil
}

func (m *model) handleRefreshedDiff(msg refreshDiffMsg) {
	if msg.kind != m.kind {
		return
	}
	if msg.err != nil {
		m.err = fmt.Errorf("refresh diff: %w", msg.err)
		return
	}
	m.showChangedFiles(msg.patch)
	m.err = nil
}

func (m model) refreshChanges() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg {
		p, err := m.refresh(kind)
		return refreshDiffMsg{patch: p, kind: kind, err: err}
	}
}

func (m model) loadSavedComments() tea.Cmd {
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

func (m *model) showChangedFiles(p patch.Patch) {
	m.currentPatch = p
	m.diffView.ReplaceChanges(p)
}
func (m *model) resizeViews() {
	m.diffView.Resize(m.width, m.height)
	m.commentView.Resize(m.width, m.height)
}

func (m model) handleKeyPress(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	name := key.String()
	if m.mode == modeComments {
		return m.showCommentsForView(name)
	}
	if m.mode == modeHelp {
		if name == "esc" || name == "?" || name == "q" {
			m.mode = modeBrowse
		}
		return m, nil
	}
	if m.mode == modeSearch {
		return m.updateSearchQuery(name, key)
	}
	return m.handleBrowseKey(name)
}

func (m model) handleBrowseKey(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	name = m.keys.MatchKeySequence(name)
	if m.navigateToView(name) {
		return m, nil
	}
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "c":
		return m.startCommentEditing()
	case "e":
		return m.openSelectedSource()
	case "R":
		return m, m.refreshChanges()
	case "tab":
		return m.showOtherChanges()
	default:
		return m.showBrowseView(name)
	}
}
func (m model) showBrowseView(name string) (tea.Model, tea.Cmd) {
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
		m.commentView.ReplaceComments(m.comments.items)
		return m, m.loadSavedComments()
	case "t":
		m.toggleSplitView()
	default:
		return m, nil
	}
	return m, nil
}

func (m model) startCommentEditing() (tea.Model, tea.Cmd) {
	cmd, err := m.startNewComment()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m model) openSelectedSource() (tea.Model, tea.Cmd) {
	cmd, err := m.openCurrentFileAtSelectedLine()
	if err != nil {
		m.err = err
		return m, nil
	}
	return m, cmd
}

func (m model) showOtherChanges() (tea.Model, tea.Cmd) {
	if m.currentPatch.Branch == "" {
		return m, nil
	}
	if m.kind == patch.Unstaged {
		m.kind = patch.Branch
	} else {
		m.kind = patch.Unstaged
	}
	m.diffView.ClearSelection()
	return m, m.refreshChanges()
}
