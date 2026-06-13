package gitdiff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/anttieskelinen/review-my-slop/internal/review"
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
	if got.Files[0].Display != "modified.go" || got.Files[1].Display != "new.py" {
		t.Fatalf("unexpected files: %q, %q", got.Files[0].Display, got.Files[1].Display)
	}
	modified := got.Files[0]
	if !containsKind(modified, review.LineRemoved, "return 1") ||
		!containsKind(modified, review.LineAdded, "return 2") {
		t.Fatalf("modified file lacks expected lines: %#v", modified.Hunks)
	}
	untracked := got.Files[1]
	if untracked.OldSource != "" || !strings.Contains(untracked.NewSource, "def hello") {
		t.Fatalf("unexpected untracked sources: %#v", untracked)
	}
	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind != review.LineAdded || line.OldNumber != 0 || line.NewNumber == 0 {
				t.Fatalf("unexpected untracked line: %#v", line)
			}
		}
	}
	if got.Fingerprint == "" {
		t.Fatal("fingerprint is empty")
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

	if got.Base != "main" {
		t.Fatalf("base = %q, want main", got.Base)
	}
	want := []string{"committed.txt", "mixed.txt", "staged.txt", "untracked.txt"}
	if len(got.Files) != len(want) {
		t.Fatalf("files = %d, want %d: %#v", len(got.Files), len(want), got.Files)
	}
	for i, name := range want {
		if got.Files[i].Display != name {
			t.Fatalf("file %d = %q, want %q", i, got.Files[i].Display, name)
		}
	}
	if !containsKind(got.Files[0], review.LineAdded, "committed on feature") ||
		!containsKind(got.Files[1], review.LineAdded, "unstaged on feature") ||
		!containsKind(got.Files[2], review.LineAdded, "staged on feature") ||
		!containsKind(got.Files[3], review.LineAdded, "untracked on feature") {
		t.Fatalf("branch diff lacks expected changes: %#v", got.Files)
	}
	if got.Files[0].OldSource != "base\n" || got.Files[0].NewSource != "committed on feature\n" {
		t.Fatalf("unexpected branch sources: old=%q new=%q", got.Files[0].OldSource, got.Files[0].NewSource)
	}
}

func TestParentBranchesOrdersDistinctStackedParentsNearestFirst(t *testing.T) {
	repo := newRepository(t)
	git(t, repo, "branch", "-M", "main")
	writeFile(t, repo, "stack.txt", "main\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "main")

	git(t, repo, "switch", "-c", "stack-one")
	writeFile(t, repo, "stack.txt", "stack one\n")
	git(t, repo, "commit", "-am", "stack one")
	git(t, repo, "branch", "duplicate-stack-one")

	git(t, repo, "switch", "-c", "stack-two")
	writeFile(t, repo, "stack.txt", "stack two\n")
	git(t, repo, "commit", "-am", "stack two")
	git(t, repo, "switch", "-c", "child")
	writeFile(t, repo, "stack.txt", "child\n")
	git(t, repo, "commit", "-am", "child")
	git(t, repo, "switch", "stack-two")
	remote := filepath.Join(t.TempDir(), "remote.git")
	git(t, filepath.Dir(remote), "init", "--bare", "-q", remote)
	git(t, repo, "remote", "add", "origin", remote)
	git(t, repo, "push", "-u", "origin", "stack-two")

	got, err := (Loader{}).ParentBranches(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"duplicate-stack-one", "main"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("parents = %#v, want %#v", got, want)
	}
}

func TestLoaderDoesNotFollowUntrackedSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}
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
	want := []review.Line{
		{Kind: review.LineContext, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: review.LineRemoved, Text: "old", OldNumber: 11},
		{Kind: review.LineAdded, Text: "new", NewNumber: 21},
		{Kind: review.LineContext, Text: "same", OldNumber: 12, NewNumber: 22},
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

func containsKind(file review.File, kind review.LineKind, text string) bool {
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
