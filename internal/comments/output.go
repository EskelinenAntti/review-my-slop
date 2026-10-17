package comments

import (
	"fmt"
	"io"
	"strings"
)

// WritePending emits repository feedback and acknowledges only emitted
// comments.
func (s Store) WritePending(w io.Writer, repository string) error {
	pending, err := s.List(repository)
	if err != nil {
		return err
	}
	if err := writePendingComments(w, pending); err != nil {
		return err
	}
	return s.acknowledge(repository, commentIDs(pending))
}

func commentIDs(comments []Comment) []string {
	ids := make([]string, len(comments))
	for i, comment := range comments {
		ids[i] = comment.ID
	}
	return ids
}

func writePendingComments(w io.Writer, comments []Comment) error {
	if len(comments) == 0 {
		_, err := fmt.Fprintln(w, "No pending review comments.")
		return err
	}
	if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range comments {
		if err := writePendingComment(w, index+1, comment); err != nil {
			return err
		}
	}
	return nil
}

func writePendingComment(
	w io.Writer,
	number int,
	comment Comment,
) error {
	anchor := comment.Anchor
	if err := writePendingCommentHeading(w, number, anchor); err != nil {
		return err
	}
	if err := writeQuotedAnchor(w, anchor.QuotedLines); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
	return err
}

func writePendingCommentHeading(
	w io.Writer,
	number int,
	anchor Anchor,
) error {
	_, err := fmt.Fprintf(
		w,
		"\n### %d. `%s` (%s)\n\n",
		number,
		anchor.FilePath,
		commentAnchorLocation(anchor),
	)
	return err
}

func writeQuotedAnchor(w io.Writer, lines []string) error {
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

func commentAnchorLocation(anchor Anchor) string {
	sides := anchorSides(anchor)
	if len(sides) == 0 {
		return "diff lines"
	}
	return strings.Join(sides, ", ")
}

func anchorSides(anchor Anchor) []string {
	var sides []string
	if anchor.OldStart > 0 {
		sides = append(sides, formatAnchorSide(
			"old",
			anchor.OldStart,
			anchor.OldEnd,
		))
	}
	if anchor.NewStart > 0 {
		sides = append(sides, formatAnchorSide(
			"new",
			anchor.NewStart,
			anchor.NewEnd,
		))
	}
	return sides
}

func formatAnchorSide(side string, start, end int) string {
	if end == 0 || end == start {
		return fmt.Sprintf("%s line %d", side, start)
	}
	return fmt.Sprintf("%s lines %d-%d", side, start, end)
}
