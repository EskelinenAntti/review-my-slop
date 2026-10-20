package editor

import (
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func commentDraft(body string, anchor comments.Anchor) string {
	suggestion := suggestionBlock(anchor.QuotedLines)
	if suggestion == "" {
		return body
	}
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	_, _ = draft.WriteString(suggestion)
	_ = draft.WriteByte('\n')
	return draft.String()
}

func stripUnchangedSuggestion(body string, quoted []string) string {
	suggestion := suggestionBlock(quoted)
	if suggestion == "" {
		return body
	}
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

// suggestionBlock turns quoted diff lines into an editable new-side suggestion.
// A deletion-only quote still produces an empty block; no quote produces none.
func suggestionBlock(quoted []string) string {
	if len(quoted) == 0 {
		return ""
	}
	lines := make([]string, 0, len(quoted))
	for _, line := range quoted {
		if line != "" && line[0] != '-' {
			lines = append(lines, line[1:])
		}
	}
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
