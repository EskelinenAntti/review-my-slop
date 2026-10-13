package comments

import (
	"fmt"
	"os"
	"path/filepath"
)

const appName = "review-my-slop"

// ApplicationDataDir returns the application data directory, respecting
// XDG_DATA_HOME.
func ApplicationDataDir() (string, error) {
	return resolveApplicationDataDir(
		"XDG_DATA_HOME",
		filepath.Join(".local", "share"),
	)
}

func resolveApplicationDataDir(
	dataHomeEnvVar, homeRelativeDataDir string,
) (string, error) {
	if root := os.Getenv(dataHomeEnvVar); filepath.IsAbs(root) {
		return filepath.Join(root, appName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, homeRelativeDataDir, appName), nil
}
