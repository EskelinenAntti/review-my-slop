package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLayoutSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review-my-slop", "ui.json")

	if enabled, err := readPreferences(path); err != nil || enabled.SideBySide {
		t.Fatalf("default layout = %+v, error = %v", enabled, err)
	}
	if err := writePreferences(path, Preferences{SideBySide: true}); err != nil {
		t.Fatal(err)
	}
	if enabled, err := readPreferences(path); err != nil || !enabled.SideBySide {
		t.Fatalf("saved layout = %+v, error = %v", enabled, err)
	}
	if err := writePreferences(path, Preferences{}); err != nil {
		t.Fatal(err)
	}
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
	if err := Save(Preferences{SideBySide: true}); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(); err != nil || !got.SideBySide {
		t.Fatalf("saved=%+v err=%v", got, err)
	}
	path, err := settingsPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("permissions=%o", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "ui.json" {
		t.Fatalf("unexpected files after atomic save: %v", entries)
	}
}

func TestReadErrors(t *testing.T) {
	for _, test := range []struct{ name, data string }{
		{"invalid JSON", "{"},
		{"invalid preference type", `{"side_by_side":"yes"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ui.json")
			if err := os.WriteFile(path, []byte(test.data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readPreferences(path); err == nil || !strings.Contains(err.Error(), "decode UI settings") {
				t.Fatalf("expected decode error, got %v", err)
			}
		})
	}
	if _, err := readPreferences(t.TempDir()); err == nil || !strings.Contains(err.Error(), "read UI settings") {
		t.Fatalf("expected read error, got %v", err)
	}
}
