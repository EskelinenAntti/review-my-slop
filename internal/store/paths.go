package store

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "review-my-slop"

func DataDir() (string, error) {
	return appDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

func StateDir() (string, error) {
	return appDir("XDG_STATE_HOME", filepath.Join(".local", "state"))
}

func DefaultPath() (string, error) {
	data, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(data, "inbox.db"), nil
}

func appDir(environment, fallback string) (string, error) {
	if root := os.Getenv(environment); filepath.IsAbs(root) {
		return filepath.Join(root, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, fallback, appName), nil
}
