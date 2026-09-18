package xdg

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	appName           = "review-my-slop"
	stateHomeVariable = "XDG_STATE_HOME"
	dataHomeVariable  = "XDG_DATA_HOME"
	stateHomeFallback = ".local/state"
	dataHomeFallback  = ".local/share"
)

func StateDir() (string, error) {
	return appDir(stateHomeVariable, stateHomeFallback)
}

func DataDir() (string, error) {
	return appDir(dataHomeVariable, dataHomeFallback)
}

func appDir(environment, fallback string) (string, error) {
	root := os.Getenv(environment)
	if !filepath.IsAbs(root) {
		// XDG base directories are only valid when absolute. Relative or empty
		// values use the conventional directory below the user's home.
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		root = filepath.Join(home, fallback)
	}
	return filepath.Join(root, appName), nil
}
