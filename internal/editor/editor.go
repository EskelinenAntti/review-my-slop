package editor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func createTemporaryCommentFile(body string, anchor comments.Anchor) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(formatCommentDraft(body, anchor)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close comment file: %w", err)
	}
	return path, nil
}

func readEditedComment(path string, commentAnchor comments.Anchor) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return stripUnchangedSuggestionBlock(string(body), commentAnchor.QuotedLines), nil
}

func formatCommentDraft(body string, anchor comments.Anchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := editableSuggestionLines(anchor.QuotedLines)
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	fence := suggestionMarkdownFence(lines)
	_, _ = draft.WriteString(fence)
	_, _ = draft.WriteString("suggestion\n")
	for _, line := range lines {
		_, _ = draft.WriteString(line)
		_ = draft.WriteByte('\n')
	}
	_, _ = draft.WriteString(fence)
	_ = draft.WriteByte('\n')
	return draft.String()
}

func stripUnchangedSuggestionBlock(
	editedBody string,
	quotedDiffLines []string,
) string {
	if len(quotedDiffLines) == 0 {
		return editedBody
	}
	lines := editableSuggestionLines(quotedDiffLines)
	fence := suggestionMarkdownFence(lines)
	var suggestion strings.Builder
	_, _ = suggestion.WriteString(fence)
	_, _ = suggestion.WriteString("suggestion\n")
	for _, line := range lines {
		_, _ = suggestion.WriteString(line)
		_ = suggestion.WriteByte('\n')
	}
	_, _ = suggestion.WriteString(fence)
	start := strings.Index(editedBody, suggestion.String())
	if start < 0 {
		return editedBody
	}
	end := start + suggestion.Len()
	before := strings.TrimRight(editedBody[:start], "\n")
	after := editedBody[end:]
	if strings.TrimSpace(after) == "" {
		return before
	}
	return before + "\n" + strings.TrimLeft(after, "\n")
}

func newCommentEditorCommand(ctx context.Context, editor, path string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", editor+" "+quoteShellArgument(path))
}

func newSourceEditorCommand(
	ctx context.Context,
	editor, path string,
	lineNumber int,
) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		"sh",
		"-c",
		editor+" +"+strconv.Itoa(lineNumber)+" "+quoteShellArgument(path),
	)
}

func editableSuggestionLines(quotedDiffLines []string) []string {
	lines := make([]string, 0, len(quotedDiffLines))
	for _, line := range quotedDiffLines {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func suggestionMarkdownFence(suggestionLines []string) string {
	longest := 0
	for _, line := range suggestionLines {
		run := 0
		for _, char := range line {
			if char == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

func quoteShellArgument(argument string) string {
	return "'" + strings.ReplaceAll(argument, "'", "'\"'\"'") + "'"
}
