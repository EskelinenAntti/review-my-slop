package editor

import (
	"fmt"
	"os"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func writeCommentDraftFile(
	body string,
	anchor comments.Anchor,
) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	draft := formatCommentDraft(body, anchor)
	if _, err := file.WriteString(draft); err != nil {
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

func readEditedCommentDraft(
	path string,
	anchor comments.Anchor,
) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return removeUnchangedSuggestion(string(body), anchor.QuotedLines), nil
}

func formatCommentDraft(body string, anchor comments.Anchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	lines := replacementLines(anchor.QuotedLines)
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	fence := suggestionFence(lines)
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

func removeUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	lines := replacementLines(quoted)
	fence := suggestionFence(lines)
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

func replacementLines(quoted []string) []string {
	lines := make([]string, 0, len(quoted))
	for _, line := range quoted {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func suggestionFence(lines []string) string {
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
