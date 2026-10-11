package comments

import (
	"path/filepath"
	"testing"
)

func TestDataDirectoryUsesXDGEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv(fixtureXDGDATAHOME, root)

	data, err := DataDir()
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		want := filepath.Join(root, appName)
		requireEqual(t, data, want, fixtureDataDirectoryQWant, data, want)
	}
}

func TestDataDirectoryFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(fixtureXDGDATAHOME, "")

	data, err := DataDir()
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		want := filepath.Join(home, ".local", "share", appName)
		requireEqual(t, data, want, fixtureDataDirectoryQWant, data, want)
	}
}

func TestRelativeXDGDataDirectoryIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(fixtureXDGDATAHOME, "relative-data")

	data, err := DataDir()
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		want := filepath.Join(home, ".local", "share", appName)
		requireEqual(t, data, want, fixtureDataDirectoryQWant, data, want)
	}
}
