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

	if enabled, err := readPreferences(path); err != nil || enabled.SideBySide {
		t.Fatalf("default layout = %+v, error = %v", enabled, err)
	}
	testutil.NoError(t, writePreferences(path, Preferences{SideBySide: true}))

	saved, err := readPreferences(path)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, saved.SideBySide, true, "saved layout=%+v", saved)
	testutil.NoError(t, writePreferences(path, Preferences{}))

	if enabled, err := readPreferences(path); err != nil || enabled.SideBySide {
		t.Fatalf("updated layout = %+v, error = %v", enabled, err)
	}
}

func TestLoadSaveUserPreferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if got, err := Load(); err != nil || got.SideBySide {
		t.Fatalf("defaults=%+v err=%v", got, err)
	}
	testutil.NoError(t, Save(Preferences{SideBySide: true}))

	if got, err := Load(); err != nil || !got.SideBySide {
		t.Fatalf("saved=%+v err=%v", got, err)
	}
	path, err := settingsPath()
	testutil.NoError(t, err)

	info, err := os.Stat(path)
	testutil.NoError(t, err)

	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("permissions=%o", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(entries), 1,
		"unexpected files after atomic save: %v", entries,
	)
	testutil.RequireEqual(t, entries[0].Name(), "ui.json",
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
			testutil.NoError(t, os.WriteFile(path, []byte(test.data), 0600))

			_, err := readPreferences(path)
			assertPreferenceError(t, err, "decode UI settings")
		})
	}
	_, err := readPreferences(t.TempDir())
	assertPreferenceError(t, err, "read UI settings")
}

func assertPreferenceError(t *testing.T, err error, message string) {
	t.Helper()
	testutil.RequireEqual(t, err != nil, true, "expected %s error", message)
	testutil.RequireEqual(t, strings.Contains(err.Error(), message), true,
		"expected %s error, got %v", message, err,
	)
}
