package tui

import (
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/view"
)

type reviewState struct {
	patch     patch.Patch
	view      view.View
	cursor    view.Cursor
	viewport  view.Viewport
	selection *view.Selection
}

type cursorIdentity struct {
	file   patch.File
	hunk   patch.Hunk
	line   patch.Line
	cursor view.Cursor
	valid  bool
}

func (m Model) identify(cursor view.Cursor) cursorIdentity {
	file, fileFound := m.review.view.File(cursor)
	hunk, hunkFound := m.review.view.Hunk(cursor)
	line, lineFound := m.review.view.Line(cursor)
	return cursorIdentity{
		file:   file,
		hunk:   hunk,
		line:   line,
		cursor: cursor,
		valid:  fileFound && hunkFound && lineFound,
	}
}

func (m *Model) rebuildView(nextPatch patch.Patch) {
	cursor := m.identify(m.review.cursor)
	first, last := m.identifySelection()
	rowsAbove := m.review.cursor.Coordinate.Y - m.review.viewport.Top.Y

	m.review.patch = nextPatch
	m.review.view = m.newReviewView(nextPatch)
	m.review.viewport = m.review.view.NewViewport(m.width, m.bodyHeight())
	m.review.cursor = m.restoreCursor(cursor)
	m.review.selection = m.restoreSelection(first, last)
	m.review.viewport.Top.Y = max(0, m.review.cursor.Coordinate.Y-rowsAbove)
	m.review.viewport = m.review.view.KeepVisible(m.review.viewport, m.review.cursor)
}

func (m Model) identifySelection() (cursorIdentity, cursorIdentity) {
	if m.review.selection == nil {
		return cursorIdentity{}, cursorIdentity{}
	}
	return m.identify(m.review.selection.First), m.identify(m.review.selection.Last)
}

func (m Model) restoreCursor(identity cursorIdentity) view.Cursor {
	if identity.valid {
		if cursor, ok := m.review.view.FindCursor(identity.file, identity.hunk, identity.line, identity.cursor.Coordinate, identity.cursor.Pane); ok {
			return cursor
		}
	}
	if cursor, ok := m.review.view.First(); ok {
		return cursor
	}
	return view.Cursor{}
}

func (m Model) restoreSelection(first, last cursorIdentity) *view.Selection {
	if !first.valid || !last.valid {
		return nil
	}
	translatedFirst, firstFound := m.findCursor(first)
	translatedLast, lastFound := m.findCursor(last)
	if !firstFound || !lastFound {
		return nil
	}
	firstHunk, firstHunkFound := m.review.view.Hunk(translatedFirst)
	firstFile, firstFileFound := m.review.view.File(translatedFirst)
	lastHunk, lastHunkFound := m.review.view.Hunk(translatedLast)
	lastFile, lastFileFound := m.review.view.File(translatedLast)
	if !firstHunkFound || !firstFileFound || !lastHunkFound || !lastFileFound {
		return nil
	}
	if !samePatchFile(firstFile, lastFile) || firstHunk.Header != lastHunk.Header {
		return nil
	}
	selection := view.Selection{First: translatedFirst, Last: translatedLast}
	return &selection
}

func (m Model) findCursor(identity cursorIdentity) (view.Cursor, bool) {
	if !identity.valid {
		return view.Cursor{}, false
	}
	return m.review.view.FindCursor(identity.file, identity.hunk, identity.line, identity.cursor.Coordinate, identity.cursor.Pane)
}

func (m Model) newReviewView(p patch.Patch) view.View {
	if m.sideBySideActive() {
		return view.NewSideBySideView(p, m.darkBackground)
	}
	return view.NewUnifiedView(p, m.darkBackground)
}

func samePatchFile(first, last patch.File) bool {
	return first.OldPath == last.OldPath && first.NewPath == last.NewPath
}
