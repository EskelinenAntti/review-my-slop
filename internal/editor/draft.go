package editor

import (
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func commentDraft(body string, anchor comments.Anchor) string {
	if len(anchor.QuotedLines) == 0 {
		return body
	}
	var draft strings.Builder
	_, _ = draft.WriteString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		_ = draft.WriteByte('\n')
	}
	_ = draft.WriteByte('\n')
	_, _ = draft.WriteString(suggestionBlock(anchor.QuotedLines))
	_ = draft.WriteByte('\n')
	return draft.String()
}

func stripUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	suggestion := suggestionBlock(quoted)
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

// suggestionBlock renders only the new version of the quoted diff, choosing a
// fence that cannot be closed by retained source text.
func suggestionBlock(quoted []string) string {
	var content strings.Builder
	for _, line := range quoted {
		if line != "" && line[0] != '-' {
			_, _ = content.WriteString(line[1:])
			_ = content.WriteByte('\n')
		}
	}
	fence := contextFence(content.String())
	return fence + "suggestion\n" + content.String() + fence
}

func contextFence(content string) string {
	longest, run := 0, 0
	for _, char := range content {
		if char == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(3, longest+1))
}
