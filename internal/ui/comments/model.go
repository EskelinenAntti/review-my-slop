// Package comments implements the pending-comment screen and editor workflow.
package comments

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/editor"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Initial struct {
	Items         []comments.Comment
	Width, Height int
}

type Dependencies struct {
	List   func() ([]comments.Comment, error)
	Save   func(comments.Draft) (comments.Comment, error)
	Delete func(id string) error
}

type Show struct{}
type BackRequested struct{}
type QuitRequested struct{}
type Saved struct{ FromPatch bool }
type Cancelled struct{ FromPatch bool }

// Failed reports an editor or persistence failure for a composition started
// from Patch browsing. The shell forwards it to the Patch screen for display.
type Failed struct{ Err error }

type Model struct {
	items         []comments.Comment
	row           int
	width, height int
	deps          Dependencies
	err           error

	reloadGeneration   uint64
	mutationGeneration uint64
	editGeneration     uint64
	reservationID      uint64
	pending            map[string]uint64
	editReservation    uint64

	edit          *editor.Edit
	editID        string
	editAnchor    patch.Anchor
	editFromPatch bool
}

func New(initial Initial, dependencies Dependencies) *Model {
	width, height := initial.Width, initial.Height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 30
	}
	return &Model{items: append([]comments.Comment(nil), initial.Items...), width: width, height: height, deps: dependencies, pending: make(map[string]uint64)}
}

func (m *Model) Resize(width, height int) {
	if width > 0 {
		m.width = width
	}
	if height > 0 {
		m.height = height
	}
}

func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case Show:
		return m.reload()
	case loadFinished:
		if msg.generation != m.reloadGeneration || msg.mutation != m.mutationGeneration {
			return nil
		}
		if msg.err != nil {
			m.err = fmt.Errorf("refresh comments: %w", msg.err)
			return nil
		}
		selected := msg.selected
		m.items = append([]comments.Comment(nil), msg.items...)
		m.row = indexOf(m.items, selected)
		if m.row < 0 {
			m.row = 0
		}
		m.row = min(m.row, max(0, len(m.items)-1))
		m.err = nil
		return nil
	case editFinished:
		if msg.generation != m.editGeneration {
			return nil
		}
		m.edit = nil
		if msg.err != nil {
			m.release(msg.id, msg.reservation)
			m.clearEdit()
			if msg.fromPatch {
				return func() tea.Msg { return Failed{Err: msg.err} }
			}
			m.err = msg.err
			return nil
		}
		body := strings.TrimSpace(msg.body)
		if body == "" {
			if msg.id == "" {
				m.release(msg.id, msg.reservation)
				m.clearEdit()
				if msg.fromPatch {
					return func() tea.Msg { return Cancelled{FromPatch: true} }
				}
				return nil
			}
			m.clearEdit()
			return m.deleteReserved(msg.id, msg.fromPatch, msg.reservation)
		}
		if m.deps.Save == nil {
			m.release(msg.id, msg.reservation)
			m.clearEdit()
			err := fmt.Errorf("comment storage is unavailable")
			if msg.fromPatch {
				return func() tea.Msg { return Failed{Err: err} }
			}
			m.err = err
			return nil
		}
		draft := comments.Draft{ID: msg.id, Anchor: cloneAnchor(msg.anchor), Body: body}
		m.mutationGeneration++
		m.reloadGeneration++
		m.clearEdit()
		return func() tea.Msg {
			saved, err := m.deps.Save(draft)
			return saveFinished{saved: saved, id: draft.ID, reservation: msg.reservation, fromPatch: msg.fromPatch, err: err}
		}
	case saveFinished:
		m.release(msg.id, msg.reservation)
		if msg.err != nil {
			if msg.fromPatch {
				return func() tea.Msg { return Failed{Err: msg.err} }
			}
			m.err = msg.err
			return nil
		}
		if msg.id == "" {
			m.items = append(m.items, msg.saved)
			m.row = len(m.items) - 1
		} else if index := indexOf(m.items, msg.id); index >= 0 {
			m.items[index] = msg.saved
		} else {
			m.items = append(m.items, msg.saved)
			m.row = len(m.items) - 1
		}
		m.mutationGeneration++
		m.reloadGeneration++
		m.err = nil
		if msg.fromPatch {
			return func() tea.Msg { return Saved{FromPatch: true} }
		}
		return nil
	case deleteFinished:
		m.release(msg.id, msg.reservation)
		if msg.err != nil {
			if msg.fromPatch {
				return func() tea.Msg { return Failed{Err: msg.err} }
			}
			m.err = msg.err
			return nil
		}
		if index := indexOf(m.items, msg.id); index >= 0 {
			m.items = append(m.items[:index], m.items[index+1:]...)
			if m.row >= len(m.items) {
				m.row = max(0, len(m.items)-1)
			}
		}
		m.mutationGeneration++
		m.reloadGeneration++
		m.err = nil
		if msg.fromPatch {
			return func() tea.Msg { return Saved{FromPatch: true} }
		}
		return nil
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return nil
}

func (m *Model) Begin(anchor patch.Anchor) tea.Cmd {
	return m.begin("", cloneAnchor(anchor), "", true)
}

func (m *Model) reload() tea.Cmd {
	if m.deps.List == nil {
		m.err = fmt.Errorf("comment storage is unavailable")
		return nil
	}
	m.reloadGeneration++
	generation, mutation := m.reloadGeneration, m.mutationGeneration
	selected := ""
	if m.row >= 0 && m.row < len(m.items) {
		selected = m.items[m.row].ID
	}
	return func() tea.Msg {
		items, err := m.deps.List()
		return loadFinished{items: items, generation: generation, mutation: mutation, selected: selected, err: err}
	}
}

func (m *Model) updateKey(msg tea.KeyPressMsg) tea.Cmd {
	key := msg.String()
	switch key {
	case "ctrl+c":
		return func() tea.Msg { return QuitRequested{} }
	case "esc", "C", "q":
		return func() tea.Msg { return BackRequested{} }
	case "j", "down":
		if m.row < len(m.items)-1 {
			m.row++
		}
	case "k", "up":
		if m.row > 0 {
			m.row--
		}
	case "enter", "e":
		if len(m.items) > 0 {
			item := m.items[m.row]
			return m.begin(item.ID, cloneAnchor(item.Anchor), item.Body, false)
		}
	case "D":
		if len(m.items) > 0 {
			return m.delete(m.items[m.row].ID, false)
		}
	}
	return nil
}

func (m *Model) begin(id string, anchor patch.Anchor, body string, fromPatch bool) tea.Cmd {
	if m.edit != nil {
		err := fmt.Errorf("a comment editor is already open")
		if fromPatch {
			return func() tea.Msg { return Failed{Err: err} }
		}
		m.err = err
		return nil
	}
	reservation, ok := m.reserve(id)
	if !ok {
		err := fmt.Errorf("comment edit is already pending")
		if fromPatch {
			return func() tea.Msg { return Failed{Err: err} }
		}
		m.err = err
		return nil
	}
	command := strings.TrimSpace(os.Getenv("EDITOR"))
	if command == "" {
		m.release(id, reservation)
		err := fmt.Errorf("$EDITOR is not set")
		if fromPatch {
			return func() tea.Msg { return Failed{Err: err} }
		}
		m.err = err
		return nil
	}
	draft, err := editor.Prepare(command, commentDraft(body, anchor))
	if err != nil {
		m.release(id, reservation)
		if fromPatch {
			return func() tea.Msg { return Failed{Err: err} }
		}
		m.err = err
		return nil
	}
	m.edit = draft
	m.editID, m.editAnchor, m.editFromPatch = id, cloneAnchor(anchor), fromPatch
	m.editReservation = reservation
	m.editGeneration++
	generation := m.editGeneration
	return tea.ExecProcess(draft.Command(), func(processErr error) tea.Msg {
		text, err := draft.Finish(processErr)
		if err == nil {
			text = stripUnchangedSuggestion(text, anchor.QuotedLines)
		}
		return editFinished{body: text, id: id, anchor: cloneAnchor(anchor), fromPatch: fromPatch, generation: generation, reservation: reservation, err: err}
	})
}

func (m *Model) delete(id string, fromPatch bool) tea.Cmd {
	reservation, ok := m.reserve(id)
	if !ok {
		err := fmt.Errorf("comment edit is already pending")
		if fromPatch {
			return func() tea.Msg { return Failed{Err: err} }
		}
		m.err = err
		return nil
	}
	return m.deleteReserved(id, fromPatch, reservation)
}

func (m *Model) deleteReserved(id string, fromPatch bool, reservation uint64) tea.Cmd {
	if m.deps.Delete == nil {
		m.release(id, reservation)
		err := fmt.Errorf("comment storage is unavailable")
		if fromPatch {
			return func() tea.Msg { return Failed{Err: err} }
		}
		m.err = err
		return nil
	}
	m.mutationGeneration++
	m.reloadGeneration++
	return func() tea.Msg {
		return deleteFinished{id: id, reservation: reservation, fromPatch: fromPatch, err: m.deps.Delete(id)}
	}
}

func (m *Model) cancelEdit() {
	if m.edit != nil {
		_ = m.edit.Close()
		m.release(m.editID, m.editReservation)
	}
	m.edit = nil
	m.editGeneration++
}

func (m *Model) reserve(id string) (uint64, bool) {
	if m.pending == nil {
		m.pending = make(map[string]uint64)
	}
	key := id
	if key == "" {
		key = "\x00new"
	}
	if _, exists := m.pending[key]; exists {
		return 0, false
	}
	m.reservationID++
	m.pending[key] = m.reservationID
	return m.reservationID, true
}

func (m *Model) release(id string, reservation uint64) {
	key := id
	if key == "" {
		key = "\x00new"
	}
	if m.pending[key] == reservation {
		delete(m.pending, key)
	}
}

func (m *Model) clearEdit() {
	m.editID = ""
	m.editAnchor = patch.Anchor{}
	m.editFromPatch = false
}

type loadFinished struct {
	items                []comments.Comment
	generation, mutation uint64
	selected             string
	err                  error
}
type editFinished struct {
	body, id    string
	anchor      patch.Anchor
	fromPatch   bool
	generation  uint64
	reservation uint64
	err         error
}
type saveFinished struct {
	saved       comments.Comment
	id          string
	reservation uint64
	fromPatch   bool
	err         error
}
type deleteFinished struct {
	id          string
	reservation uint64
	fromPatch   bool
	err         error
}

func indexOf(items []comments.Comment, id string) int {
	if id == "" {
		return -1
	}
	for index := range items {
		if items[index].ID == id {
			return index
		}
	}
	return -1
}

func cloneAnchor(anchor patch.Anchor) patch.Anchor {
	anchor.QuotedLines = append([]string(nil), anchor.QuotedLines...)
	return anchor
}
