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

type saveCommentFunc func(comments.Comment, patch.Patch) (comments.Comment, error)
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

// commentState pairs stored comments with the revision used to reject stale loads.
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

func newModel(p patch.Patch, comments []comments.Comment, save saveCommentFunc, layout initialLayout) model {
	size := layout.size
	if size.Width <= 0 || size.Height <= 0 {
		size = defaultSize
	}
	m := model{
		ctx:          context.Background(),
		currentPatch: p,
		comments:     commentState{items: comments},
		commentView:  commentscreen.New(comments),
		edit:         commentEdit{index: -1},
		width:        size.Width,
		height:       size.Height,
		save:         save,
		saveLayout:   layout.SaveSideBySide,
		diffOptions:  diffscreen.Options{SideBySide: layout.SideBySide, Dark: true},
		kind:         p.Kind,
		keys: keymap.New(
			keymap.Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true},
			keymap.Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
			keymap.Sequence{Prefix: "[", Keys: []string{"f"}},
			keymap.Sequence{Prefix: "]", Keys: []string{"f"}},
			keymap.Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}},
		),
	}
	m.diffView = diffscreen.New(p, m.diffOptions)
	m.resizeScreens()
	return m
}

func newWithStore(store commentStore, p patch.Patch, items []comments.Comment, size size) (model, error) {
	preferences, err := settings.Load()
	if err != nil {
		return model{}, err
	}
	m := newModel(p, items, func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
		comment.Repository = current.Root
		if comment.ID != "" {
			if err := store.Update(comment); err != nil {
				return comments.Comment{}, err
			}
			return comment, nil
		}
		return store.Add(comment)
	}, initialLayout{
		SideBySide:     preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error { return settings.Save(settings.Preferences{SideBySide: enabled}) },
		size:           size,
	})
	m.setDelete(func(comment comments.Comment, current patch.Patch) error {
		return store.Delete(current.Root, comment.ID)
	})
	m.setLoadComments(func() ([]comments.Comment, error) {
		return store.List(p.Root)
	})
	return m, nil
}

func (m *model) setRefresh(refresh refreshDiffFunc)    { m.refresh = refresh }
func (m *model) setDelete(delete deleteCommentFunc)    { m.delete = delete }
func (m *model) setLoadComments(load loadCommentsFunc) { m.load = load }
func (m *model) configureSideBySide(enabled bool, save saveSideBySideFunc) {
	m.saveLayout = save
	m.setSideBySide(enabled)
}

func (m model) Init() tea.Cmd { return func() tea.Msg { return tea.RequestBackgroundColor() } }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		m.diffView.EndDrag()
		return m.updateKey(msg)
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case sourceEditorFinishedMsg:
		return m.sourceEditorFinished(msg)
	default:
		m.updateState(msg)
	}
	return m, nil
}

func (m *model) updateState(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.updateBackground(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeScreens()
	default:
		m.updateEditorState(msg)
	}
}

func (m *model) updateEditorState(msg tea.Msg) {
	switch msg := msg.(type) {
	case commentEditorFinishedMsg:
		m.commentEditorFinished(msg)
	case commentsLoadedMsg:
		m.commentsLoaded(msg)
	default:
		m.updatePresentation(msg)
	}
}

func (m *model) updatePresentation(msg tea.Msg) {
	switch msg := msg.(type) {
	case refreshDiffMsg:
		m.diffRefreshed(msg)
	case tea.MouseClickMsg:
		m.mouseClick(msg)
	default:
		m.updateMouseState(msg)
	}
}

func (m *model) updateMouseState(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		m.mouseMotion(msg)
	case tea.MouseReleaseMsg:
		m.mouseRelease(msg)
	default:
		if wheel, ok := msg.(tea.MouseWheelMsg); ok {
			m.mouseWheel(wheel)
		}
	}
}

func (m *model) updateBackground(msg tea.BackgroundColorMsg) {
	if dark := msg.IsDark(); dark != m.diffOptions.Dark {
		m.diffOptions.Dark = dark
		m.diffView.Configure(m.diffOptions)
	}
}

func (m *model) commentEditorFinished(msg commentEditorFinishedMsg) {
	if msg.err != nil {
		m.err = msg.err
		m.clearCommentEdit()
		return
	}
	m.edit.body = msg.body
	m.finishCommentEdit()
}

func (m *model) commentsLoaded(msg commentsLoadedMsg) {
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

func (m model) sourceEditorFinished(msg sourceEditorFinishedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = fmt.Errorf("editor: %w", msg.err)
		return m, nil
	}
	return m, m.loadRefresh()
}

func (m *model) diffRefreshed(msg refreshDiffMsg) {
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

func (m *model) mouseClick(msg tea.MouseClickMsg) {
	if msg.Button != tea.MouseLeft {
		return
	}
	switch m.mode {
	case modeBrowse:
		m.diffView.BeginDrag(msg.X, msg.Y)
	case modeComments:
		m.commentView.Click(msg.X, msg.Y)
	}
}

func (m *model) mouseMotion(msg tea.MouseMotionMsg) {
	if msg.Button == tea.MouseLeft && m.mode == modeBrowse {
		m.diffView.DragTo(msg.X, msg.Y)
	}
}

func (m *model) mouseRelease(msg tea.MouseReleaseMsg) {
	if msg.Button != tea.MouseLeft && msg.Button != tea.MouseNone {
		return
	}
	if m.mode == modeBrowse {
		m.diffView.DragTo(msg.X, msg.Y)
	}
	m.diffView.EndDrag()
}

func (m *model) mouseWheel(msg tea.MouseWheelMsg) {
	m.diffView.EndDrag()
	if m.mode != modeBrowse && m.mode != modeSearch {
		return
	}
	m.scrollWheel(msg.Button)
}

func (m *model) scrollWheel(button tea.MouseButton) {
	switch button {
	case tea.MouseWheelUp:
		m.diffView.ScrollVertical(-verticalScrollStep)
	case tea.MouseWheelDown:
		m.diffView.ScrollVertical(verticalScrollStep)
	case tea.MouseWheelLeft:
		m.diffView.ScrollHorizontal(-horizontalScrollStep)
	case tea.MouseWheelRight:
		m.diffView.ScrollHorizontal(horizontalScrollStep)
	}
}

func (m model) loadRefresh() tea.Cmd {
	if m.refresh == nil {
		return nil
	}
	kind := m.kind
	return func() tea.Msg { p, err := m.refresh(kind); return refreshDiffMsg{patch: p, kind: kind, err: err} }
}

func (m model) loadComments() tea.Cmd {
	if m.load == nil {
		return nil
	}
	revision := m.comments.revision
	return func() tea.Msg {
		comments, err := m.load()
		return commentsLoadedMsg{comments: comments, revision: revision, err: err}
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
		return m.updateHelp(name)
	case modeSearch:
		return m.updateSearch(name, key)
	}
	return m.updateBrowse(m.keys.Feed(name))
}

func (m model) updateHelp(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "esc", "?", "q":
		m.mode = modeBrowse
	}
	return m, nil
}

var browseMotions = map[string]diffscreen.Motion{
	"j": diffscreen.NextLine, "down": diffscreen.NextLine,
	"k": diffscreen.PreviousLine, "up": diffscreen.PreviousLine,
	"ctrl+d": diffscreen.NextPage, "ctrl+u": diffscreen.PreviousPage,
	"g g": diffscreen.FirstLine, "G": diffscreen.LastLine,
	"] f": diffscreen.NextFile, "[ f": diffscreen.PreviousFile,
	"ctrl+w h": diffscreen.OldPane, "ctrl+w l": diffscreen.NewPane,
	"ctrl+w ctrl+w": diffscreen.OtherPane,
}
var browseAlignments = map[string]diffscreen.Alignment{
	"z z": diffscreen.Center, "z t": diffscreen.Top, "z b": diffscreen.Bottom,
}
var browseScrolls = map[string]int{
	"h": -horizontalScrollStep, "left": -horizontalScrollStep,
	"l": horizontalScrollStep, "right": horizontalScrollStep,
	"0": -int(^uint(0) >> 1), "$": int(^uint(0) >> 1),
}

func (m model) updateBrowse(name string) (tea.Model, tea.Cmd) {
	m.err = nil
	if motion, ok := browseMotions[name]; ok {
		m.diffView.Move(motion)
		return m, nil
	}
	if alignment, ok := browseAlignments[name]; ok {
		m.diffView.Align(alignment)
		return m, nil
	}
	if scroll, ok := browseScrolls[name]; ok {
		m.diffView.ScrollHorizontal(scroll)
		return m, nil
	}
	return m.browseAction(name)
}

func (m model) browseAction(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "C":
		m.mode = modeComments
		m.commentView.Update(m.comments.items)
		return m, m.loadComments()
	default:
		return m.browseSearch(name)
	}
	return m, nil
}

func (m model) browseSearch(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "/":
		m.mode = modeSearch
		m.diffView.BeginSearch()
	case "n":
		m.diffView.Find(diffscreen.Forward)
	case "N":
		m.diffView.Find(diffscreen.Backward)
	default:
		return m.browseSelection(name)
	}
	return m, nil
}

func (m model) browseSelection(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "v":
		m.diffView.ToggleSelection()
	case "esc":
		m.diffView.ClearSelection()
	default:
		return m.browseEdit(name)
	}
	return m, nil
}

func (m model) browseEdit(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "c":
		cmd, err := m.beginComment()
		m.err = err
		return m, cmd
	case "e":
		cmd, err := m.openCurrentLine()
		m.err = err
		return m, cmd
	default:
		return m.browseRefresh(name)
	}
}

func (m model) browseRefresh(name string) (tea.Model, tea.Cmd) {
	switch name {
	case "R":
		return m, m.loadRefresh()
	case "tab":
		return m.switchPatch()
	case "t":
		m.toggleSideBySide()
	}
	return m, nil
}

func (m model) switchPatch() (tea.Model, tea.Cmd) {
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
