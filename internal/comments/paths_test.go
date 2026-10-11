package comments

import (
	"path/filepath"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestDataDirectoryUsesXDGEnvironment(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)

	data, err := DataDir()
	testutil.Require(t, err == nil, err)
	{
		want := filepath.Join(root, appName)
		testutil.Requiref(t, data == want, "data directory = %q, want %q", data, want)
	}
}

func TestDataDirectoryFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	data, err := DataDir()
	testutil.Require(t, err == nil, err)
	{
		want := filepath.Join(home, ".local", "share", appName)
		testutil.Requiref(t, data == want, "data directory = %q, want %q", data, want)
	}
}

func TestRelativeXDGDataDirectoryIsIgnored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "relative-data")

	data, err := DataDir()
	testutil.Require(t, err == nil, err)
	{
		want := filepath.Join(home, ".local", "share", appName)
		testutil.Requiref(t, data == want, "data directory = %q, want %q", data, want)
	}
}
