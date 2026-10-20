package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

// ToggleSelection starts a range at the focused line or clears the current
// range.
func (v *View) ToggleSelection() {
	if v.selection != nil {
		v.ClearSelection()
		return
	}
	v.search.repeatMiss = false
	if _, ok := v.view.line(v.cursor); ok {
		selection := beginSelection(v.cursor)
		v.selection = &selection
	}
}

// ClearSelection clears the selected range and ends any drag.
func (v *View) ClearSelection() {
	v.EndDrag()
	v.selection = nil
	v.search.repeatMiss = false
}

// Selected returns the focused line when no explicit range is selected.
// The bool is false when the patch has no selectable code.
func (v *View) Selected() (patch.File, []patch.Line, bool) {
	selection := beginSelection(v.cursor)
	if v.selection != nil {
		selection = *v.selection
	}
	file, ok := v.view.file(selection.First)
	lines := v.view.lines(selection)
	return file, lines, ok && len(lines) > 0
}

// extendSelectionTo accepts unrestricted movement when no range is active.
// An active range changes only when its endpoint remains in the original hunk.
func (v *View) extendSelectionTo(cursor diffCursor) bool {
	if v.selection == nil {
		return true
	}
	selection, ok := v.view.selectionBetween(v.selection.First, cursor)
	if ok {
		v.selection = &selection
	}
	return ok
}
