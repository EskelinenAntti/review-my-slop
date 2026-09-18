package inbox

import (
	"fmt"
	"io"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func WritePrompt(w io.Writer, comments []review.Comment) error {
	if len(comments) == 0 {
		return writeLine(w, "No pending review comments.")
	}
	if err := writeLine(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range comments {
		if err := writeComment(w, index+1, comment); err != nil {
			return err
		}
	}
	return nil
}

func writeComment(w io.Writer, number int, comment review.Comment) error {
	anchor := comment.Anchor
	if _, err := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", number, anchor.FilePath, describeRange(anchor)); err != nil {
		return err
	}
	if err := writeQuotedLines(w, anchor.QuotedLines); err != nil {
		return err
	}
	return writeLine(w, strings.TrimSpace(comment.Body))
}

func writeQuotedLines(w io.Writer, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	if err := writeLine(w, "```diff"); err != nil {
		return err
	}
	for _, line := range lines {
		if err := writeLine(w, line); err != nil {
			return err
		}
	}
	return writeLine(w, "```")
}

func writeLine(w io.Writer, line string) error {
	_, err := fmt.Fprintln(w, line)
	return err
}

func describeRange(anchor review.Anchor) string {
	var ranges []string
	if anchor.OldStart > 0 {
		ranges = append(ranges, lineRange("old", anchor.OldStart, anchor.OldEnd))
	}
	if anchor.NewStart > 0 {
		ranges = append(ranges, lineRange("new", anchor.NewStart, anchor.NewEnd))
	}
	if len(ranges) == 0 {
		return "diff lines"
	}
	return strings.Join(ranges, ", ")
}

func lineRange(side string, start, end int) string {
	if end == 0 || end == start {
		return fmt.Sprintf("%s line %d", side, start)
	}
	return fmt.Sprintf("%s lines %d-%d", side, start, end)
}
