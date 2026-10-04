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
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := suggestionLines(anchor.QuotedLines)
	var draft strings.Builder
	draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		draft.WriteByte('\n')
	}
	draft.WriteByte('\n')
	fence := contextFence(lines)
	draft.WriteString(fence)
	draft.WriteString("suggestion\n")
	for _, line := range lines {
		draft.WriteString(line)
		draft.WriteByte('\n')
	}
	draft.WriteString(fence)
	draft.WriteByte('\n')
	return draft.String()
}

func stripUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	lines := suggestionLines(quoted)
	fence := contextFence(lines)
	var suggestion strings.Builder
	suggestion.WriteString(fence)
	suggestion.WriteString("suggestion\n")
	for _, line := range lines {
		suggestion.WriteString(line)
		suggestion.WriteByte('\n')
	}
	suggestion.WriteString(fence)
	start := strings.Index(body, suggestion.String())
	if start < 0 {
		return body
	}
	end := start + suggestion.Len()
	before := strings.TrimRight(body[:start], "\n")
	after := body[end:]
	if strings.TrimSpace(after) == "" {
		return before
	}
	return before + "\n" + strings.TrimLeft(after, "\n")
}

func commentCommand(ctx context.Context, editor, path string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", editor+" "+shellQuote(path))
}

func sourceCommand(ctx context.Context, editor, path string, line int) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", editor+" +"+strconv.Itoa(line)+" "+shellQuote(path))
}

func suggestionLines(quoted []string) []string {
	lines := make([]string, 0, len(quoted))
	for _, line := range quoted {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func contextFence(lines []string) string {
	longest := 0
	for _, line := range lines {
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
