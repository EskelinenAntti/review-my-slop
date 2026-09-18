package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

// CreateCommentFile creates a private draft for the external editor.
func CreateCommentFile(body string, anchor comments.Anchor) (string, error) {
	state, err := stateDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(state, 0o700); err != nil {
		return "", fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chmod(state, 0o700); err != nil {
		return "", fmt.Errorf("secure state directory: %w", err)
	}
	file, err := os.CreateTemp(state, "comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(CommentDraft(body, anchor)); err != nil {
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

// ReadCommentFile reads and removes a completed editor draft.
func ReadCommentFile(path string, anchor comments.Anchor, editorErr error) (string, error) {
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

// CommentDraft combines the comment body and an optional suggestion block.
func CommentDraft(body string, anchor comments.Anchor) string {
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

// StripUnchangedSuggestion removes the untouched suggestion inserted by the
// application, leaving an agent-facing comment body.
func StripUnchangedSuggestion(body string, quoted []string) string {
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

func CommentCommand(editor, path string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" "+shellQuote(path))
}

func SourceCommand(editor, path string, line int) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" +"+strconv.Itoa(line)+" "+shellQuote(path))
}

func stateDir() (string, error) {
	root := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(root) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		root = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(root, "review-my-slop"), nil
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
