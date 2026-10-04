package ui

import (
	"path/filepath"
	"testing"
)

func TestLayoutSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "review-my-slop", "ui.json")

	if enabled, err := readLayoutSettings(path); err != nil || enabled {
		t.Fatalf("default layout = %v, error = %v", enabled, err)
	}
	if err := writeLayoutSettings(path, true); err != nil {
		t.Fatal(err)
	}
	if enabled, err := readLayoutSettings(path); err != nil || !enabled {
		t.Fatalf("saved layout = %v, error = %v", enabled, err)
	}
	if err := writeLayoutSettings(path, false); err != nil {
		t.Fatal(err)
	}
	if enabled, err := readLayoutSettings(path); err != nil || enabled {
		t.Fatalf("updated layout = %v, error = %v", enabled, err)
	}
}
