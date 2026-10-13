package patch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "large.txt", strings.Repeat("x", maxFileBytes+1))
	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 1,
		fmt.Sprintf("files = %#v", got.Files),
	)

	file := got.Files[0]
	testutil.RequireEqual(t, file.OldPath, "",
		fmt.Sprintf("large file = %#v", file),
	)
	testutil.RequireEqual(t, file.NewPath, "large.txt",
		fmt.Sprintf("large file = %#v", file),
	)
	testutil.RequireEqual(t, file.NewSource, "",
		fmt.Sprintf("large file = %#v", file),
	)
	testutil.RequireEqual(t, len(file.Hunks), 0,
		fmt.Sprintf("large file = %#v", file),
	)
	testutil.RequireEqual(t, strings.Contains(
		strings.Join(file.Metadata, "\n"),
		"exceeds 2 MiB",
	), true, fmt.Sprintf("large file = %#v", file),
	)
}

func TestGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "tracked.bin", "\x00old")
	git(t, repo, "add", "tracked.bin")
	git(t, repo, "commit", "-m", "base")
	writeFile(t, repo, "tracked.bin", "\x00new")
	writeFile(t, repo, "untracked.bin", "\x00content")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 2,
		fmt.Sprintf("files = %d, want 2: %#v", len(got.Files), got.Files),
	)

	for _, file := range got.Files {
		testutil.CheckEqual(t, file.NewPath == file.DisplayPath &&
			(file.DisplayPath != "tracked.bin" ||
				file.OldPath == "tracked.bin"), true,
			fmt.Sprintf("binary paths = %#v", file),
		)
		testutil.RequireEqual(t, len(file.Hunks), 0,
			fmt.Sprintf("binary file %q has diff lines: %#v", file.DisplayPath,
				file.Hunks,
			),
		)
		testutil.RequireEqual(t, strings.Contains(
			strings.ToLower(strings.Join(file.Metadata, "\n")),
			"binary",
		), true,
			fmt.Sprintf("binary file %q lacks metadata: %#v", file.DisplayPath,
				file.Metadata,
			),
		)
	}
}

func TestGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	testutil.NoError(t, os.WriteFile(outside, []byte("do not read"), 0o600))
	testutil.NoError(t, os.Symlink(outside, filepath.Join(repo, "link")))

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 1,
		fmt.Sprintf("files = %d, want 1", len(got.Files)),
	)
	testutil.RequireEqual(t, got.Files[0].NewSource, outside,
		fmt.Sprintf("symlink source = %q, want link target %q",
			got.Files[0].NewSource, outside,
		),
	)
	testutil.RequireEqual(t, got.Files[0].OldPath, "",
		fmt.Sprintf("symlink paths = %#v", got.Files[0]),
	)
	testutil.RequireEqual(t, got.Files[0].NewPath, "link",
		fmt.Sprintf("symlink paths = %#v", got.Files[0]),
	)
	testutil.RequireEqual(t,
		!strings.Contains(got.Files[0].NewSource, "do not read"), true,
		"symlink target contents were read",
	)
}

func assertUntrackedSource(t *testing.T, untracked File) {
	t.Helper()
	testutil.RequireEqual(t, untracked.OldSource, "",
		fmt.Sprintf("unexpected untracked sources: %#v", untracked),
	)
	testutil.RequireEqual(t,
		strings.Contains(untracked.NewSource, "def hello"), true,
		fmt.Sprintf("unexpected untracked sources: %#v", untracked),
	)

	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			testutil.RequireEqual(t, line.Kind, Addition,
				fmt.Sprintf("unexpected untracked line: %#v", line),
			)
			testutil.RequireEqual(t, line.OldNumber, 0,
				fmt.Sprintf("unexpected untracked line: %#v", line),
			)
			testutil.RequireEqual(t, line.NewNumber != 0, true,
				fmt.Sprintf("unexpected untracked line: %#v", line),
			)
		}
	}
}
