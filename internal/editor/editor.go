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

func createCommentFile(body string, anchor comments.Anchor) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(commentDraft(body, anchor)); err != nil {
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

func readCommentFile(path string, anchor comments.Anchor) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return stripUnchangedSuggestion(string(body), anchor.QuotedLines), nil
}

func commentDraft(body string, anchor comments.Anchor) string {
	if len(anchor.QuotedLines) == zero {
		return body
	}
	lines := suggestionLines(anchor.QuotedLines)
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, newline) {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	fence := contextFence(lines)
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

func stripUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == zero {
		return body
	}
	lines := suggestionLines(quoted)
	fence := contextFence(lines)
	var suggestion strings.Builder
	_, _ = suggestion.WriteString(fence)
	_, _ = suggestion.WriteString("suggestion\n")
	for _, line := range lines {
		_, _ = suggestion.WriteString(line)
		_ = suggestion.WriteByte('\n')
	}
	_, _ = suggestion.WriteString(fence)
	start := strings.Index(body, suggestion.String())
	if start < zero {
		return body
	}
	end := start + suggestion.Len()
	before := strings.TrimRight(body[:start], newline)
	after := body[end:]
	if strings.TrimSpace(after) == "" {
		return before
	}
	return before + newline + strings.TrimLeft(after, newline)
}

func commentCommand(ctx context.Context, editor, path string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", editor+" "+shellQuote(path))
}

func sourceCommand(
	ctx context.Context,
	editor,
	path string,
	line int,
) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		"sh",
		"-c",
		editor+" +"+strconv.Itoa(line)+" "+shellQuote(path),
	)
}

func suggestionLines(quoted []string) []string {
	lines := make([]string, zero, len(quoted))
	for _, line := range quoted {
		if line != "" && line[zero] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func contextFence(lines []string) string {
	longest := zero
	for _, line := range lines {
		run := zero
		for _, char := range line {
			if char == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = zero
			}
		}
	}
	return strings.Repeat("`", max(three, longest+1))
}

func shellQuote(value string) string {
	return fixtureText + strings.ReplaceAll(value, fixtureText, "'\"'\"'") +
		fixtureText
}
