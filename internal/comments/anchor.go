package comments

import (
	"fmt"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

// AnchorFor attaches a comment to the supplied patch lines in file.
func AnchorFor(file patch.File, lines []patch.Line) (Anchor, error) {
	if len(lines) == 0 {
		return Anchor{}, fmt.Errorf("select code lines before commenting")
	}
	anchor := Anchor{FilePath: file.Path()}
	for _, line := range lines {
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = "+"
		}
		if line.Kind == patch.Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor, nil
}

func accumulateRange(start, end *int, value int) {
	if value == 0 {
		return
	}
	if *start == 0 || value < *start {
		*start = value
	}
	if value > *end {
		*end = value
	}
}
