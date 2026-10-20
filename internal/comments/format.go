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
		appendPromptComment(&prompt, index+1, comment)
	}
	return prompt.String()
}

func appendPromptComment(prompt *strings.Builder, number int, comment Comment) {
	anchor := comment.Anchor
	fmt.Fprintf(
		prompt,
		"\n### %d. `%s` (%s)\n\n",
		number,
		anchor.FilePath,
		describeRange(anchor),
	)
	appendQuotedLines(prompt, anchor.QuotedLines)
	_, _ = prompt.WriteString(strings.TrimSpace(comment.Body))
	_ = prompt.WriteByte('\n')
}

func appendQuotedLines(prompt *strings.Builder, lines []string) {
	if len(lines) == 0 {
		return
	}
	_, _ = prompt.WriteString("```diff\n")
	for _, line := range lines {
		_, _ = prompt.WriteString(line)
		_ = prompt.WriteByte('\n')
	}
	_, _ = prompt.WriteString("```\n")
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
