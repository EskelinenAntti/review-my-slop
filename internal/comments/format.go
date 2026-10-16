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
	if err := WriteReviewPrompt(w, pending); err != nil {
		return err
	}
	ids := make([]string, len(pending))
	for i, comment := range pending {
		ids[i] = comment.ID
	}
	return s.Acknowledge(repository, ids)
}

// WriteReviewPrompt writes comments as Markdown feedback for a coding agent.
func WriteReviewPrompt(w io.Writer, reviewComments []Comment) error {
	if len(reviewComments) == 0 {
		_, err := fmt.Fprintln(w, "No pending review comments.")
		return err
	}
	if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range reviewComments {
		if err := writeReviewPromptComment(w, index+1, comment); err != nil {
			return err
		}
	}
	return nil
}

func writeReviewPromptComment(w io.Writer, commentNumber int, comment Comment) error {
	anchor := comment.Anchor
	_, headingErr := fmt.Fprintf(
		w,
		"\n### %d. `%s` (%s)\n\n",
		commentNumber,
		anchor.FilePath,
		describeAnchorRange(anchor),
	)
	if headingErr != nil {
		return headingErr
	}
	if err := writeQuotedDiff(w, anchor.QuotedLines); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
	return err
}

func writeQuotedDiff(w io.Writer, quotedDiffLines []string) error {
	if len(quotedDiffLines) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "```diff"); err != nil {
		return err
	}
	for _, line := range quotedDiffLines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, "```")
	return err
}

func describeAnchorRange(anchor Anchor) string {
	var sides []string
	if anchor.OldStart > 0 {
		sides = append(
			sides,
			formatSideLineRange("old", anchor.OldStart, anchor.OldEnd),
		)
	}
	if anchor.NewStart > 0 {
		sides = append(
			sides,
			formatSideLineRange("new", anchor.NewStart, anchor.NewEnd),
		)
	}
	if len(sides) == 0 {
		return "diff lines"
	}
	return strings.Join(sides, ", ")
}

func formatSideLineRange(diffSide string, startLine, endLine int) string {
	if endLine == 0 || endLine == startLine {
		return fmt.Sprintf("%s line %d", diffSide, startLine)
	}
	return fmt.Sprintf("%s lines %d-%d", diffSide, startLine, endLine)
}
