package comments

import (
	"path/filepath"
	"testing"
)

func TestDataDirectoryUsesXDGEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)

	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, appName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}

func TestDataDirectoryFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", appName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}

func TestRelativeXDGDataDirectoryIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative-data")

	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", appName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}
