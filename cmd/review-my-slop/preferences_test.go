package main

import (
	json "encoding/json"
	errors "errors"
	fmt "fmt"
	os "os"
	filepath "path/filepath"
	strings "strings"
	testing "testing"
)

func TestSettingsLayoutSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review-my-slop", "ui.json")

	if enabled, err := (func(path string) (settingsPreferences, error) {
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
	})(path); err != nil || enabled.SideBySide {
		t.Fatalf("default layout = %+v, error = %v", enabled, err)
	}
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, (func(path string, preferences settingsPreferences) error {
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
	})(path, settingsPreferences{SideBySide: true}))

	saved, err := (func(path string) (settingsPreferences, error) {
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
	})(path)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want bool, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, saved.SideBySide, true, "saved layout=%+v", saved)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, (func(path string, preferences settingsPreferences) error {
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
	})(path, settingsPreferences{}))

	if enabled, err := (func(path string) (settingsPreferences, error) {
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
	})(path); err != nil || enabled.SideBySide {
		t.Fatalf("updated layout = %+v, error = %v", enabled, err)
	}
}

func TestSettingsLoadSaveUserPreferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got, err := (func() (settingsPreferences, error) {
		path, err := (func() (string, error) {
			config, err := os.UserConfigDir()
			if err != nil {
				return "", fmt.Errorf("resolve UI settings directory: %w", err)
			}
			return filepath.Join(config, "review-my-slop", "ui.json"), nil
		})()
		if err != nil {
			return settingsPreferences{}, err
		}
		return (func(path string) (settingsPreferences, error) {
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
		})(path)
	})(); err != nil || got.SideBySide {
		t.Fatalf("defaults=%+v err=%v", got, err)
	}
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, (func(preferences settingsPreferences) error {
		path, err := (func() (string, error) {
			config, err := os.UserConfigDir()
			if err != nil {
				return "", fmt.Errorf("resolve UI settings directory: %w", err)
			}
			return filepath.Join(config, "review-my-slop", "ui.json"), nil
		})()
		if err != nil {
			return err
		}
		return (func(path string, preferences settingsPreferences) error {
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
		})(path, preferences)
	})(settingsPreferences{SideBySide: true}))

	if got, err := (func() (settingsPreferences, error) {
		path, err := (func() (string, error) {
			config, err := os.UserConfigDir()
			if err != nil {
				return "", fmt.Errorf("resolve UI settings directory: %w", err)
			}
			return filepath.Join(config, "review-my-slop", "ui.json"), nil
		})()
		if err != nil {
			return settingsPreferences{}, err
		}
		return (func(path string) (settingsPreferences, error) {
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
		})(path)
	})(); err != nil || !got.SideBySide {
		t.Fatalf("saved=%+v err=%v", got, err)
	}
	path, err := (func() (string, error) {
		config, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("resolve UI settings directory: %w", err)
		}
		return filepath.Join(config, "review-my-slop", "ui.json"), nil
	})()
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	info, err := os.Stat(path)
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)

	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("permissions=%o", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	(func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	})(t, err)
	(func(t *testing.T, got, want int, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, len(entries), 1,
		"unexpected files after atomic save: %v", entries,
	)
	(func(t *testing.T, got, want string, format string, args ...any) {
		t.Helper()
		if got != want {
			t.Fatalf(format, args...)
		}
	})(t, entries[0].Name(), "ui.json",
		"unexpected files after atomic save: %v", entries,
	)
}

func TestSettingsReadErrors(t *testing.T) {
	for _, test := range []struct{ name, data string }{
		{"invalid JSON", "{"},
		{"invalid preference type", `{"side_by_side":"yes"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ui.json")
			(func(t *testing.T, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
			})(t, os.WriteFile(path, []byte(test.data), 0600))

			_, err := (func(path string) (settingsPreferences, error) {
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
			})(path)
			(func(t *testing.T, err error, message string) {
				t.Helper()
				(func(t *testing.T, got, want bool, format string, args ...any) {
					t.Helper()
					if got != want {
						t.Fatalf(format, args...)
					}
				})(t, err != nil, true, "expected %s error", message)
				(func(t *testing.T, got, want bool, format string, args ...any) {
					t.Helper()
					if got != want {
						t.Fatalf(format, args...)
					}
				})(t, strings.Contains(err.Error(), message), true,
					"expected %s error, got %v", message, err,
				)
			})(t, err, "decode UI settings")
		})
	}
	_, err := (func(path string) (settingsPreferences, error) {
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
	})(t.TempDir())
	(func(t *testing.T, err error, message string) {
		t.Helper()
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, err != nil, true, "expected %s error", message)
		(func(t *testing.T, got, want bool, format string, args ...any) {
			t.Helper()
			if got != want {
				t.Fatalf(format, args...)
			}
		})(t, strings.Contains(err.Error(), message), true,
			"expected %s error, got %v", message, err,
		)
	})(t, err, "read UI settings")
}
