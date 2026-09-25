package comments

import (
	"fmt"
	"io"
	"strings"
)

func WritePrompt(w io.Writer, comments []Comment) error {
	println := fmt.Fprintln
	if len(comments) == 0 {
		_, err := println(w, "No pending review comments.")
		return err
	}
	if _, err := println(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range comments {
		a := comment.Anchor
		if _, err := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, a.FilePath, describeRange(a)); err != nil {
			return err
		}
		quotedLines := a.QuotedLines
		if len(quotedLines) > 0 {
			if _, err := println(w, "```diff"); err != nil {
				return err
			}
			for _, line := range quotedLines {
				if _, err := println(w, line); err != nil {
					return err
				}
			}
			if _, err := println(w, "```"); err != nil {
				return err
			}
		}
		if _, err := println(w, strings.TrimSpace(comment.Body)); err != nil {
			return err
		}
	}
	return nil
}

func describeRange(anchor Anchor) string {
	var sides []string
	oldStart, newStart := anchor.OldStart, anchor.NewStart
	if oldStart > 0 {
		sides = append(sides, lineRange("old", oldStart, anchor.OldEnd))
	}
	if newStart > 0 {
		sides = append(sides, lineRange("new", newStart, anchor.NewEnd))
	}
	if len(sides) == 0 {
		return "diff lines"
	}
	return strings.Join(sides, ", ")
}

func lineRange(side string, start, end int) string {
	if end == 0 || end == start {
		return formatString("%s line %d", side, start)
	}
	return formatString("%s lines %d-%d", side, start, end)
}
