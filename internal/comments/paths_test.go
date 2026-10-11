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

func TestDefaultStoreUsesDataDirectory(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	want := filepath.Join(root, appName, "comments.db")
	path, err := DefaultPath()
	if err != nil || path != want {
		t.Fatalf("path=%q error=%v", path, err)
	}
	store, err := OpenDefault()
	if err != nil || store.Path != want {
		t.Fatalf("store=%+v error=%v", store, err)
	}
}
