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
		if err := func() error {
			anchor := comment.Anchor
			_, headingErr := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, anchor.FilePath, func() string {
				var sides []string
				if anchor.OldStart > 0 {
					sides = append(sides, func() string {
						var side string = "old"
						if anchor.OldEnd == 0 || anchor.OldEnd == anchor.OldStart {
							return fmt.Sprintf("%s line %d", side, anchor.OldStart)
						}
						return fmt.Sprintf("%s lines %d-%d", side, anchor.OldStart, anchor.OldEnd)
					}())
				}
				if anchor.NewStart > 0 {
					sides = append(sides, func() string {
						var side string = "new"
						if anchor.NewEnd == 0 || anchor.NewEnd == anchor.NewStart {
							return fmt.Sprintf("%s line %d", side, anchor.NewStart)
						}
						return fmt.Sprintf("%s lines %d-%d", side, anchor.NewStart, anchor.NewEnd)
					}())
				}
				if len(sides) == 0 {
					return "diff lines"
				}
				return strings.Join(sides, ", ")
			}())
			if headingErr != nil {
				return headingErr
			}
			if err := func() error {
				if len(anchor.QuotedLines) == 0 {
					return nil
				}
				if _, err := fmt.Fprintln(w, "```diff"); err != nil {
					return err
				}
				for _, line := range anchor.QuotedLines {
					if _, err := fmt.Fprintln(w, line); err != nil {
						return err
					}
				}
				_, err := fmt.Fprintln(w, "```")
				return err
			}(); err != nil {
				return err
			}
			_, err := fmt.Fprintln(w, strings.TrimSpace(comment.Body))
			return err
		}(); err != nil {
			return err
		}
	}
	return nil
}
