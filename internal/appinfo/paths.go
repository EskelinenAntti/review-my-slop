// Package appinfo provides the application identity and user storage paths.
package appinfo

import (
	"fmt"
	"os"
	"path/filepath"
)

// Name is the application name used for user-facing and storage paths.
const Name = "review-my-slop"

// DataDir returns the application data directory, respecting XDG_DATA_HOME.
func DataDir() (string, error) {
	if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, Name), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", Name), nil
}

// ConfigDir returns the application configuration directory.
func ConfigDir() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, Name), nil
}
