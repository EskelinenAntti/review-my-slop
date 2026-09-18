package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/view"
)

type searchState struct {
	query []rune
	term  string
	from  view.Cursor
	miss  bool
}

func (m *Model) cancelSelection() { m.review.selection = nil }

func (m *Model) move(direction view.Direction) {
	next, ok := m.review.view.Move(m.review.cursor, direction)
	if !ok {
		return
	}
	if !m.extendSelection(next) {
		return
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor view.Cursor) {
	m.review.cursor = cursor
	m.review.viewport = m.review.view.KeepVisible(m.review.viewport, cursor)
}

func (m *Model) halfPage(direction view.Direction) {
	viewport, cursor := m.review.view.ScrollHalfPage(m.review.viewport, m.review.cursor, direction)
	if !m.extendSelection(cursor) {
		return
	}
	m.review.viewport, m.review.cursor = viewport, cursor
}

func (m *Model) extendSelection(cursor view.Cursor) bool {
	if m.review.selection == nil {
		return true
	}
	selection, ok := m.review.view.ExtendSelection(*m.review.selection, cursor)
	if !ok {
		return false
	}
	m.review.selection = &selection
	return true
}

func (m *Model) jumpFile(direction view.Direction) {
	m.cancelSelection()
	if cursor, ok := m.review.view.JumpFile(m.review.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane view.Pane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.review.view.SwitchPane(m.review.cursor, pane)
	if !ok {
		return
	}
	if m.review.selection != nil {
		selection, ok := m.switchSelectionPane(*m.review.selection, pane)
		if !ok {
			return
		}
		m.review.selection = selection
	}
	m.setCursor(cursor)
}

func (m Model) switchSelectionPane(selection view.Selection, pane view.Pane) (*view.Selection, bool) {
	first, firstFound := m.review.view.SwitchPane(selection.First, pane)
	last, lastFound := m.review.view.SwitchPane(selection.Last, pane)
	if !firstFound || !lastFound {
		return nil, false
	}
	switched := m.review.view.BeginSelection(first)
	switched, ok := m.review.view.ExtendSelection(switched, last)
	if !ok {
		return nil, false
	}
	return &switched, true
}

func (m Model) sideBySideActive() bool {
	return m.sideBySide && m.width >= minimumSideBySideWidth
}

func (m *Model) toggleSideBySide() {
	enabled := !m.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	m.saveSideBySidePreference()
}

func (m *Model) saveSideBySidePreference() {
	if m.saveSideBySide != nil {
		if err := m.saveSideBySide(m.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *Model) setSideBySide(enabled bool) {
	wasActive := m.sideBySideActive()
	m.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.review.patch)
	}
}

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.cancelSearch()
	case "enter":
		m.finishSearch()
	case "backspace":
		if len(m.search.query) > 0 {
			m.search.query = m.search.query[:len(m.search.query)-1]
		}
		m.updateIncrementalSearch()
	default:
		if key.Text != "" {
			m.search.query = append(m.search.query, []rune(key.Text)...)
			m.updateIncrementalSearch()
		}
	}
	return m, nil
}

func (m *Model) cancelSearch() {
	m.setCursor(m.search.from)
	m.mode = modeBrowse
	m.search.query = nil
	m.search.miss = false
}

func (m *Model) finishSearch() {
	if len(m.search.query) > 0 && !m.search.miss {
		m.search.term = string(m.search.query)
	}
	m.mode = modeBrowse
	m.search.query = nil
	m.search.miss = false
}

func (m *Model) updateIncrementalSearch() {
	if len(m.search.query) == 0 {
		m.setCursor(m.search.from)
		m.search.miss = false
		return
	}
	match, ok := m.review.view.Search(string(m.search.query), m.search.from, view.Forward)
	m.search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction view.Direction) {
	if m.search.term == "" {
		return
	}
	match, ok := m.review.view.Search(m.search.term, m.review.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
		return
	}
	m.setCursor(match)
}
