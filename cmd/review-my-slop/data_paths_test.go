package main

import (
	fmt "fmt"
	os "os"
	filepath "path/filepath"
	testing "testing"
)

func TestCommentsDataDirectoryUsesXDGEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)

	data, err := (func() (string, error) {
		var fallback string = filepath.Join(".local", "share")
		if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
			return filepath.Join(root, commentsAppName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		return filepath.Join(home, fallback, commentsAppName), nil
	})()
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

	data, err := (func() (string, error) {
		var fallback string = filepath.Join(".local", "share")
		if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
			return filepath.Join(root, commentsAppName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		return filepath.Join(home, fallback, commentsAppName), nil
	})()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", commentsAppName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}

func TestCommentsRelativeXDGDataDirectoryIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative-data")

	data, err := (func() (string, error) {
		var fallback string = filepath.Join(".local", "share")
		if root := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(root) {
			return filepath.Join(root, commentsAppName), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve user home directory: %w", err)
		}
		return filepath.Join(home, fallback, commentsAppName), nil
	})()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", commentsAppName); data != want {
		t.Fatalf("data directory = %q, want %q", data, want)
	}
}
