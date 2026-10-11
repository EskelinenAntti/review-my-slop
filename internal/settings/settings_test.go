package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayoutSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review-my-slop", "ui.json")

	{
		enabled, err := readPreferences(path)
		requireEqual(
			t,
			err == nil && !enabled.SideBySide, true,
			"default layout = %+v, error = %v",
			enabled,
			err,
		)
	}
	{
		err := writePreferences(path, Preferences{SideBySide: true})
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		enabled, err := readPreferences(path)
		requireEqual(
			t,
			err == nil && enabled.SideBySide, true,
			"saved layout = %+v, error = %v",
			enabled,
			err,
		)
	}
	{
		err := writePreferences(path, Preferences{})
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		enabled, err := readPreferences(path)
		requireEqual(
			t,
			err == nil && !enabled.SideBySide, true,
			"updated layout = %+v, error = %v",
			enabled,
			err,
		)
	}
}

func TestLoadSaveUserPreferences(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	{
		got, err := Load()
		requireEqual(
			t,
			err == nil && !got.SideBySide, true,
			"defaults=%+v err=%v",
			got,
			err,
		)
	}
	{
		err := Save(Preferences{SideBySide: true})
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		got, err := Load()
		requireEqual(
			t,
			err == nil && got.SideBySide, true,
			"saved=%+v err=%v",
			got,
			err,
		)
	}
	path, err := settingsPath()
	requireEqual(t, err == nil, true, fixtureV, err)
	info, err := os.Stat(path)
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		got := info.Mode().Perm()
		requireEqual(t, got, fixtureNumber0600, "permissions=%o", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(entries) == 1 && entries[0].Name() == "ui.json", true,
		"unexpected files after atomic save: %v",
		entries,
	)
}

func TestReadErrors(t *testing.T) {
	for _, test := range []struct{ name, data string }{
		{"invalid JSON", "{"},
		{"invalid preference type", `{"side_by_side":"yes"}`},
	} {
		t.Run(
			test.name,
			func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "ui.json")
				{
					err := os.WriteFile(
						path,
						[]byte(test.data),
						fixtureNumber0600,
					)
					requireEqual(t, err == nil, true, fixtureV, err)
				}
				{
					_, err := readPreferences(path)
					requireEqual(
						t,
						err != nil &&
							strings.Contains(
								err.Error(),
								"decode UI settings",
							),
						true,
						"expected decode error, got %v",
						err,
					)
				}
			},
		)
	}
	{
		_, err := readPreferences(t.TempDir())
		requireEqual(
			t,
			err != nil && strings.Contains(err.Error(), "read UI settings"),
			true,
			"expected read error, got %v",
			err,
		)
	}
}
