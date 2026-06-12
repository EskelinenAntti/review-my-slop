package inbox

import (
	"fmt"
	"io"
	"strings"

	"github.com/anttieskelinen/review-my-slop/internal/review"
)

func WritePrompt(w io.Writer, batches []review.Batch) error {
	if len(batches) == 0 {
		_, err := fmt.Fprintln(w, "No pending review comments.")
		return err
	}
	if _, err := fmt.Fprintln(w, "Apply the following review feedback to the current repository. Address every comment, run relevant tests, and report what changed."); err != nil {
		return err
	}
	for batchIndex, batch := range batches {
		if _, err := fmt.Fprintf(w, "\n## Review batch %d\n", batchIndex+1); err != nil {
			return err
		}
		for commentIndex, comment := range batch.Comments {
			a := comment.Anchor
			if _, err := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", commentIndex+1, a.File, describeRange(a)); err != nil {
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
