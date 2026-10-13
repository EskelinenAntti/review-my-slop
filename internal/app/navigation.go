package app

import "github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"

// applyNavigationKey applies a Patch navigation binding and reports whether it
// handled the key.
func (m *model) applyNavigationKey(name string) bool {
	if motion, ok := navigationMotions[name]; ok {
		m.patchView.Move(motion)
		return true
	}
	if alignment, ok := navigationAlignments[name]; ok {
		m.patchView.Align(alignment)
		return true
	}
	switch name {
	case "n":
		m.patchView.Find(diffscreen.Forward)
	case "N":
		m.patchView.Find(diffscreen.Backward)
	case "h", "left":
		m.patchView.ScrollHorizontal(-horizontalScrollStep)
	case "l", "right":
		m.patchView.ScrollHorizontal(horizontalScrollStep)
	case "0":
		m.patchView.ScrollHorizontal(-int(^uint(0) >> 1))
	case "$":
		m.patchView.ScrollHorizontal(int(^uint(0) >> 1))
	default:
		return false
	}
	return true
}

var navigationMotions = map[string]diffscreen.Motion{
	"j":             diffscreen.NextLine,
	"down":          diffscreen.NextLine,
	"k":             diffscreen.PreviousLine,
	"up":            diffscreen.PreviousLine,
	"ctrl+d":        diffscreen.NextPage,
	"ctrl+u":        diffscreen.PreviousPage,
	"g g":           diffscreen.FirstLine,
	"G":             diffscreen.LastLine,
	"] f":           diffscreen.NextFile,
	"[ f":           diffscreen.PreviousFile,
	"ctrl+w h":      diffscreen.OldPane,
	"ctrl+w l":      diffscreen.NewPane,
	"ctrl+w ctrl+w": diffscreen.OtherPane,
}

var navigationAlignments = map[string]diffscreen.Alignment{
	"z z": diffscreen.Center,
	"z t": diffscreen.Top,
	"z b": diffscreen.Bottom,
}
