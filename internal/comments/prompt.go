package comments

import (
	"fmt"
	"io"
	"strings"
)

// WritePrompt writes comments in the agent-facing format. It intentionally
// knows nothing about storage batches or delivery state.
func WritePrompt(w io.Writer, comments []Comment) error {
	if len(comments) == 0 {
		_, err := fmt.Fprintln(w, "No pending review comments.")
		return err
	}
	if _, err := fmt.Fprintln(w, "New comments since last run:"); err != nil {
		return err
	}
	for index, comment := range comments {
		anchor := comment.Anchor
		if _, err := fmt.Fprintf(w, "\n### %d. `%s` (%s)\n\n", index+1, safeText(anchor.FilePath), describeRange(anchor)); err != nil {
			return err
		}
		if len(anchor.QuotedLines) > 0 {
			if _, err := fmt.Fprintln(w, "```diff"); err != nil {
				return err
			}
			for _, line := range anchor.QuotedLines {
				if _, err := fmt.Fprintln(w, safeText(line)); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintln(w, "```"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, strings.TrimSpace(safeBody(comment.Body))); err != nil {
			return err
		}
	}
	return nil
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

func safeText(value string) string {
	var result strings.Builder
	for _, char := range value {
		switch {
		case char == '\n':
			result.WriteString(`\n`)
		case char == '\r':
			result.WriteString(`\r`)
		case char < 0x20 || char == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, char)
		default:
			result.WriteRune(char)
		}
	}
	return result.String()
}

func safeBody(value string) string {
	var result strings.Builder
	for _, char := range value {
		switch {
		case char == '\n':
			result.WriteRune(char)
		case char == '\r':
			result.WriteString(`\r`)
		case char < 0x20 || char == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, char)
		default:
			result.WriteRune(char)
		}
	}
	return result.String()
}
