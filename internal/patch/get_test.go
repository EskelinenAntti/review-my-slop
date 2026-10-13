package patch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestGetIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := newRepository(t)
	writeRepositoryFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 1 }\n",
	)
	writeRepositoryFile(t, repo, "staged.txt", "before\n")
	runRepositoryGit(t, repo, "add", ".")
	runRepositoryGit(t, repo, "commit", "-m", "base")

	writeRepositoryFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 2 }\n",
	)
	writeRepositoryFile(t, repo, "staged.txt", "after\n")
	runRepositoryGit(t, repo, "add", "staged.txt")
	writeRepositoryFile(t, repo, "new.py", "def hello():\n    return 'world'\n")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Root, repo,
		fmt.Sprintf("root = %q, kind = %v", got.Root, got.Kind),
	)
	testutil.RequireEqual(t, got.Kind, Unstaged,
		fmt.Sprintf("root = %q, kind = %v", got.Root, got.Kind),
	)
	testutil.RequireEqual(t, len(got.Files), 2,
		fmt.Sprintf("files = %d, want 2: %#v", len(got.Files), got.Files),
	)
	testutil.RequireEqual(t, got.Files[0].DisplayPath, "modified.go",
		fmt.Sprintf("unexpected files: %q, %q", got.Files[0].DisplayPath,
			got.Files[1].DisplayPath,
		),
	)
	testutil.RequireEqual(t, got.Files[1].DisplayPath, "new.py",
		fmt.Sprintf("unexpected files: %q, %q", got.Files[0].DisplayPath,
			got.Files[1].DisplayPath,
		),
	)

	modified := got.Files[0]
	testutil.RequireEqual(t, patchContainsLine(modified, Deletion, "return 1"),
		true,
		fmt.Sprintf("modified file lacks expected lines: %#v", modified.Hunks),
	)
	testutil.RequireEqual(t, patchContainsLine(
		modified,
		Addition,
		"return 2",
	), true,
		fmt.Sprintf("modified file lacks expected lines: %#v", modified.Hunks),
	)

	assertUntrackedSource(t, got.Files[1])
}

func TestGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := newRepository(t)
	writeRepositoryFile(t, repo, "nested/odd\nname.go", "package main\n")
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Root, repo, fmt.Sprintf("patch = %#v", got))
	testutil.RequireEqual(t, len(got.Files), 1,
		fmt.Sprintf("patch = %#v", got),
	)

	file := got.Files[0]
	testutil.RequireEqual(t, file.OldPath, "",
		fmt.Sprintf("file paths = %#v", file),
	)
	testutil.RequireEqual(t, file.NewPath, "nested/odd\nname.go",
		fmt.Sprintf("file paths = %#v", file),
	)
	testutil.RequireEqual(t, file.DisplayPath, `nested/odd\nname.go`,
		fmt.Sprintf("file paths = %#v", file),
	)
}

func TestGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(newRepository(t))
	local, err := Get(context.Background(), Unstaged)
	testutil.RequireEqual(t, err, nil,
		fmt.Sprintf("local patch = %#v, error = %v", local, err),
	)
	testutil.RequireEqual(t, local.Branch, "",
		fmt.Sprintf("local patch = %#v, error = %v", local, err),
	)

	for _, test := range []struct {
		kind    Kind
		message string
	}{
		{Branch, "default branch is unavailable"},
		{Kind(255), "unknown patch kind"},
	} {
		got, err := Get(context.Background(), test.kind)
		testutil.RequireEqual(t, err != nil, true,
			fmt.Sprintf("kind %v: patch = %#v, error = %v", test.kind, got,
				err,
			),
		)
		testutil.RequireEqual(t, strings.Contains(err.Error(), test.message),
			true,
			fmt.Sprintf("kind %v: patch = %#v, error = %v", test.kind, got,
				err,
			),
		)
		testutil.RequireEqual(t, reflect.DeepEqual(got, Patch{}), true,
			fmt.Sprintf("kind %v: patch = %#v, error = %v", test.kind, got,
				err,
			),
		)
	}
}

func TestGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := Get(context.Background(), Unstaged)
	testutil.RequireEqual(t, err != nil, true,
		fmt.Sprintf("patch = %#v, error = %v", got, err),
	)
	testutil.RequireEqual(t, reflect.DeepEqual(got, Patch{}), true,
		fmt.Sprintf("patch = %#v, error = %v", got, err),
	)
}

func TestGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(newRepository(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := Get(ctx, Unstaged)
	testutil.RequireEqual(t, errors.Is(err, context.Canceled), true,
		fmt.Sprintf("patch = %#v, error = %v", got, err),
	)
	testutil.RequireEqual(t, reflect.DeepEqual(got, Patch{}), true,
		fmt.Sprintf("patch = %#v, error = %v", got, err),
	)
}
