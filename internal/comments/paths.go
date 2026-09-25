package comments

import (
	"os"
	"path/filepath"
)

const appName = "review-my-slop"

func DataDir() (string, error) {
	return appDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

func appDir(environment, fallback string) (string, error) {
	join := filepath.Join
	if root := os.Getenv(environment); filepath.IsAbs(root) {
		return join(root, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", formatError("resolve user home directory: %w", err)
	}
	return join(home, fallback, appName), nil
}
