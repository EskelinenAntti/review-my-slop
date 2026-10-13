package comments

import (
	"fmt"
	"io"
	"strings"
)

// WritePending writes the repository's pending feedback and acknowledges only
// those comments after successful output. Failed output leaves them pending.
func (s Store) WritePending(w io.Writer, repository string) error {
	pending, err := s.List(repository)
	if err != nil {
		return err
	}
	if err := WritePrompt(w, pending); err != nil {
		return err
	}
	ids := make([]string, len(pending))
	for i, comment := range pending {
		ids[i] = comment.ID
	}
	return s.Acknowledge(repository, ids)
}

// WritePrompt writes comments as Markdown feedback for a coding agent.
func WritePrompt(w io.Writer, comments []Comment) error {
	if len(comments) == 0 {
		_, err := fmt.Fprintln(w, "No pending review comments.")
		return err
	}
	if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range comments {
		if err := writePromptComment(w, index+1, comment); err != nil {
			return err
		}
	}
	return nil
}

func writePromptComment(w io.Writer, number int, comment Comment) error {
	anchor := comment.Anchor
	_, headingErr := fmt.Fprintf(
		w,
		"\n### %d. `%s` (%s)\n\n",
		number,
		anchor.FilePath,
		describeAnchorLineRange(anchor),
	)
	if headingErr != nil {
		return headingErr
	}
	if err := writeQuotedLines(w, anchor.QuotedLines); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
	return err
}

func writeQuotedLines(w io.Writer, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "```diff"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "```")
	return err
}

func describeAnchorLineRange(anchor Anchor) string {
	var sides []string
	if anchor.OldStart > 0 {
		sides = append(sides, formatSourceLineRange(
			"old", anchor.OldStart, anchor.OldEnd,
		))
	}
	if anchor.NewStart > 0 {
		sides = append(sides, formatSourceLineRange(
			"new", anchor.NewStart, anchor.NewEnd,
		))
	}
	if len(sides) == 0 {
		return "diff lines"
	}
	return strings.Join(sides, ", ")
}

func formatSourceLineRange(sourceSide string, start, end int) string {
	if end == 0 || end == start {
		return fmt.Sprintf("%s line %d", sourceSide, start)
	}
	return fmt.Sprintf("%s lines %d-%d", sourceSide, start, end)
}
