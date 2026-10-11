package patch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func containsKind(file File, kind LineKind, text string) bool {
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind == kind && strings.Contains(line.Text, text) {
				return true
			}
		}
	}
	return false
}

func newRepository(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	requireEqual(t, err == nil, true, fixtureV, err)
	git(t, repo, "init", "-q")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test")
	return repo
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	{
		out, err := cmd.CombinedOutput()
		requireEqual(
			t,
			err == nil, true,
			"git %s: %v\n%s",
			strings.Join(args, " "),
			err,
			out,
		)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	{
		err := os.MkdirAll(filepath.Dir(path), executableDirMode)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		err := os.WriteFile(path, []byte(content), publicFileMode)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
}

func (r *defaultBranchRunner) matchesAvailableBranch(args []string) bool {
	return len(args) == 4 && args[0] == "rev-parse" &&
		args[1] == "--verify" &&
		args[2] == "--quiet" &&
		args[3] == r.available+"^{commit}" &&
		r.available != ""
}

func assertUntrackedLines(t *testing.T, untracked File) {
	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			requireEqual(
				t,
				line.Kind == Addition && line.OldNumber == 0 &&
					line.NewNumber != 0,
				true,
				"unexpected untracked line: %#v",
				line,
			)
		}
	}
}

func branchRepository(t *testing.T) string {
	repo := newRepository(t)
	git(t, repo, "branch", "-M", fixtureMain)
	writeFile(t, repo, fixtureCommittedTxt, fixtureBaseContent)
	writeFile(t, repo, fixtureMixedTxt, fixtureBaseContent)
	writeFile(t, repo, fixtureStagedTxt, fixtureBaseContent)
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)

	git(t, repo, "switch", "-c", "feature")
	writeFile(t, repo, fixtureCommittedTxt, "committed on feature\n")
	git(t, repo, gitAddCommand, fixtureCommittedTxt)
	git(t, repo, gitCommitCommand, gitMessageFlag, "feature commit")
	writeFile(t, repo, fixtureStagedTxt, "staged on feature\n")
	git(t, repo, gitAddCommand, fixtureStagedTxt)
	writeFile(t, repo, fixtureMixedTxt, "unstaged on feature\n")
	writeFile(t, repo, fixtureUntrackedTxt, "untracked on feature\n")

	return repo
}

func assertUnstagedFileList(t *testing.T, got Patch, repo string) {
	requireEqual(
		t,
		got.Root == repo && got.Kind == Unstaged, true,
		"root = %q, kind = %v",
		got.Root,
		got.Kind,
	)
	requireEqual(
		t,
		len(got.Files), 2,
		"files = %d, want 2: %#v",
		len(got.Files),
		got.Files,
	)
	requireEqual(
		t,
		got.Files[0].DisplayPath == "modified.go" &&
			got.Files[1].DisplayPath == "new.py",
		true,
		"unexpected files: %q, %q",
		got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)
}
