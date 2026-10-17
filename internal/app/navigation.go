package app

import (
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

// navigate applies a diff navigation binding and reports whether it handled the
// key.
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
