package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) move(direction Direction) {
	next, ok := m.review.present.Move(m.review.cursor, direction)
	if !ok {
		return
	}
	if m.review.selection != nil {
		selection, selectionOK := m.review.present.ExtendSelection(*m.review.selection, next)
		if !selectionOK {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(next)
}

func (m *Model) setCursor(cursor Cursor) {
	m.review.cursor = cursor
	m.review.viewport = m.review.present.KeepVisible(m.review.viewport, cursor)
}

func (m *Model) halfPage(direction Direction) {
	viewport, cursor := m.review.present.ScrollHalfPage(m.review.viewport, m.review.cursor, direction)
	if m.review.selection != nil {
		selection, ok := m.review.present.ExtendSelection(*m.review.selection, cursor)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.review.viewport, m.review.cursor = viewport, cursor
}

func (m *Model) jumpFile(direction Direction) {
	m.cancelSelection()
	if cursor, ok := m.review.present.JumpFile(m.review.cursor, direction); ok {
		m.setCursor(cursor)
	}
}

func (m *Model) switchPane(pane Pane) {
	if !m.sideBySideActive() {
		return
	}
	cursor, ok := m.review.present.SwitchPane(m.review.cursor, pane)
	if !ok {
		return
	}
	if m.review.selection != nil {
		first, firstOK := m.review.present.SwitchPane(m.review.selection.First, pane)
		last, lastOK := m.review.present.SwitchPane(m.review.selection.Last, pane)
		if !firstOK || !lastOK {
			return
		}
		selection := m.review.present.BeginSelection(first)
		selection, ok = m.review.present.ExtendSelection(selection, last)
		if !ok {
			return
		}
		m.review.selection = &selection
	}
	m.setCursor(cursor)
}

func (m *Model) toggleSideBySide() {
	enabled := !m.review.sideBySide
	if enabled && m.width < minimumSideBySideWidth {
		m.err = fmt.Errorf("side-by-side view requires a terminal at least %d columns wide", minimumSideBySideWidth)
		return
	}
	m.setSideBySide(enabled)
	if m.deps.SaveSideBySide != nil {
		if err := m.deps.SaveSideBySide(m.review.sideBySide); err != nil {
			m.err = fmt.Errorf("save side-by-side preference: %w", err)
		}
	}
}

func (m *Model) setSideBySide(enabled bool) {
	wasActive := m.sideBySideActive()
	m.review.sideBySide = enabled
	if wasActive != m.sideBySideActive() {
		m.rebuildView(m.review.changes)
	}
}

func (m Model) updateKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	pending := m.pendingKey
	m.pendingKey = ""
	if pending == "[" || pending == "]" {
		if pending+name == "]f" {
			m.jumpFile(Forward)
		}
		if pending+name == "[f" {
			m.jumpFile(Backward)
		}
		return m, nil
	}
	if pending == "z" {
		switch name {
		case "z":
			m.review.viewport = m.review.present.Align(m.review.viewport, m.review.cursor, Middle)
		case "t":
			m.review.viewport = m.review.present.Align(m.review.viewport, m.review.cursor, Top)
		case "b":
			m.review.viewport = m.review.present.Align(m.review.viewport, m.review.cursor, Bottom)
		}
		return m, nil
	}
	if pending == "ctrl+w" {
		switch name {
		case "h":
			m.switchPane(Left)
		case "l":
			m.switchPane(Right)
		case "ctrl+w":
			m.switchPane(m.review.cursor.Pane.Other())
		}
		return m, nil
	}
	switch name {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = modeHelp
	case "/":
		m.cancelSelection()
		m.mode = modeSearch
		m.search.query = nil
		m.search.from = m.review.cursor
		m.search.miss = false
	case "n":
		m.repeatSearch(Forward)
	case "N":
		m.repeatSearch(Backward)
	case "j", "down":
		m.move(Forward)
	case "k", "up":
		m.move(Backward)
	case "h", "left":
		m.review.viewport = m.review.present.ScrollHorizontal(m.review.viewport, -horizontalScrollStep)
	case "l", "right":
		m.review.viewport = m.review.present.ScrollHorizontal(m.review.viewport, horizontalScrollStep)
	case "0":
		m.review.viewport.LeftColumn = 0
	case "$":
		m.review.viewport = m.review.present.ScrollHorizontal(m.review.viewport, int(^uint(0)>>1))
	case "ctrl+d":
		m.halfPage(Forward)
	case "ctrl+u":
		m.halfPage(Backward)
	case "ctrl+w":
		m.pendingKey = name
	case "g":
		if pending == "g" {
			if cursor, ok := m.review.present.First(); ok {
				m.setCursor(cursor)
			}
		} else {
			m.pendingKey = "g"
		}
	case "G":
		if cursor, ok := m.review.present.Last(); ok {
			m.setCursor(cursor)
		}
	case "z":
		m.pendingKey = "z"
	case "]", "[":
		m.pendingKey = name
	case "v":
		if m.review.selection == nil {
			selection := m.review.present.BeginSelection(m.review.cursor)
			m.review.selection = &selection
		} else {
			m.cancelSelection()
		}
	case "esc":
		m.cancelSelection()
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
		m.comments.row = min(m.comments.row, max(0, len(m.comments.items)-1))
		return m, m.loadComments()
	case "R":
		return m, m.loadRefresh()
	case "tab":
		if m.defaultBranch == "" {
			return m, nil
		}
		if m.viewMode == LocalChanges {
			m.viewMode = BranchChanges
		} else {
			m.viewMode = LocalChanges
		}
		m.cancelSelection()
		return m, m.loadRefresh()
	case "t":
		m.toggleSideBySide()
	}
	return m, nil
}

func (m Model) updateSearch(name string, key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch name {
	case "esc":
		m.setCursor(m.search.from)
		m.mode = modeBrowse
		m.search.query = nil
		m.search.miss = false
	case "enter":
		if len(m.search.query) > 0 && !m.search.miss {
			m.search.term = string(m.search.query)
		}
		m.mode = modeBrowse
		m.search.query = nil
		m.search.miss = false
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

func (m *Model) updateIncrementalSearch() {
	if len(m.search.query) == 0 {
		m.setCursor(m.search.from)
		m.search.miss = false
		return
	}
	match, ok := m.review.present.Search(string(m.search.query), m.search.from, Forward)
	m.search.miss = !ok
	if ok {
		m.setCursor(match)
	}
}

func (m *Model) repeatSearch(direction Direction) {
	if m.search.term == "" {
		return
	}
	match, ok := m.review.present.Search(m.search.term, m.review.cursor, direction)
	if !ok {
		m.err = fmt.Errorf("no matches for %q", m.search.term)
		return
	}
	m.setCursor(match)
}
