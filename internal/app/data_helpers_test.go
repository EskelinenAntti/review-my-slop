package app

import (
	"os/exec"
	"path/filepath"
	"testing"

	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

func commentsStoreFromTestEnvironment() (feedback.Store, error) {
	return userCommentsStore()
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
