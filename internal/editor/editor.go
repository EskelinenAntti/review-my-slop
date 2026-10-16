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

func createCommentDraftFile(
	body string,
	anchor comments.Anchor,
) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	_, err = file.WriteString(formatCommentDraft(body, anchor))
	if err != nil {
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

func readCommentDraftFile(path string, anchor comments.Anchor) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return removeUneditedSuggestion(string(body), anchor.QuotedLines), nil
}

func formatCommentDraft(body string, anchor comments.Anchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := linesToSuggest(anchor.QuotedLines)
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	fence := quoteMarksForSuggestion(lines)
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

func removeUneditedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	lines := linesToSuggest(quoted)
	fence := quoteMarksForSuggestion(lines)
	var suggestion strings.Builder
	_, _ = suggestion.WriteString(fence)
	_, _ = suggestion.WriteString("suggestion\n")
	for _, line := range lines {
		_, _ = suggestion.WriteString(line)
		_ = suggestion.WriteByte('\n')
	}
	_, _ = suggestion.WriteString(fence)
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

func openDraftInEditor(ctx context.Context, editor, path string) *exec.Cmd {
	return exec.CommandContext(ctx, "sh", "-c", editor+" "+quoteForShell(path))
}

func openSourceInEditor(
	ctx context.Context,
	editor, path string,
	line int,
) *exec.Cmd {
	return exec.CommandContext(
		ctx,
		"sh",
		"-c",
		editor+" +"+strconv.Itoa(line)+" "+quoteForShell(path),
	)
}

func linesToSuggest(quoted []string) []string {
	lines := make([]string, 0, len(quoted))
	for _, line := range quoted {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func quoteMarksForSuggestion(lines []string) string {
	longest := 0
	for _, line := range lines {
		runCommand := 0
		for _, char := range line {
			if char == '`' {
				runCommand++
				longest = max(longest, runCommand)
			} else {
				runCommand = 0
			}
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}

func quoteForShell(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
