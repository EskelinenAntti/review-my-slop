package patch

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestReadGitSourceUsesIndexOrRevision(t *testing.T) {
	repo := newRepository(t)
	path := "nested/source file.txt"
	writeFile(t, repo, path, "committed content\n")
	git(t, repo, "add", "--", path)
	git(t, repo, "commit", "-qm", "initial")
	writeFile(t, repo, path, "index content\n")
	git(t, repo, "add", "--", path)
	writeFile(t, repo, path, "working-tree content\n")
	for _, tt := range []struct {
		name, revision, want string
	}{
		{"index", "", "index content\n"},
		{"revision", "HEAD", "committed content\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := readGitSource(context.Background(), execRunner{}, repo,
				tt.revision, path,
			)
			testutil.RequireEqual(t, got, tt.want,
				"source = %q, want %q", got, tt.want,
			)
		})
	}
}

func TestReadLimitedFileEnforcesLimitAfterStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("small"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", maxFileBytes+1)
	if err := os.WriteFile(path, []byte(large), 0o600); err != nil {
		t.Fatal(err)
	}

	content, oversized, err := readLimitedFile(path, info)
	if err != nil {
		t.Fatal(err)
	}
	if !oversized {
		t.Fatal("oversized file was not reported")
	}
	if len(content) != 0 {
		t.Fatalf("oversized content length = %d, want 0", len(content))
	}
}
