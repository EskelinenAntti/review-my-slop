package comments

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "review-my-slop"

func DataDir() (string, error) {
	root := os.Getenv("XDG_DATA_HOME")
	if filepath.IsAbs(root) {
		return filepath.Join(root, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", appName), nil
}

func DefaultPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "comments.db"), nil
}
