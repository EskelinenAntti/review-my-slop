package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eskelinenantti/review-my-slop/internal/appinfo"
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
	var preferences Preferences
	if err := json.Unmarshal(data, &preferences); err != nil {
		return Preferences{}, fmt.Errorf("decode UI settings: %w", err)
	}
	return preferences, nil
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
	data, err := json.Marshal(preferences)
	if err != nil {
		return fmt.Errorf("encode UI settings: %w", err)
	}
	return replaceSettingsFile(path, append(data, '\n'))
}

func settingsPath() (string, error) {
	config, err := appinfo.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve UI settings directory: %w", err)
	}
	return filepath.Join(config, "ui.json"), nil
}
