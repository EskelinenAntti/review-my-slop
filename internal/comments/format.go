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
	_, err := fmt.Fprint(w, formatPrompt(comments))
	return err
}

func formatPrompt(comments []Comment) string {
	if len(comments) == 0 {
		return "No pending review comments.\n"
	}
	var prompt strings.Builder
	_, _ = prompt.WriteString("New comments since last run:\n")
	for index, comment := range comments {
		anchor := comment.Anchor
		_, _ = fmt.Fprintf(
			&prompt,
			"\n### %d. `%s` (%s)\n\n",
			index+1,
			anchor.FilePath,
			describeRange(anchor),
		)
		if len(anchor.QuotedLines) > 0 {
			_, _ = prompt.WriteString("```diff\n")
			for _, line := range anchor.QuotedLines {
				_, _ = fmt.Fprintln(&prompt, line)
			}
			_, _ = prompt.WriteString("```\n")
		}
		_, _ = fmt.Fprintln(&prompt, strings.TrimSpace(comment.Body))
	}
	return prompt.String()
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
