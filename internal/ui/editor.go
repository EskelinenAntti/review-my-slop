package ui

import (
	"fmt"
	"os"
	"strings"
)

func CreateCommentFile(body string, anchor reviewAnchor) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path, draft := file.Name(), body
	if len(anchor.QuotedLines) > 0 {
		separator := "\n"
		if body != "" && !strings.HasSuffix(body, "\n") {
			separator = "\n\n"
		}
		draft = body + separator + suggestionBlock(suggestionLines(anchor.QuotedLines)) + "\n"
	}
	if _, err := file.WriteString(draft); err != nil {
		file.Close()
		os.Remove(path)
		return "", fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close comment file: %w", err)
	}
	return path, nil
}

func ReadCommentFile(path string, anchor reviewAnchor, editorErr error) (string, error) {
	defer os.Remove(path)
	if editorErr != nil {
		return "", fmt.Errorf("editor: %w", editorErr)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return StripUnchangedSuggestion(string(body), anchor.QuotedLines), nil
}

func StripUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	lines := suggestionLines(quoted)
	suggestion := suggestionBlock(lines)
	before, after, found := strings.Cut(body, suggestion)
	if found {
		before = strings.TrimRight(before, "\n")
		if strings.TrimSpace(after) == "" {
			return before
		}
		return before + "\n" + strings.TrimLeft(after, "\n")
	}
	return body
}

func suggestionLines(quoted []string) []string {
	lines := []string{}
	for _, line := range quoted {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
	return lines
}

func suggestionBlock(lines []string) string {
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
	fence := strings.Repeat("`", max(3, longest+1))
	suggestion := fence + "suggestion\n" + strings.Join(lines, "\n")
	if len(lines) > 0 {
		suggestion += "\n"
	}
	return suggestion + fence
}
