package ui

import "testing"

func TestLayoutSettingsRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	if enabled, err := loadLayoutSettings(); err != nil || enabled {
		t.Fatalf("default layout = %v, error = %v", enabled, err)
	}
	if err := saveLayoutSettings(true); err != nil {
		t.Fatal(err)
	}
	if enabled, err := loadLayoutSettings(); err != nil || !enabled {
		t.Fatalf("saved layout = %v, error = %v", enabled, err)
	}
	if err := saveLayoutSettings(false); err != nil {
		t.Fatal(err)
	}
	if enabled, err := loadLayoutSettings(); err != nil || enabled {
		t.Fatalf("updated layout = %v, error = %v", enabled, err)
	}
}
