package xdg

import (
	"path/filepath"
	"testing"
)

func TestDirectoriesUseAbsoluteXDGPathsAndHomeFallbacks(t *testing.T) {
	home := t.TempDir()
	stateRoot := filepath.Join(home, "state")
	dataRoot := filepath.Join(home, "data")
	tests := []struct {
		name      string
		stateEnv  string
		dataEnv   string
		stateRoot string
		dataRoot  string
	}{
		{
			name:      "absolute XDG paths",
			stateEnv:  stateRoot,
			dataEnv:   dataRoot,
			stateRoot: stateRoot,
			dataRoot:  dataRoot,
		},
		{
			name:      "empty XDG paths",
			stateRoot: filepath.Join(home, stateHomeFallback),
			dataRoot:  filepath.Join(home, dataHomeFallback),
		},
		{
			name:      "relative XDG paths",
			stateEnv:  "relative-state",
			dataEnv:   "relative-data",
			stateRoot: filepath.Join(home, stateHomeFallback),
			dataRoot:  filepath.Join(home, dataHomeFallback),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			t.Setenv(stateHomeVariable, test.stateEnv)
			t.Setenv(dataHomeVariable, test.dataEnv)

			state, err := StateDir()
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(test.stateRoot, appName); state != want {
				t.Fatalf("state directory = %q, want %q", state, want)
			}

			data, err := DataDir()
			if err != nil {
				t.Fatal(err)
			}
			if want := filepath.Join(test.dataRoot, appName); data != want {
				t.Fatalf("data directory = %q, want %q", data, want)
			}
		})
	}
}
