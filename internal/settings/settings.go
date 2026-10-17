package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Preferences contains the persisted UI preferences.
type Preferences struct {
	SideBySide bool `json:"side_by_side"`
}

// Load reads preferences from the user configuration directory. Missing files
// use defaults.
func Load() (Preferences, error) {
	path, err := settingsPath()
	if err != nil {
		return Preferences{}, err
	}
	return readPreferences(path)
}

func readPreferences(path string) (Preferences, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Preferences{}, nil
	}
	if err != nil {
		return Preferences{}, fmt.Errorf("read UI settings: %w", err)
	}
	var settings Preferences
	if err := json.Unmarshal(data, &settings); err != nil {
		return Preferences{}, fmt.Errorf("decode UI settings: %w", err)
	}
	return settings, nil
}

// Save atomically replaces the user preferences file.
func Save(preferences Preferences) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	return writePreferences(path, preferences)
}

func writePreferences(path string, preferences Preferences) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create UI settings directory: %w", err)
	}
	data, err := encodePreferences(preferences)
	if err != nil {
		return err
	}
	return replacePreferencesFile(path, data)
}

func encodePreferences(preferences Preferences) ([]byte, error) {
	data, err := json.Marshal(preferences)
	if err != nil {
		return nil, fmt.Errorf("encode UI settings: %w", err)
	}
	return append(data, '\n'), nil
}

func replacePreferencesFile(path string, data []byte) error {
	temporary, err := createSettingsFile(filepath.Dir(path))
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := writeAndCloseSettings(temporary, data); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace UI settings: %w", err)
	}
	return nil
}

func createSettingsFile(directory string) (*os.File, error) {
	temporary, err := os.CreateTemp(directory, "ui-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create UI settings file: %w", err)
	}
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		_ = os.Remove(temporary.Name())
		return nil, fmt.Errorf("secure UI settings file: %w", err)
	}
	return temporary, nil
}

func writeAndCloseSettings(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write UI settings: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close UI settings: %w", err)
	}
	return nil
}

func settingsPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve UI settings directory: %w", err)
	}
	return filepath.Join(config, "review-my-slop", "ui.json"), nil
}
