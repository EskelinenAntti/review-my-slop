package app

import (
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func commentAnchor(file patch.File, lines []patch.Line) comments.Anchor {
	path := file.NewPath
	if path == "" {
		path = file.OldPath
	}
	anchor := comments.Anchor{FilePath: path}
	for _, line := range lines {
		prefix := " "
		if line.Kind == patch.Addition {
			prefix = "+"
		}
		if line.Kind == patch.Deletion {
			prefix = "-"
		}
		anchor.QuotedLines = append(anchor.QuotedLines, prefix+line.Text)
		extendLineRange(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		extendLineRange(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor
}

func extendLineRange(start, end *int, value int) {
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
