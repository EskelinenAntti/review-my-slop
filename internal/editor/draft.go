package editor

import (
	"fmt"
	"os"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
)

func createCommentFile(body string, quoted []string) (string, error) {
	file, err := os.CreateTemp("", appinfo.Name+"-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	succeeded := false
	defer func() {
		if succeeded {
			return
		}
		_ = file.Close()
		_ = os.Remove(path)
	}()
	if _, err := file.WriteString(commentDraft(body, quoted)); err != nil {
		return "", fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close comment file: %w", err)
	}
	succeeded = true
	return path, nil
}

func readCommentFile(path string, quoted []string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return stripUnchangedSuggestion(string(body), quoted), nil
}

func commentDraft(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	suggestion := suggestionBlock(suggestionLines(quoted))
	return body + "\n" + suggestion + "\n"
}

func stripUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	suggestion := suggestionBlock(suggestionLines(quoted))
	before, after, found := strings.Cut(body, suggestion)
	if !found {
		return body
	}
	before = strings.TrimRight(before, "\n")
	if strings.TrimSpace(after) == "" {
		return before
	}
	return before + "\n" + strings.TrimLeft(after, "\n")
}

func suggestionBlock(lines []string) string {
	fence := contextFence(lines)
	var block strings.Builder
	_, _ = block.WriteString(fence)
	_, _ = block.WriteString("suggestion\n")
	for _, line := range lines {
		_, _ = block.WriteString(line)
		_ = block.WriteByte('\n')
	}
	_, _ = block.WriteString(fence)
	return block.String()
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
