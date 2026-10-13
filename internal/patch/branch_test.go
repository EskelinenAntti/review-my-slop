package patch

import (
	"context"
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestGetBranchIncludesCommittedStagedUnstagedAndUntrackedChanges(
	t *testing.T,
) {
	repo := newRepository(t)
	runRepositoryGit(t, repo, "branch", "-M", "main")
	writeRepositoryFile(t, repo, "committed.txt", "base\n")
	writeRepositoryFile(t, repo, "mixed.txt", "base\n")
	writeRepositoryFile(t, repo, "staged.txt", "base\n")
	runRepositoryGit(t, repo, "add", ".")
	runRepositoryGit(t, repo, "commit", "-m", "base")

	runRepositoryGit(t, repo, "switch", "-c", "feature")
	writeRepositoryFile(t, repo, "committed.txt", "committed on feature\n")
	runRepositoryGit(t, repo, "add", "committed.txt")
	runRepositoryGit(t, repo, "commit", "-m", "feature commit")
	writeRepositoryFile(t, repo, "staged.txt", "staged on feature\n")
	runRepositoryGit(t, repo, "add", "staged.txt")
	writeRepositoryFile(t, repo, "mixed.txt", "unstaged on feature\n")
	writeRepositoryFile(t, repo, "untracked.txt", "untracked on feature\n")

	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Root, repo,
		fmt.Sprintf("patch metadata = %#v", got),
	)
	testutil.RequireEqual(t, got.Kind, Branch,
		fmt.Sprintf("patch metadata = %#v", got),
	)
	testutil.RequireEqual(t, got.Branch, "main",
		fmt.Sprintf("patch metadata = %#v", got),
	)

	want := []struct{ name, addition string }{
		{"committed.txt", "committed on feature"},
		{"mixed.txt", "unstaged on feature"},
		{"staged.txt", "staged on feature"},
		{"untracked.txt", "untracked on feature"},
	}
	testutil.RequireEqual(t, len(got.Files), len(want),
		fmt.Sprintf("files = %#v", got.Files),
	)
	for i, expected := range want {
		file := got.Files[i]
		testutil.RequireEqual(t, file.DisplayPath, expected.name,
			fmt.Sprintf("file %d = %#v, want %#v", i, file, expected),
		)
		testutil.RequireEqual(t,
			patchContainsLine(file, Addition, expected.addition),
			true, fmt.Sprintf("missing addition in %#v", file),
		)
	}
	testutil.RequireEqual(t, got.Files[0].OldSource, "base\n",
		fmt.Sprintf("unexpected branch sources: old=%q new=%q",
			got.Files[0].OldSource, got.Files[0].NewSource,
		),
	)
	testutil.RequireEqual(t, got.Files[0].NewSource, "committed on feature\n",
		fmt.Sprintf("unexpected branch sources: old=%q new=%q",
			got.Files[0].OldSource, got.Files[0].NewSource,
		),
	)
}

func TestGetBranchKeepsPathsForAdditionsDeletionsAndRenames(t *testing.T) {
	repo := newRepository(t)
	runRepositoryGit(t, repo, "branch", "-M", "main")
	writeRepositoryFile(t, repo, "deleted.txt", "delete this\n")
	writeRepositoryFile(
		t,
		repo,
		"old.go",
		"package main\n\nfunc example() {}\n",
	)
	runRepositoryGit(t, repo, "add", ".")
	runRepositoryGit(t, repo, "commit", "-m", "base")
	runRepositoryGit(t, repo, "switch", "-c", "feature")
	runRepositoryGit(t, repo, "mv", "old.go", "new.go")
	runRepositoryGit(t, repo, "rm", "deleted.txt")
	writeRepositoryFile(t, repo, "added.txt", "new content\n")
	runRepositoryGit(t, repo, "add", ".")
	runRepositoryGit(t, repo, "commit", "-m", "changes")
	writeRepositoryFile(t, repo, "untracked.txt", "untracked content\n")
	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	testutil.NoError(t, err)

	want := []struct {
		display string
		old     string
		new     string
	}{
		{"added.txt", "", "added.txt"},
		{"deleted.txt", "deleted.txt", ""},
		{"new.go", "old.go", "new.go"},
		{"untracked.txt", "", "untracked.txt"},
	}
	testutil.RequireEqual(t, len(got.Files), len(want),
		fmt.Sprintf("files = %#v", got.Files),
	)

	for i, paths := range want {
		file := got.Files[i]
		testutil.CheckEqual(t, file.DisplayPath == paths.display &&
			file.OldPath == paths.old && file.NewPath == paths.new, true,
			fmt.Sprintf("file %d paths = %#v, want %#v", i, file, paths),
		)
	}
}
