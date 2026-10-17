package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

type settingsPreferences struct {
	SideBySide bool `json:"side_by_side"`
}

const commentsAppName = "review-my-slop"

func userCommentsStore() (feedback.Store, error) {
	dataDirectory, err := userDataDirectory()
	if err != nil {
		return feedback.Store{}, err
	}
	return feedback.Store{
		Path: filepath.Join(dataDirectory, "comments.db"),
	}, nil
}

func userDataDirectory() (string, error) {
	if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, commentsAppName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", commentsAppName), nil
}

func userSettingsPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve UI settings directory: %w", err)
	}
	return filepath.Join(config, "review-my-slop", "ui.json"), nil
}

func readSettingsPreferences(path string) (settingsPreferences, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settingsPreferences{}, nil
	}
	if err != nil {
		return settingsPreferences{}, fmt.Errorf("read UI settings: %w", err)
	}
	var settings settingsPreferences
	if err := json.Unmarshal(data, &settings); err != nil {
		return settingsPreferences{}, fmt.Errorf("decode UI settings: %w", err)
	}
	return settings, nil
}

func writeUserSettingsFile(path string, preferences settingsPreferences) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create UI settings directory: %w", err)
	}
	data, err := json.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("encode UI settings: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), "ui-*.tmp")
	if err != nil {
		return fmt.Errorf("create UI settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure UI settings file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write UI settings: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close UI settings: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace UI settings: %w", err)
	}
	return nil
}

func makeInitialLayout(
	preferences settingsPreferences,
	dimensions size,
) initialLayout {
	return initialLayout{
		SideBySide: preferences.SideBySide,
		SaveSideBySide: func(enabled bool) error {
			return saveLayoutPreference(enabled)
		},
		size: dimensions,
	}
}

func saveLayoutPreference(enabled bool) error {
	path, err := userSettingsPath()
	if err != nil {
		return err
	}
	return writeUserSettingsFile(path, settingsPreferences{SideBySide: enabled})
}
