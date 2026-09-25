package ui

import (
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func CreateCommentFile(body string, anchor comments.Anchor) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", formatError("create comment file: %w", err)
	}
	path := file.Name()
	closeFile, remove := file.Close, os.Remove
	if _, err := file.WriteString(CommentDraft(body, anchor)); err != nil {
		_ = closeFile()
		_ = remove(path)
		return "", formatError("write comment file: %w", err)
	}
	if err := closeFile(); err != nil {
		_ = remove(path)
		return "", formatError("close comment file: %w", err)
	}
	return path, nil
}

func ReadCommentFile(path string, anchor comments.Anchor, editorErr error) (string, error) {
	defer os.Remove(path)
	if editorErr != nil {
		return "", formatError("editor: %w", editorErr)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", formatError("read comment file: %w", err)
	}
	return StripUnchangedSuggestion(string(body), anchor.QuotedLines), nil
}

func CommentDraft(body string, anchor comments.Anchor) string {
	quotedLines := anchor.QuotedLines
	if len(quotedLines) == 0 {
		return body
	}
	lines := suggestionLines(quotedLines)
	var draft strings.Builder
	writeString, writeByte := draft.WriteString, draft.WriteByte
	writeString(body)
	if body != "" && !strings.HasSuffix(body, "\n") {
		writeByte('\n')
	}
	writeByte('\n')
	writeString(suggestionBlock(lines))
	writeByte('\n')
	return draft.String()
}

func StripUnchangedSuggestion(body string, quoted []string) string {
	if len(quoted) == 0 {
		return body
	}
	lines := suggestionLines(quoted)
	suggestion := suggestionBlock(lines)
	start := strings.Index(body, suggestion)
	if start < 0 {
		return body
	}
	end := start + len(suggestion)
	before := strings.TrimRight(body[:start], "\n")
	after := body[end:]
	if trimSpace(after) == "" {
		return before
	}
	return before + "\n" + strings.TrimLeft(after, "\n")
}

func suggestionBlock(lines []string) string {
	fence := contextFence(lines)
	var suggestion strings.Builder
	writeString, writeByte := suggestion.WriteString, suggestion.WriteByte
	writeString(fence)
	writeString("suggestion\n")
	for _, line := range lines {
		writeString(line)
		writeByte('\n')
	}
	writeString(fence)
	return suggestion.String()
}

func CommentCommand(editor, path string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" "+shellQuote(path))
}

func SourceCommand(editor, path string, line int) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" +"+strconv.Itoa(line)+" "+shellQuote(path))
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
	return repeat("`", max(3, longest+1))
}

func shellQuote(value string) string {
	return "'" + replaceAll(value, "'", "'\"'\"'") + "'"
}
