package ui

import (
	"encoding/json"
	"errors"
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
		return false, formatError("read UI settings: %w", err)
	}
	var settings layoutSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return false, formatError("decode UI settings: %w", err)
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
		return formatError("create UI settings directory: %w", err)
	}
	data, err := json.Marshal(layoutSettings{SideBySide: enabled})
	if err != nil {
		return formatError("encode UI settings: %w", err)
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(directory, "ui-*.tmp")
	if err != nil {
		return formatError("create UI settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	closeTemporary := temporary.Close
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		closeTemporary()
		return formatError("secure UI settings file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		closeTemporary()
		return formatError("write UI settings: %w", err)
	}
	if err := closeTemporary(); err != nil {
		return formatError("close UI settings: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return formatError("replace UI settings: %w", err)
	}
	return nil
}

func layoutSettingsPath() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", formatError("resolve UI settings directory: %w", err)
	}
	return filepath.Join(config, "review-my-slop", "ui.json"), nil
}
