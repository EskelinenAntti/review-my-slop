package comments

import "github.com/eskelinenantti/review-my-slop/internal/patch"

// NewAnchor captures a file's selected diff lines as quoted text and old/new
// line ranges. Lines absent on one side do not contribute to that side's range.
func NewAnchor(file patch.File, lines []patch.Line) Anchor {
	anchor := Anchor{FilePath: file.PreferredPath()}
	for _, line := range lines {
		anchor.QuotedLines = append(
			anchor.QuotedLines,
			line.Kind.DiffPrefix()+line.Text,
		)
		accumulateRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		accumulateRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor
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
