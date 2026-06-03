package review

import (
	"io"

	"github.com/eskelinenantti/review-my-slop/internal/diffparse"
	"github.com/eskelinenantti/review-my-slop/internal/tui"
)

type App struct {
	Loader   Loader
	Editor   Editor
	lines    []diffparse.Line
	folded   map[string]bool
	viewport tui.Viewport
}

func New(loader Loader, editor Editor) (*App, error) {
	app := &App{
		Loader: loader,
		Editor: editor,
		folded: map[string]bool{},
	}
	if app.Editor == nil {
		app.Editor = ExecEditor{}
	}
	if err := app.Reload(); err != nil {
		return nil, err
	}
	return app, nil
}

func (a *App) Reload() error {
	before := a.currentSelectionLocation()
	lines, err := a.Loader.Load()
	if err != nil {
		return err
	}
	a.lines = lines
	a.viewport.Set(len(a.lines), a.viewport.Rows)
	a.restorePosition(before)
	return nil
}

func (a *App) Draw(w io.Writer, rows, cols int) {
	indexes := a.visibleIndexes()
	viewCursor := a.visiblePosition(indexes, a.viewport.Cursor)
	if viewCursor < 0 {
		a.viewport.Cursor = a.nearestSelectable(a.viewport.Cursor)
		viewCursor = a.visiblePosition(indexes, a.viewport.Cursor)
	}
	if viewCursor < 0 {
		viewCursor = 0
	}
	viewTop := a.visiblePosition(indexes, a.viewport.Top)
	if viewTop < 0 {
		viewTop = 0
	}
	view := tui.Viewport{Cursor: viewCursor, Top: viewTop}
	a.layoutViewport(&view, rows, indexes)
	a.viewport.Rows = view.Rows
	if view.Top >= 0 && view.Top < len(indexes) {
		a.viewport.Top = indexes[view.Top]
	}
	text := make([]string, len(indexes))
	viewLines := make([]diffparse.Line, len(indexes))
	for i, original := range indexes {
		line := a.lines[original]
		text[i] = line.Text
		viewLines[i] = line
	}
	tui.RenderWithOptions(w, text, view, cols, tui.RenderOptions{
		Sticky: stickyHeader(viewLines, view.Top),
	})
}

func (a *App) Handle(key tui.Key, term tui.Terminal) (bool, error) {
	switch key {
	case tui.KeyCtrlC, tui.KeyEscape, "q":
		return true, nil
	case "j", tui.KeyDown:
		a.moveSelection(1)
	case "k", tui.KeyUp:
		a.moveSelection(-1)
	case "l", tui.KeyRight:
		a.unfoldCurrentFile()
	case "h", tui.KeyLeft:
		a.foldCurrentFile()
	case tui.KeyCtrlD, tui.KeyPageDown:
		a.moveSelection(max(1, a.viewport.Rows/2))
	case tui.KeyCtrlN:
		a.moveFile(1)
	case tui.KeyCtrlP:
		a.moveFile(-1)
	case tui.KeyCtrlU, tui.KeyPageUp:
		a.moveSelection(-max(1, a.viewport.Rows/2))
	case "e":
		if err := a.edit(term); err != nil {
			return false, err
		}
	}
	return false, nil
}

func (a *App) edit(term tui.Terminal) error {
	loc := a.currentEditableLocation()
	if loc.File == "" || loc.Line == 0 {
		return nil
	}
	err := term.Suspend(func() error {
		return a.Editor.Open(loc.File, loc.Line)
	})
	if err != nil {
		return err
	}
	return a.Reload()
}

func (a *App) currentSelectionLocation() diffparse.Location {
	if a.viewport.Cursor < 0 || a.viewport.Cursor >= len(a.lines) {
		return diffparse.Location{}
	}
	if loc := a.lines[a.viewport.Cursor].Location; a.lines[a.viewport.Cursor].Selectable && loc.File != "" && loc.Line > 0 {
		return loc
	}
	return diffparse.Location{}
}

func (a *App) currentEditableLocation() diffparse.Location {
	if a.viewport.Cursor < 0 || a.viewport.Cursor >= len(a.lines) {
		return diffparse.Location{}
	}
	if loc := a.lines[a.viewport.Cursor].Location; a.lines[a.viewport.Cursor].Editable && loc.File != "" && loc.Line > 0 {
		return loc
	}
	return diffparse.Location{}
}

func (a *App) restorePosition(loc diffparse.Location) {
	if len(a.lines) == 0 {
		a.viewport.Set(0, a.viewport.Rows)
		return
	}
	if loc.File != "" && loc.Line > 0 {
		for i, line := range a.lines {
			if line.Location == loc {
				a.viewport.Cursor = i
				a.viewport.KeepVisible()
				return
			}
		}
	}
	if a.viewport.Cursor >= len(a.lines) {
		a.viewport.Cursor = len(a.lines) - 1
	}
	if !a.selectable(a.viewport.Cursor) {
		a.viewport.Cursor = a.nearestSelectable(a.viewport.Cursor)
	}
	a.viewport.KeepVisible()
}

func (a *App) moveSelection(delta int) {
	if delta == 0 || len(a.lines) == 0 {
		return
	}
	cursor := a.viewport.Cursor
	step := 1
	if delta < 0 {
		step = -1
		delta = -delta
	}
	if !a.selectable(cursor) {
		cursor = a.nearestSelectable(cursor)
		delta--
	}
	for delta > 0 {
		next := a.nextSelectable(cursor, step)
		if next == cursor {
			break
		}
		cursor = next
		delta--
	}
	a.viewport.Cursor = cursor
}

func (a *App) nearestSelectable(index int) int {
	if len(a.lines) == 0 {
		return 0
	}
	if index < 0 {
		index = 0
	}
	if index >= len(a.lines) {
		index = len(a.lines) - 1
	}
	if a.selectable(index) {
		return index
	}
	for distance := 1; distance < len(a.lines); distance++ {
		down := index + distance
		if down < len(a.lines) && a.selectable(down) {
			return down
		}
		up := index - distance
		if up >= 0 && a.selectable(up) {
			return up
		}
	}
	return 0
}

func (a *App) nextSelectable(index, step int) int {
	for next := index + step; next >= 0 && next < len(a.lines); next += step {
		if a.selectable(next) {
			return next
		}
	}
	return index
}

func (a *App) foldCurrentFile() {
	header := a.currentHeaderIndex()
	if header < 0 {
		return
	}
	a.folded[a.headerKey(header)] = true
	a.viewport.Cursor = header
	a.viewport.Top = header
}

func (a *App) unfoldCurrentFile() {
	header := a.currentHeaderIndex()
	if header < 0 {
		return
	}
	key := a.headerKey(header)
	if !a.folded[key] {
		return
	}
	delete(a.folded, key)
	if target, ok := a.firstSelectableAfter(header); ok {
		a.viewport.Cursor = target
	}
}

func (a *App) moveFile(direction int) {
	if direction > 0 {
		if target, ok := a.nextFileSelection(); ok {
			a.viewport.Cursor = target
			a.viewport.KeepVisible()
		}
		return
	}
	if target, ok := a.previousFileSelection(); ok {
		a.viewport.Cursor = target
		a.viewport.KeepVisible()
	}
}

func (a *App) nextFileSelection() (int, bool) {
	for i := a.viewport.Cursor + 1; i < len(a.lines); i++ {
		if !a.lines[i].Header {
			continue
		}
		if target, ok := a.firstSelectableAfter(i); ok {
			return target, true
		}
	}
	return 0, false
}

func (a *App) previousFileSelection() (int, bool) {
	currentHeader := a.currentHeaderIndex()
	if currentHeader <= 0 {
		return 0, false
	}
	for i := currentHeader - 1; i >= 0; i-- {
		if !a.lines[i].Header {
			continue
		}
		if target, ok := a.firstSelectableAfter(i); ok {
			return target, true
		}
	}
	return 0, false
}

func (a *App) currentHeaderIndex() int {
	for i := min(a.viewport.Cursor, len(a.lines)-1); i >= 0; i-- {
		if a.lines[i].Header {
			return i
		}
	}
	return -1
}

func (a *App) firstSelectableAfter(header int) (int, bool) {
	if a.folded[a.headerKey(header)] {
		return header, true
	}
	for i := header + 1; i < len(a.lines); i++ {
		if a.lines[i].Header {
			return 0, false
		}
		if a.selectable(i) {
			return i, true
		}
	}
	return 0, false
}

func (a *App) selectable(index int) bool {
	if index < 0 || index >= len(a.lines) {
		return false
	}
	if a.lines[index].Header && a.folded[a.headerKey(index)] {
		return true
	}
	if a.hidden(index) {
		return false
	}
	return a.lines[index].Selectable
}

func (a *App) keepContextHeaderVisible(v *tui.Viewport, indexes []int) {
	if v.Rows < 2 || v.Cursor <= 0 || v.Top != v.Cursor {
		return
	}
	previous := indexes[v.Cursor-1]
	if a.selectable(previous) {
		return
	}
	v.Top--
}

func (a *App) layoutViewport(v *tui.Viewport, rows int, indexes []int) {
	v.Set(len(indexes), rows)
	a.keepContextHeaderVisible(v, indexes)
	viewLines := make([]diffparse.Line, len(indexes))
	for i, original := range indexes {
		viewLines[i] = a.lines[original]
	}
	for range len(indexes) + 1 {
		if !stickyHeader(viewLines, v.Top).Active || v.Rows < 2 {
			return
		}
		visibleRows := v.Rows - 1
		if v.Cursor < v.Top {
			v.Top = v.Cursor
			continue
		}
		bottom := v.Top + visibleRows - 1
		if v.Cursor > bottom {
			v.Top = v.Cursor - visibleRows + 1
			continue
		}
		return
	}
}

func stickyHeader(lines []diffparse.Line, top int) tui.StickyLine {
	if len(lines) == 0 || top >= len(lines) {
		return tui.StickyLine{}
	}
	header := -1
	for i := top; i >= 0; i-- {
		if lines[i].Header {
			header = i
			break
		}
	}
	if header < 0 || header == top {
		return tui.StickyLine{}
	}
	return tui.StickyLine{Text: lines[header].Text, Active: true}
}

func (a *App) hidden(index int) bool {
	if index < 0 || index >= len(a.lines) || a.lines[index].Header {
		return false
	}
	header := a.headerFor(index)
	return header >= 0 && a.folded[a.headerKey(header)]
}

func (a *App) headerFor(index int) int {
	for i := min(index, len(a.lines)-1); i >= 0; i-- {
		if a.lines[i].Header {
			return i
		}
	}
	return -1
}

func (a *App) headerKey(index int) string {
	if index < 0 || index >= len(a.lines) {
		return ""
	}
	return a.lines[index].Text
}

func (a *App) visibleIndexes() []int {
	indexes := make([]int, 0, len(a.lines))
	for i := range a.lines {
		if !a.hidden(i) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

func (a *App) visiblePosition(indexes []int, original int) int {
	for i, index := range indexes {
		if index == original {
			return i
		}
	}
	return -1
}
