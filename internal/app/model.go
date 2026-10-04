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

	diff        *diffscreen.View
	commentList *commentscreen.View
	width       int
	height      int
	mode        mode
	keys        keymap.Matcher

	currentPatch patch.Patch
	kind         patch.Kind
	diffOptions  diffscreen.Options
	comments     commentState
	edit         commentEdit
	searchQuery  []rune

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
		commentList:  commentscreen.New(comments),
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
	m.diff = diffscreen.New(p, m.diffOptions)
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
	case tea.BackgroundColorMsg:
		if dark := msg.IsDark(); dark != m.diffOptions.Dark {
			m.diffOptions.Dark = dark
			m.diff.Configure(m.diffOptions)
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeScreens()
	case commentEditorFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.clearCommentEdit()
		} else {
			m.edit.body = msg.body
			m.finishCommentEdit()
		}
	case commentsLoadedMsg:
		if msg.revision != m.comments.revision {
			break
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh comments: %w", msg.err)
		} else {
			m.comments.items = msg.comments
			m.commentList.Update(m.comments.items)
			m.err = nil
		}
	case sourceEditorFinishedMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("editor: %w", msg.err)
			break
		}
		return m, m.loadRefresh()
	case tea.FocusMsg:
		return m, m.loadRefresh()
	case refreshDiffMsg:
		if msg.kind != m.kind {
			return m, nil
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh diff: %w", msg.err)
		} else {
			m.updatePatch(msg.patch)
			m.err = nil
		}
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
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
	m.diff.Update(p)
}
func (m *model) resizeScreens() {
	m.diff.Resize(m.width, m.height)
	m.commentList.Resize(m.width, m.height)
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
	m.err = nil
	name = m.keys.Feed(name)
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		m.diff.ClearSelection()
		m.mode = modeSearch
		m.searchQuery = nil
		m.diff.BeginSearch()
	case "n":
		m.diff.Find(diffscreen.Forward)
	case "N":
		m.diff.Find(diffscreen.Backward)
	case "j", "down":
		m.diff.Move(diffscreen.NextLine)
	case "k", "up":
		m.diff.Move(diffscreen.PreviousLine)
	case "h", "left":
		m.diff.ScrollHorizontal(-horizontalScrollStep)
	case "l", "right":
		m.diff.ScrollHorizontal(horizontalScrollStep)
	case "0":
		m.diff.ScrollHorizontal(-int(^uint(0) >> 1))
	case "$":
		m.diff.ScrollHorizontal(int(^uint(0) >> 1))
	case "ctrl+d":
		m.diff.Move(diffscreen.NextPage)
	case "ctrl+u":
		m.diff.Move(diffscreen.PreviousPage)
	case "g g":
		m.diff.Move(diffscreen.FirstLine)
	case "G":
		m.diff.Move(diffscreen.LastLine)
	case "] f":
		m.diff.Move(diffscreen.NextFile)
	case "[ f":
		m.diff.Move(diffscreen.PreviousFile)
	case "z z":
		m.diff.Align(diffscreen.Center)
	case "z t":
		m.diff.Align(diffscreen.Top)
	case "z b":
		m.diff.Align(diffscreen.Bottom)
	case "ctrl+w h":
		m.diff.Move(diffscreen.OldPane)
	case "ctrl+w l":
		m.diff.Move(diffscreen.NewPane)
	case "ctrl+w ctrl+w":
		m.diff.Move(diffscreen.OtherPane)
	case "v":
		m.diff.ToggleSelection()
	case "esc":
		m.diff.ClearSelection()
	case "c":
		cmd, err := m.beginComment()
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, cmd
	case "e":
		cmd, err := m.openCurrentLine()
		if err != nil {
			m.err = err
			return m, nil
		}
		return m, cmd
	case "C":
		m.mode = modeComments
		m.commentList.Update(m.comments.items)
		return m, m.loadComments()
	case "R":
		return m, m.loadRefresh()
	case "tab":
		if m.currentPatch.Branch == "" {
			return m, nil
		}
		if m.kind == patch.Unstaged {
			m.kind = patch.Branch
		} else {
			m.kind = patch.Unstaged
		}
		m.diff.ClearSelection()
		return m, m.loadRefresh()
	case "t":
		m.toggleSideBySide()
	}
	return m, nil
}
