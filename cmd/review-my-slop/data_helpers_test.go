package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func loadPatch(ctx context.Context, kind patchKind) (patchPatch, error) {
	return loadCurrentPatch(ctx, kind)
}

func commentsStoreFromTestEnvironment() (commentsStore, error) {
	return userCommentsStore()
}

func createGitTestRepository(t *testing.T,

) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	requireNoError(t, err)
	var args = []string{"init", "-q"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	{
		var args = []string{"config", "user.email", "test@example.com"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args = []string{"config", "user.name", "Test"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	return repo
}

func createInitializedGitTestRepository(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	requireNoError(t, err)

	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return repo
}

func writeGitTestFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	requireNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	requireNoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

func runGitTestCommand(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func patchContainsLine(file patchFile, kind patchLineKind, text string) bool {
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind == kind && strings.Contains(line.Text, text) {
				return true
			}
		}
	}
	return false
}

func createBranchChangesTestRepository(t *testing.T) string {
	t.Helper()
	repo := createGitTestRepository(t)
	runGitTestCommand(t, repo, "branch", "-M", "main")
	writeGitTestFile(t, repo, "committed.txt", "base\n")
	writeGitTestFile(t, repo, "mixed.txt", "base\n")
	writeGitTestFile(t, repo, "staged.txt", "base\n")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "base")
	runGitTestCommand(t, repo, "switch", "-c", "feature")
	writeGitTestFile(t, repo, "committed.txt", "committed on feature\n")
	runGitTestCommand(t, repo, "add", "committed.txt")
	runGitTestCommand(t, repo, "commit", "-m", "feature commit")
	writeGitTestFile(t, repo, "staged.txt", "staged on feature\n")
	runGitTestCommand(t, repo, "add", "staged.txt")
	writeGitTestFile(t, repo, "mixed.txt", "unstaged on feature\n")
	writeGitTestFile(t, repo, "untracked.txt", "untracked on feature\n")
	return repo
}

func createBranchPathChangesTestRepository(t *testing.T) string {
	t.Helper()
	repo := createGitTestRepository(t)
	runGitTestCommand(t, repo, "branch", "-M", "main")
	writeGitTestFile(t, repo, "deleted.txt", "delete this\n")
	writeGitTestFile(t, repo, "old.go", "package main\n\nfunc example() {}\n")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "base")
	runGitTestCommand(t, repo, "switch", "-c", "feature")
	runGitTestCommand(t, repo, "mv", "old.go", "new.go")
	runGitTestCommand(t, repo, "rm", "deleted.txt")
	writeGitTestFile(t, repo, "added.txt", "new content\n")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "changes")
	writeGitTestFile(t, repo, "untracked.txt", "untracked content\n")
	return repo
}
