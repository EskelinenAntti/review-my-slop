package ui

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func CommentDraft(body string, anchor comments.Anchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := suggestionLines(anchor.QuotedLines)
	separator := "\n"
	if body != "" && !strings.HasSuffix(body, "\n") {
		separator = "\n\n"
	}
	return body + separator + suggestionBlock(lines) + "\n"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func SourceCommand(editor, path string, line int) *exec.Cmd {
	return CommentCommand(editor+" +"+strconv.Itoa(line), path)
}

func CommentCommand(editor, path string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" "+shellQuote(path))
}
