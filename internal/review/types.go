package review

import (
	"time"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type Anchor struct {
	FilePath    string   `json:"file"`
	OldStart    int      `json:"old_start,omitempty"`
	OldEnd      int      `json:"old_end,omitempty"`
	NewStart    int      `json:"new_start,omitempty"`
	NewEnd      int      `json:"new_end,omitempty"`
	QuotedLines []string `json:"quoted_lines"`
}

type Comment struct {
	ID         string    `json:"id"`
	Repository string    `json:"repository"`
	CreatedAt  time.Time `json:"created_at"`
	Anchor     Anchor    `json:"anchor"`
	Body       string    `json:"body"`
}

func NewAnchor(filePath string, lines []patch.Line) Anchor {
	anchor := Anchor{FilePath: filePath}
	for _, line := range lines {
		anchor.QuotedLines = append(anchor.QuotedLines, linePrefix(line)+line.Text)
		addLineNumber(&anchor.OldStart, &anchor.OldEnd, int(line.OldNumber))
		addLineNumber(&anchor.NewStart, &anchor.NewEnd, int(line.NewNumber))
	}
	return anchor
}

func linePrefix(line patch.Line) string {
	switch line.Kind {
	case patch.Addition:
		return "+"
	case patch.Deletion:
		return "-"
	default:
		return " "
	}
}

func addLineNumber(start, end *int, value int) {
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
