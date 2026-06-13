package xdg

import (
	"path/filepath"
	"testing"
)

func TestDirectoriesUseXDGEnvironment(t *testing.T) {
	root := t.TempDir()
	stateRoot := filepath.Join(root, "state")
	dataRoot := filepath.Join(root, "data")
	t.Setenv("XDG_STATE_HOME", stateRoot)
	t.Setenv("XDG_DATA_HOME", dataRoot)

	state, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(stateRoot, appName); state != want {
		t.Fatalf("state directory = %q, want %q", state, want)
	}

	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dataRoot, appName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}

func TestDirectoriesFallBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")

	state, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "state", appName); state != want {
		t.Fatalf("state directory = %q, want %q", state, want)
	}

	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", appName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}

func TestRelativeXDGDirectoriesAreIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "relative-state")
	t.Setenv("XDG_DATA_HOME", "relative-data")

	state, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "state", appName); state != want {
		t.Fatalf("state directory = %q, want %q", state, want)
	}

	data, err := DataDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", appName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}
