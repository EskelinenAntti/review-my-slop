package patch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func patchContainsLine(file File, kind LineKind, text string) bool {
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
	testutil.NoError(t, err)

	runRepositoryGit(t, repo, "init", "-q")
	runRepositoryGit(t, repo, "config", "user.email", "test@example.com")
	runRepositoryGit(t, repo, "config", "user.name", "Test")
	return repo
}

func runRepositoryGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeRepositoryFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
