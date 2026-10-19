package editor

import (
	"fmt"
	"os"

	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func createCommentFile(body string, anchor comments.Anchor) (string, error) {
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
	if _, err := file.WriteString(commentDraft(body, anchor)); err != nil {
		return "", fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close comment file: %w", err)
	}
	succeeded = true
	return path, nil
}

func readCommentFile(path string, anchor comments.Anchor) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read comment file: %w", err)
	}
	return stripUnchangedSuggestion(string(body), anchor.QuotedLines), nil
}
