package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func CreateCommentFile(body string, anchor comments.Anchor) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(comments.Draft(body, anchor)); err != nil {
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

func ReadCommentFile(path string, anchor comments.Anchor, editorErr error) (string, error) {
	defer os.Remove(path)
	if editorErr != nil {
		return "", fmt.Errorf("editor: %w", editorErr)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return comments.StripUnchangedSuggestion(string(body), anchor), nil
}

func CommentCommand(editor, path string) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" "+shellQuote(path))
}

func SourceCommand(editor, path string, line int) *exec.Cmd {
	return exec.Command("sh", "-c", editor+" +"+strconv.Itoa(line)+" "+shellQuote(path))
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
