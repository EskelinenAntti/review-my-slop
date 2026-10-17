package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsLayoutSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review-my-slop", "ui.json")

	enabled, err := readSettingsPreferences(path)
	if err != nil || enabled.SideBySide {
		t.Fatalf("default layout = %+v, error = %v", enabled, err)
	}
	requireNoError(
		t,
		writeUserSettingsFile(path, settingsPreferences{SideBySide: true}),
	)

	saved, err := readSettingsPreferences(path)
	requireNoError(t, err)
	requireEqual(t, saved.SideBySide, true, "saved layout=%+v", saved)
	requireNoError(t, writeUserSettingsFile(path, settingsPreferences{}))

	enabled, err = readSettingsPreferences(path)
	if err != nil || enabled.SideBySide {
		t.Fatalf("updated layout = %+v, error = %v", enabled, err)
	}
}

func TestSettingsLoadSaveUserPreferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := userSettingsPath()
	requireNoError(t, err)
	got, err := readSettingsPreferences(path)
	if err != nil || got.SideBySide {
		t.Fatalf("defaults=%+v err=%v", got, err)
	}
	requireNoError(t, writeUserSettingsFile(
		path, settingsPreferences{SideBySide: true},
	))

	got, err = readSettingsPreferences(path)
	if err != nil || !got.SideBySide {
		t.Fatalf("saved=%+v err=%v", got, err)
	}

	info, err := os.Stat(path)
	requireNoError(t, err)

	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("permissions=%o", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	requireNoError(t, err)
	requireEqual(t, len(entries), 1,
		"unexpected files after atomic save: %v", entries,
	)
	requireEqual(t, entries[0].Name(), "ui.json",
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
			requireNoError(t, os.WriteFile(path, []byte(test.data), 0600))

			_, err := readSettingsPreferences(path)
			requireSettingsReadError(t, err, "decode UI settings")
		})
	}
	_, err := readSettingsPreferences(t.TempDir())
	requireSettingsReadError(t, err, "read UI settings")
}

func requireSettingsReadError(t *testing.T, err error, message string) {
	t.Helper()
	requireEqual(t, err != nil, true, "expected %s error", message)
	requireEqual(t, strings.Contains(err.Error(), message), true,
		"expected %s error, got %v", message, err,
	)
}
