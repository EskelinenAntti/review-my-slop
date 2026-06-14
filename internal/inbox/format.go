package inbox

import (
	"fmt"
	"io"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/review"
)

func WritePrompt(w io.Writer, messages []Message) error {
	if len(messages) == 0 {
		_, err := fmt.Fprintln(w, "No pending review comments.")
		return err
	}
	if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, message := range messages {
		comment := message.Comment
		a := comment.Anchor
		if _, err := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, a.File, describeRange(a)); err != nil {
			return err
		}
		if len(a.QuotedLines) > 0 {
			if _, err := fmt.Fprintln(w, "```diff"); err != nil {
				return err
			}
			for _, line := range a.QuotedLines {
				if _, err := fmt.Fprintln(w, line); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w, "```"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body)); err != nil {
			return err
		}
	}
	return nil
}

func describeRange(anchor review.Anchor) string {
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
