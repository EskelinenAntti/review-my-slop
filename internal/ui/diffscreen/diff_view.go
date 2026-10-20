package diffscreen

import (
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type diffView struct {
	patch        patch.Patch
	rows         []entry
	maxLineWidth int
	countsLabel  string
	split        bool
	styles       lineStyles
}

func newDiffView(p patch.Patch, options Options) *diffView {
	rows, maxLineWidth := projectRows(p, options)
	return &diffView{
		patch:        p,
		rows:         rows,
		maxLineWidth: maxLineWidth,
		countsLabel:  changeCountsLabel(p),
		split:        options.SideBySide,
		styles:       newLineStyles(options.Dark),
	}
}

func changeCountsLabel(p patch.Patch) string {
	added, removed := 0, 0
	for _, file := range p.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				switch line.Kind {
				case patch.Addition:
					added++
				case patch.Deletion:
					removed++
				default:
					continue
				}
			}
		}
	}
	return fmt.Sprintf("+%d-%d", added, removed)
}
