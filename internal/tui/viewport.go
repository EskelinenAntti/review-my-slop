package tui

type Viewport struct {
	Cursor int
	Top    int
	Rows   int
	Total  int
}

func (v *Viewport) Set(total, rows int) {
	v.Total = max(0, total)
	v.Rows = max(1, rows)
	if v.Cursor >= v.Total {
		v.Cursor = max(0, v.Total-1)
	}
	if v.Cursor < 0 {
		v.Cursor = 0
	}
	v.KeepVisible()
}

func (v *Viewport) Move(delta int) {
	v.Cursor += delta
	if v.Cursor < 0 {
		v.Cursor = 0
	}
	if v.Cursor >= v.Total {
		v.Cursor = max(0, v.Total-1)
	}
	v.KeepVisible()
}

func (v *Viewport) PageDown() {
	v.Move(max(1, v.Rows/2))
}

func (v *Viewport) PageUp() {
	v.Move(-max(1, v.Rows/2))
}

func (v *Viewport) KeepVisible() {
	if v.Cursor < v.Top {
		v.Top = v.Cursor
	}
	if v.Cursor >= v.Top+v.Rows {
		v.Top = v.Cursor - v.Rows + 1
	}
	if v.Top < 0 {
		v.Top = 0
	}
	limit := max(0, v.Total-v.Rows)
	if v.Top > limit {
		v.Top = limit
	}
}
