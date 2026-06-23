package gitdiff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestLoaderIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "modified.go", "package main\n\nfunc value() int { return 1 }\n")
	writeFile(t, repo, "staged.txt", "before\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")

	writeFile(t, repo, "modified.go", "package main\n\nfunc value() int { return 2 }\n")
	writeFile(t, repo, "staged.txt", "after\n")
	git(t, repo, "add", "staged.txt")
	writeFile(t, repo, "new.py", "def hello():\n    return 'world'\n")

	got, err := (Loader{}).Load(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}

	if got.Repository != repo {
		t.Fatalf("repository = %q, want %q", got.Repository, repo)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %d, want 2: %#v", len(got.Files), got.Files)
	}
	if got.Files[0].DisplayPath != "modified.go" || got.Files[1].DisplayPath != "new.py" {
		t.Fatalf("unexpected files: %q, %q", got.Files[0].DisplayPath, got.Files[1].DisplayPath)
	}
	modified := got.Files[0]
	if !containsKind(modified, patch.Deletion, "return 1") ||
		!containsKind(modified, patch.Addition, "return 2") {
		t.Fatalf("modified file lacks expected lines: %#v", modified.Hunks)
	}
	untracked := got.Files[1]
	if untracked.OldSource != "" || !strings.Contains(untracked.NewSource, "def hello") {
		t.Fatalf("unexpected untracked sources: %#v", untracked)
	}
	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind != patch.Addition || line.OldNumber != 0 || line.NewNumber == 0 {
				t.Fatalf("unexpected untracked line: %#v", line)
			}
		}
	}
	if got.Fingerprint == "" {
		t.Fatal("fingerprint is empty")
	}
}

func TestAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := addedFile("odd\nname.go", "package main\n")
	if file.NewPath != "odd\nname.go" {
		t.Fatalf("raw path = %q", file.NewPath)
	}
	if file.DisplayPath != `odd\nname.go` {
		t.Fatalf("display path = %q", file.DisplayPath)
	}
}

func TestLoaderShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "tracked.bin", "\x00old")
	git(t, repo, "add", "tracked.bin")
	git(t, repo, "commit", "-m", "base")
	writeFile(t, repo, "tracked.bin", "\x00new")
	writeFile(t, repo, "untracked.bin", "\x00content")

	got, err := (Loader{}).Load(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %d, want 2: %#v", len(got.Files), got.Files)
	}
	for _, file := range got.Files {
		if len(file.Hunks) != 0 {
			t.Fatalf("binary file %q has diff lines: %#v", file.DisplayPath, file.Hunks)
		}
		if !strings.Contains(strings.ToLower(strings.Join(file.Metadata, "\n")), "binary") {
			t.Fatalf("binary file %q lacks metadata: %#v", file.DisplayPath, file.Metadata)
		}
	}
}

func TestLoadBranchIncludesCommittedStagedUnstagedAndUntrackedChanges(t *testing.T) {
	repo := newRepository(t)
	git(t, repo, "branch", "-M", "main")
	writeFile(t, repo, "committed.txt", "base\n")
	writeFile(t, repo, "mixed.txt", "base\n")
	writeFile(t, repo, "staged.txt", "base\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")

	git(t, repo, "switch", "-c", "feature")
	writeFile(t, repo, "committed.txt", "committed on feature\n")
	git(t, repo, "add", "committed.txt")
	git(t, repo, "commit", "-m", "feature commit")
	writeFile(t, repo, "staged.txt", "staged on feature\n")
	git(t, repo, "add", "staged.txt")
	writeFile(t, repo, "mixed.txt", "unstaged on feature\n")
	writeFile(t, repo, "untracked.txt", "untracked on feature\n")

	got, err := (Loader{}).LoadBranch(context.Background(), repo, "main")
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"committed.txt", "mixed.txt", "staged.txt", "untracked.txt"}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %d, want %d: %#v", len(got.Files), len(want), got.Files)
	}
	for i, name := range want {
		if got.Files[i].DisplayPath != name {
			t.Fatalf("file %d = %q, want %q", i, got.Files[i].DisplayPath, name)
		}
	}
	if !containsKind(got.Files[0], patch.Addition, "committed on feature") ||
		!containsKind(got.Files[1], patch.Addition, "unstaged on feature") ||
		!containsKind(got.Files[2], patch.Addition, "staged on feature") ||
		!containsKind(got.Files[3], patch.Addition, "untracked on feature") {
		t.Fatalf("branch diff lacks expected changes: %#v", got.Files)
	}
	if got.Files[0].OldSource != "base\n" || got.Files[0].NewSource != "committed on feature\n" {
		t.Fatalf("unexpected branch sources: old=%q new=%q", got.Files[0].OldSource, got.Files[0].NewSource)
	}
}

func TestDefaultBranchFallbacks(t *testing.T) {
	tests := []struct {
		name       string
		originHEAD string
		available  string
		want       string
	}{
		{name: "origin HEAD", originHEAD: "origin/trunk", want: "origin/trunk"},
		{name: "origin main", available: "origin/main", want: "origin/main"},
		{name: "local main", available: "main", want: "main"},
		{name: "origin master", available: "origin/master", want: "origin/master"},
		{name: "local master", available: "master", want: "master"},
		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			runner := &defaultBranchRunner{root: root, originHEAD: tt.originHEAD, available: tt.available}
			got, err := (Loader{Runner: runner}).DefaultBranch(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("default branch = %q, want %q", got, tt.want)
			}
			for _, call := range runner.calls {
				if strings.Contains(call, "for-each-ref") || strings.Contains(call, "merge-base") || strings.Contains(call, "rev-list") {
					t.Fatalf("default branch discovery inspected refs: git %s", call)
				}
			}
		})
	}
}

type defaultBranchRunner struct {
	root       string
	originHEAD string
	available  string
	calls      []string
}

func (r *defaultBranchRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case call == "symbolic-ref --quiet --short refs/remotes/origin/HEAD" && r.originHEAD != "":
		return []byte(r.originHEAD + "\n"), nil
	case len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "--quiet" && args[3] == r.available+"^{commit}" && r.available != "":
		return []byte("commit\n"), nil
	default:
		return nil, exec.ErrNotFound
	}
}

func TestLoaderDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("do not read"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "link")); err != nil {
		t.Fatal(err)
	}

	got, err := (Loader{}).Load(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(got.Files))
	}
	if got.Files[0].NewSource != outside {
		t.Fatalf("symlink source = %q, want link target %q", got.Files[0].NewSource, outside)
	}
	if strings.Contains(got.Files[0].NewSource, "do not read") {
		t.Fatal("symlink target contents were read")
	}
}

func TestParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := parseHunkBody(10, 20, []byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []patch.Line{
		{Kind: patch.Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: patch.Deletion, Text: "old", OldNumber: 11},
		{Kind: patch.Addition, Text: "new", NewNumber: 21},
		{Kind: patch.Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	if len(lines) != len(want) {
		t.Fatalf("lines = %#v", lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d = %#v, want %#v", i, lines[i], want[i])
		}
	}
}

func TestParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := parseHunkBody(1, 1, []byte("+hello\x1b[2J\n"))
	if err != nil {
		t.Fatal(err)
	}
	if lines[0].Text != `hello\x1b[2J` {
		t.Fatalf("text = %q", lines[0].Text)
	}
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = parseHunkBody(1, 1, body)
	})
}

func containsKind(file patch.File, kind patch.LineKind, text string) bool {
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
	if err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-q")
	git(t, repo, "config", "user.email", "test@example.com")
	git(t, repo, "config", "user.name", "Test")
	return repo
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
