package comments

import "github.com/eskelinenantti/review-my-slop/internal/patch"

// NewAnchor records the selected diff lines in their supplied order, using the
// file's preferred path and the minimum and maximum line numbers on each side.
// Zero line numbers denote an absent side and do not contribute to its range.
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
