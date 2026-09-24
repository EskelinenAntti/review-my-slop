package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type layoutSettings struct {
	SideBySide bool `json:"side_by_side"`
}

func loadLayoutSettings() (bool, error) {
	path, err := layoutSettingsPath()
	if err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read UI settings: %w", err)
	}
	var settings layoutSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return false, fmt.Errorf("decode UI settings: %w", err)
	}
	return settings.SideBySide, nil
}

func saveLayoutSettings(enabled bool) error {
	path, err := layoutSettingsPath()
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create UI settings directory: %w", err)
	}
	data, err := json.Marshal(layoutSettings{enabled})
	if err != nil {
		return fmt.Errorf("encode UI settings: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(directory, "ui-*.tmp")
	if err != nil {
		return fmt.Errorf("create UI settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	defer temporary.Close()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure UI settings file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
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

func layoutSettingsPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve UI settings directory: %w", err)
	}
	return filepath.Join(config, "review-my-slop", "ui.json"), nil
}
