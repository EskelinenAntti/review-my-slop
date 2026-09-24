package comments

import (
	"fmt"
	"io"
	"strings"
)

func WritePrompt(w io.Writer, comments []Comment) error {
	println := func(values ...any) error {
		_, err := fmt.Fprintln(w, values...)
		return err
	}
	if len(comments) == 0 {
		return println("No pending review comments.")
	}
	if err := println("New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range comments {
		a := comment.Anchor
		if _, err := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, a.FilePath, describeRange(a)); err != nil {
			return err
		}
		if len(a.QuotedLines) > 0 {
			if err := println("```diff"); err != nil {
				return err
			}
			for _, line := range a.QuotedLines {
				if err := println(line); err != nil {
					return err
				}
			}
			if err := println("```"); err != nil {
				return err
			}
		}
		if err := println(strings.TrimSpace(comment.Body)); err != nil {
			return err
		}
	}
	return nil
}

func describeRange(anchor Anchor) string {
	var sides []string
	if anchor.OldStart > 0 {
		sides = append(sides, lineRange("old", anchor.OldStart, anchor.OldEnd))
	}
	if anchor.NewStart > 0 {
		sides = append(sides, lineRange("new", anchor.NewStart, anchor.NewEnd))
	}
	if len(sides) == 0 {
		return "diff lines"
	}
	return strings.Join(sides, ", ")
}

func lineRange(side string, start, end int) string {
	if end == 0 || end == start {
		return fmt.Sprintf("%s line %d", side, start)
	}
	return fmt.Sprintf("%s lines %d-%d", side, start, end)
}
