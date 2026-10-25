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
		text := promptComment(index+1, comment)
		if _, err := fmt.Fprint(w, text); err != nil {
			return err
		}
	}
	return nil
}

func promptComment(number int, comment Comment) string {
	var text strings.Builder
	anchor := comment.Anchor
	_, _ = fmt.Fprintf(
		&text,
		"\n### %d. `%s` (%s)\n\n",
		number,
		anchor.FilePath,
		describeRange(anchor),
	)
	writeQuotedLines(&text, anchor.QuotedLines)
	_, _ = fmt.Fprintln(&text, strings.TrimSpace(comment.Body))
	return text.String()
}

func writeQuotedLines(text *strings.Builder, lines []string) {
	if len(lines) == 0 {
		return
	}
	_, _ = text.WriteString("```diff\n")
	for _, line := range lines {
		_, _ = text.WriteString(line)
		_ = text.WriteByte('\n')
	}
	_, _ = text.WriteString("```\n")
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
