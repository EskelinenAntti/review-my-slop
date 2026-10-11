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

	{
		enabled, err := readPreferences(path)
		testutil.Requiref(t, err == nil && !enabled.SideBySide, "default layout = %+v, error = %v", enabled, err)
	}

	testutil.NoError(t, writePreferences(path, Preferences{SideBySide: true}))

	{
		enabled, err := readPreferences(path)
		testutil.Requiref(t, err == nil && enabled.SideBySide, "saved layout = %+v, error = %v", enabled, err)
	}

	testutil.NoError(t, writePreferences(path, Preferences{}))

	{
		enabled, err := readPreferences(path)
		testutil.Requiref(t, err == nil && !enabled.SideBySide, "updated layout = %+v, error = %v", enabled, err)
	}
}

func TestLoadSaveUserPreferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	{
		got, err := Load()
		testutil.Requiref(t, err == nil && !got.SideBySide, "defaults=%+v err=%v", got, err)
	}

	testutil.NoError(t, Save(Preferences{SideBySide: true}))

	{
		got, err := Load()
		testutil.Requiref(t, err == nil && got.SideBySide, "saved=%+v err=%v", got, err)
	}
	path, err := settingsPath()
	testutil.Require(t, err == nil, err)
	info, err := os.Stat(path)
	testutil.Require(t, err == nil, err)
	{
		got := info.Mode().Perm()
		testutil.Requiref(t, got == 0600, "permissions=%o", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(entries) == 1 && entries[0].Name() == "ui.json", "unexpected files after atomic save: %v", entries)
}

func TestReadErrors(t *testing.T) {
	for _, test := range []struct{ name, data string }{
		{"invalid JSON", "{"},
		{"invalid preference type", `{"side_by_side":"yes"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ui.json")

			testutil.NoError(t, os.WriteFile(path, []byte(test.data), 0600))

			{
				_, err := readPreferences(path)
				testutil.Requiref(t, err != nil && strings.Contains(err.Error(), "decode UI settings"), "expected decode error, got %v", err)
			}
		})
	}
	{
		_, err := readPreferences(t.TempDir())
		testutil.Requiref(t, err != nil && strings.Contains(err.Error(), "read UI settings"), "expected read error, got %v", err)
	}
}
