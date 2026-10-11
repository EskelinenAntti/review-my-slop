package patch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	{
		err := os.WriteFile(outside, []byte("do not read"), privateFileMode)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		err := os.Symlink(outside, filepath.Join(repo, "link"))
		requireEqual(t, err == nil, true, fixtureV, err)
	}

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(t, len(got.Files), 1, "files = %d, want 1", len(got.Files))
	requireEqual(
		t,
		got.Files[0].NewSource, outside,
		"symlink source = %q, want link target %q",
		got.Files[0].NewSource,
		outside,
	)
	requireEqual(
		t,
		got.Files[0].OldPath == "" && got.Files[0].NewPath == "link",
		true,
		"symlink paths = %#v",
		got.Files[0],
	)
	requireEqual(
		t,
		!strings.Contains(got.Files[0].NewSource, "do not read"),
		true,
		fixtureV,
		"symlink target contents were read",
	)
}
