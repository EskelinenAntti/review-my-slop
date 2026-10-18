package comments

import (
	"fmt"
	"io"
	"strings"
)

// WritePending writes the repository's pending feedback and acknowledges only
// those comments after successful output. Failed output leaves them pending.
func (s Store) WritePending(w io.Writer, repository string) error {
	pendingComments, err := s.List(repository)
	if err != nil {
		return err
	}
	if err := WritePrompt(w, pendingComments); err != nil {
		return err
	}
	ids := make([]string, len(pendingComments))
	for index, comment := range pendingComments {
		ids[index] = comment.ID
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
	quoted := ""
	if len(anchor.QuotedLines) > 0 {
		quoted = "```diff\n" +
			strings.Join(anchor.QuotedLines, "\n") + "\n```\n"
	}
	_, err := fmt.Fprintf(
		w,
		"\n### %d. `%s` (%s)\n\n%s%s\n",
		number,
		anchor.FilePath,
		describeRange(anchor),
		quoted,
		strings.TrimSpace(comment.Body),
	)
	return err
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
