package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/eskelinenantti/review-my-slop/internal/diff"
)

func TestLoaderIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repository := newRepository(t)
	writeFile(t, repository, "modified.go", "package main\n\nfunc value() int { return 1 }\n")
	writeFile(t, repository, "staged.txt", "before\n")
	gitCommand(t, repository, "add", ".")
	gitCommand(t, repository, "commit", "-m", "base")

	writeFile(t, repository, "modified.go", "package main\n\nfunc value() int { return 2 }\n")
	writeFile(t, repository, "staged.txt", "after\n")
	gitCommand(t, repository, "add", "staged.txt")
	writeFile(t, repository, "new.py", "def hello():\n    return 'world'\n")

	got, err := NewLoader(nil).Load(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repository != repository || len(got.Files) != 2 {
		t.Fatalf("changes = %#v", got)
	}
	if got.Files[0].Path() != "modified.go" || got.Files[1].Path() != "new.py" {
		t.Fatalf("paths = %q, %q", got.Files[0].Path(), got.Files[1].Path())
	}
	if !containsLine(got.Files[0], domain.Deletion, "return 1") || !containsLine(got.Files[0], domain.Addition, "return 2") {
		t.Fatalf("modified lines = %#v", got.Files[0].Hunks)
	}
	if got.Files[1].OldSource != "" || !strings.Contains(got.Files[1].NewSource, "def hello") {
		t.Fatalf("untracked source = %#v", got.Files[1])
	}
	for _, line := range got.Files[1].Hunks[0].Lines {
		if line.Kind != domain.Addition || line.OldNumber != 0 || line.NewNumber == 0 {
			t.Fatalf("untracked line = %#v", line)
		}
	}
}

func TestLoaderKeepsRawPathsAndSourceForPresentation(t *testing.T) {
	repository := newRepository(t)
	writeFile(t, repository, "odd\nname.go", "package main\n")
	got, err := NewLoader(nil).Load(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Files[0].Path() != "odd\nname.go" || got.Files[0].Hunks[0].Lines[0].Text != "package main" {
		t.Fatalf("raw values = %#v", got.Files)
	}
}

func TestLoaderShowsBinaryAndLargeMetadataWithoutReadingContent(t *testing.T) {
	repository := newRepository(t)
	writeFile(t, repository, "tracked.bin", "\x00old")
	gitCommand(t, repository, "add", "tracked.bin")
	gitCommand(t, repository, "commit", "-m", "base")
	writeFile(t, repository, "tracked.bin", "\x00new")
	writeFile(t, repository, "untracked.bin", "\x00content")

	got, err := NewLoader(nil).Load(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %#v", got.Files)
	}
	for _, file := range got.Files {
		if len(file.Hunks) != 0 || !strings.Contains(strings.ToLower(strings.Join(file.Metadata, "\n")), "binary") {
			t.Fatalf("binary file = %#v", file)
		}
	}
}

func TestLoadBranchIncludesCommittedAndWorktreeChanges(t *testing.T) {
	repository := newRepository(t)
	gitCommand(t, repository, "branch", "-M", "main")
	writeFile(t, repository, "committed.txt", "base\n")
	writeFile(t, repository, "mixed.txt", "base\n")
	gitCommand(t, repository, "add", ".")
	gitCommand(t, repository, "commit", "-m", "base")
	gitCommand(t, repository, "switch", "-c", "feature")
	writeFile(t, repository, "committed.txt", "committed on feature\n")
	gitCommand(t, repository, "add", "committed.txt")
	gitCommand(t, repository, "commit", "-m", "feature commit")
	writeFile(t, repository, "mixed.txt", "unstaged on feature\n")
	writeFile(t, repository, "untracked.txt", "untracked on feature\n")

	got, err := NewLoader(nil).LoadBranch(context.Background(), repository, "main")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"committed.txt", "mixed.txt", "untracked.txt"}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %#v", got.Files)
	}
	for index, name := range want {
		if got.Files[index].Path() != name {
			t.Fatalf("file %d = %q, want %q", index, got.Files[index].Path(), name)
		}
	}
	if got.Files[0].OldSource != "base\n" || got.Files[0].NewSource != "committed on feature\n" {
		t.Fatalf("branch sources = %q, %q", got.Files[0].OldSource, got.Files[0].NewSource)
	}
}

func TestDefaultBranchUsesConfiguredFallbacks(t *testing.T) {
	root := t.TempDir()
	runner := &branchRunner{root: root, available: "main"}
	got, err := NewLoader(runner).DefaultBranch(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if got != "main" {
		t.Fatalf("default branch = %q", got)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "for-each-ref") || strings.Contains(call, "merge-base") {
			t.Fatalf("inspected unrelated refs: %s", call)
		}
	}
}

func TestLoaderDoesNotFollowUntrackedSymlink(t *testing.T) {
	repository := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte("do not read"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repository, "link")); err != nil {
		t.Fatal(err)
	}
	got, err := NewLoader(nil).Load(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Files) != 1 || got.Files[0].NewSource != outside || strings.Contains(got.Files[0].NewSource, "do not read") {
		t.Fatalf("symlink file = %#v", got.Files)
	}
}

func TestParseHunkBodyPreservesLineNumbersAndRawControls(t *testing.T) {
	lines, err := parseHunkBody(10, 20, []byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.Line{
		{Kind: domain.Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: domain.Deletion, Text: "old", OldNumber: 11},
		{Kind: domain.Addition, Text: "new", NewNumber: 21},
		{Kind: domain.Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	if len(lines) != len(want) {
		t.Fatalf("lines = %#v", lines)
	}
	for index := range want {
		if lines[index] != want[index] {
			t.Fatalf("line %d = %#v, want %#v", index, lines[index], want[index])
		}
	}
	lines, err = parseHunkBody(1, 1, []byte("+hello\x1b[2J\n"))
	if err != nil || lines[0].Text != "hello\x1b[2J" {
		t.Fatalf("raw control = %#v, %v", lines, err)
	}
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = parseHunkBody(1, 1, body)
	})
}

type branchRunner struct {
	root      string
	available string
	calls     []string
}

func (r *branchRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "--quiet" && args[3] == r.available+"^{commit}":
		return []byte("commit\n"), nil
	default:
		return nil, exec.ErrNotFound
	}
}

func containsLine(file domain.File, kind domain.LineKind, text string) bool {
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
	repository, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitCommand(t, repository, "init", "-q")
	gitCommand(t, repository, "config", "user.email", "test@example.com")
	gitCommand(t, repository, "config", "user.name", "Test")
	return repository
}

func gitCommand(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
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
