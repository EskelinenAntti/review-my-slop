package app

import (
	"path/filepath"
	"testing"
)

func TestCommentsDataDirectoryUsesXDGEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)

	data, err := userDataDirectory()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, commentsAppName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}

func TestCommentsDataDirectoryFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	data, err := userDataDirectory()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(
		home,
		".local",
		"share",
		commentsAppName,
	)
	if data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}

func TestCommentsRelativeXDGDataDirectoryIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative-data")

	data, err := userDataDirectory()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(
		home,
		".local",
		"share",
		commentsAppName,
	)
	if data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}
