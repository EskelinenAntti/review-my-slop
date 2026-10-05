package comments

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "review-my-slop"

func DataDir() (string, error) {
	if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", appName), nil
}
