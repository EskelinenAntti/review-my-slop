package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
