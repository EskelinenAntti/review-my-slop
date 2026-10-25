package diffscreen

import "github.com/eskelinenantti/review-my-slop/internal/patch"

func (v *diffView) valid(cursor diffCursor) bool {
	return cursor.row >= 0 && cursor.row < len(v.rows) &&
		v.rows[cursor.row].lineIndex(cursor.pane) >= 0
}

// lineIndex uses the projection's pane indexes; unified rows store the same
// source index in both panes, so lookup does not need to know the layout.
func (e entry) lineIndex(pane diffPane) int {
	if e.kind != lineRow {
		return -1
	}
	if pane == right {
		return e.rightLine
	}
	return e.leftLine
}

func (v *diffView) file(cursor diffCursor) (patch.File, bool) {
	if !v.valid(cursor) {
		return patch.File{}, false
	}
	return v.patch.Files[v.rows[cursor.row].file], true
}

func (v *diffView) line(cursor diffCursor) (patch.Line, bool) {
	if !v.valid(cursor) {
		return patch.Line{}, false
	}
	current := v.rows[cursor.row]
	index := current.lineIndex(cursor.pane)
	return v.patch.Files[current.file].Hunks[current.hunk].Lines[index], true
}
