package patch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := addedFile("odd\nname.go", "package main\n")
	testutil.RequireEqual(t, file.NewPath, "odd\nname.go", "raw path = %q",
		file.NewPath,
	)
	testutil.RequireEqual(t, file.DisplayPath, `odd\nname.go`,
		"display path = %q", file.DisplayPath,
	)
}

func TestGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "nested/odd\nname.go", "package main\n")
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Root, repo, "patch = %#v", got)
	testutil.RequireEqual(t, len(got.Files), 1, "patch = %#v", got)

	file := got.Files[0]
	testutil.RequireEqual(t, file.OldPath, "", "file paths = %#v", file)
	testutil.RequireEqual(t, file.NewPath, "nested/odd\nname.go",
		"file paths = %#v", file,
	)
	testutil.RequireEqual(t, file.DisplayPath, `nested/odd\nname.go`,
		"file paths = %#v", file,
	)
}

func TestGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "large.txt", strings.Repeat("x", maxFileBytes+1))
	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 1, "files = %#v", got.Files)

	file := got.Files[0]
	testutil.RequireEqual(t, file.OldPath, "", "large file = %#v", file)
	testutil.RequireEqual(t, file.NewPath, "large.txt", "large file = %#v",
		file,
	)
	testutil.RequireEqual(t, file.NewSource, "", "large file = %#v", file)
	testutil.RequireEqual(t, len(file.Hunks), 0, "large file = %#v", file)
	testutil.RequireEqual(t,
		strings.Contains(
			strings.Join(file.Metadata, "\n"),
			"exceeds 2 MiB",
		), true, "large file = %#v", file)
}

func TestGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	testutil.NoError(t, os.WriteFile(outside, []byte("do not read"), 0o600))
	testutil.NoError(t, os.Symlink(outside, filepath.Join(repo, "link")))

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 1, "files = %d, want 1",
		len(got.Files),
	)
	testutil.RequireEqual(t, got.Files[0].NewSource, outside,
		"symlink source = %q, want link target %q", got.Files[0].NewSource,
		outside,
	)
	testutil.RequireEqual(t, got.Files[0].OldPath, "", "symlink paths = %#v",
		got.Files[0],
	)
	testutil.RequireEqual(t, got.Files[0].NewPath, "link",
		"symlink paths = %#v", got.Files[0],
	)
	testutil.RequireEqual(t,
		!strings.Contains(got.Files[0].NewSource, "do not read"), true, "%s",
		"symlink target contents were read",
	)
}
