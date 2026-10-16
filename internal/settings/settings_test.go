package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestLayoutSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review-my-slop", "ui.json")

	if enabled, err := readPreferencesFromFile(path); err != nil ||
		enabled.SideBySide {
		t.Fatalf("default layout = %+v, error = %v", enabled, err)
	}
	testutil.RequireNoError(
		t,
		writePreferencesToFile(path, Preferences{SideBySide: true}),
	)

	saved, err := readPreferencesFromFile(path)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(
		t,
		saved.SideBySide,
		true,
		"saved layout=%+v",
		saved,
	)
	testutil.RequireNoError(t, writePreferencesToFile(path, Preferences{}))

	if enabled, err := readPreferencesFromFile(path); err != nil ||
		enabled.SideBySide {
		t.Fatalf("updated layout = %+v, error = %v", enabled, err)
	}
}

func TestLoadSaveUserPreferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got, err := LoadPreferences(); err != nil || got.SideBySide {
		t.Fatalf("defaults=%+v err=%v", got, err)
	}
	testutil.RequireNoError(t, SavePreferences(Preferences{SideBySide: true}))

	if got, err := LoadPreferences(); err != nil || !got.SideBySide {
		t.Fatalf("saved=%+v err=%v", got, err)
	}
	path, err := preferencesFilePath()
	testutil.RequireNoError(t, err)

	info, err := os.Stat(path)
	testutil.RequireNoError(t, err)

	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("permissions=%o", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(entries), 1,
		"unexpected files after atomic save: %v", entries,
	)
	testutil.RequireValuesEqual(t, entries[0].Name(), "ui.json",
		"unexpected files after atomic save: %v", entries,
	)
}

func TestReadErrors(t *testing.T) {
	for _, test := range []struct{ name, data string }{
		{"invalid JSON", "{"},
		{"invalid preference type", `{"side_by_side":"yes"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ui.json")
			testutil.RequireNoError(
				t,
				os.WriteFile(path, []byte(test.data), 0600),
			)

			_, err := readPreferencesFromFile(path)
			assertPreferenceError(t, err, "decode UI settings")
		})
	}
	_, err := readPreferencesFromFile(t.TempDir())
	assertPreferenceError(t, err, "read UI settings")
}

func assertPreferenceError(t *testing.T, err error, message string) {
	t.Helper()
	testutil.RequireValuesEqual(
		t,
		err != nil,
		true,
		"expected %s error",
		message,
	)
	testutil.RequireValuesEqual(t, strings.Contains(err.Error(), message), true,
		"expected %s error, got %v", message, err,
	)
}
