package diff

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadShowsUnstagedAndUntrackedChanges(t *testing.T) {
	repository := newRepository(t)
	writeFile(t, repository, "changed.go", "package main\n\nfunc value() int { return 1 }\n")
	writeFile(t, repository, "staged.txt", "before\n")
	git(t, repository, "add", ".")
	git(t, repository, "commit", "-m", "base")

	writeFile(t, repository, "changed.go", "package main\n\nfunc value() int { return 2 }\n")
	writeFile(t, repository, "staged.txt", "after\n")
	git(t, repository, "add", "staged.txt")
	writeFile(t, repository, "new.py", "def hello():\n    return 'world'\n")

	changes, err := (Loader{}).Load(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fileNames(changes), []string{"changed.go", "new.py"}; !equalStrings(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	changed := changes.Files[0]
	if !hasLine(changed, Deletion, "return 1") || !hasLine(changed, Addition, "return 2") {
		t.Fatalf("changed file does not contain both versions: %#v", changed.Hunks)
	}
	if changes.Files[1].OldSource != "" || !strings.Contains(changes.Files[1].NewSource, "def hello") {
		t.Fatalf("unexpected untracked sources: %#v", changes.Files[1])
	}
	if changes.Fingerprint == "" {
		t.Fatal("change fingerprint is empty")
	}
}

func TestLoadSeparatesRawAndDisplayPaths(t *testing.T) {
	repository := newRepository(t)
	path := "odd\nname.go"
	writeFile(t, repository, path, "package main\n")
	changes, err := (Loader{}).Load(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes.Files) != 1 || changes.Files[0].NewPath != path || changes.Files[0].DisplayPath != `odd\nname.go` {
		t.Fatalf("file = %#v", changes.Files)
	}
}

func TestLoadBranchIncludesCommittedAndWorkingTreeChanges(t *testing.T) {
	repository := newRepository(t)
	git(t, repository, "branch", "-M", "main")
	writeFile(t, repository, "base.txt", "base\n")
	writeFile(t, repository, "mixed.txt", "base\n")
	writeFile(t, repository, "staged.txt", "base\n")
	git(t, repository, "add", ".")
	git(t, repository, "commit", "-m", "base")

	git(t, repository, "switch", "-c", "feature")
	writeFile(t, repository, "base.txt", "feature commit\n")
	git(t, repository, "add", "base.txt")
	git(t, repository, "commit", "-m", "feature")
	writeFile(t, repository, "staged.txt", "staged change\n")
	git(t, repository, "add", "staged.txt")
	writeFile(t, repository, "mixed.txt", "working tree change\n")
	writeFile(t, repository, "new.txt", "untracked change\n")

	changes, err := (Loader{}).LoadBranch(context.Background(), repository, "main")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fileNames(changes), []string{"base.txt", "mixed.txt", "new.txt", "staged.txt"}; !equalStrings(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	if changes.Files[0].OldSource != "base\n" || changes.Files[0].NewSource != "feature commit\n" {
		t.Fatalf("branch source mismatch: old=%q new=%q", changes.Files[0].OldSource, changes.Files[0].NewSource)
	}
}

func TestLoadDescribesBinaryFilesAndDoesNotReadSymlinkTargets(t *testing.T) {
	repository := newRepository(t)
	writeFile(t, repository, "tracked.bin", "\x00old")
	git(t, repository, "add", "tracked.bin")
	git(t, repository, "commit", "-m", "base")
	writeFile(t, repository, "tracked.bin", "\x00new")
	writeFile(t, repository, "outside.txt", "secret")
	if err := os.Symlink(filepath.Join(repository, "outside.txt"), filepath.Join(repository, "link.txt")); err != nil {
		t.Fatal(err)
	}

	changes, err := (Loader{}).Load(context.Background(), repository)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range changes.Files {
		if file.DisplayPath == "tracked.bin" && !strings.Contains(strings.ToLower(strings.Join(file.Metadata, "\n")), "binary") {
			t.Fatalf("binary metadata missing: %#v", file)
		}
		if file.DisplayPath == "link.txt" {
			if file.NewSource != filepath.Join(repository, "outside.txt") || strings.Contains(file.NewSource, "secret") {
				t.Fatalf("symlink target was read: %#v", file)
			}
		}
	}
}

func TestDefaultBranchUsesConfiguredFallbacks(t *testing.T) {
	tests := []struct {
		name       string
		originHead string
		available  string
		want       string
	}{
		{name: "origin head", originHead: "origin/trunk", want: "origin/trunk"},
		{name: "origin main", available: "origin/main", want: "origin/main"},
		{name: "local main", available: "main", want: "main"},
		{name: "origin master", available: "origin/master", want: "origin/master"},
		{name: "local master", available: "master", want: "master"},
		{name: "none"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			runner := &branchRunner{root: root, originHead: test.originHead, available: test.available}
			got, err := (Loader{Runner: runner}).DefaultBranch(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("default branch = %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseHunkBodyTracksNumbersAndIgnoresNoNewlineMarker(t *testing.T) {
	lines, err := parseHunkBody(10, 20, []byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 4 {
		t.Fatalf("parsed lines = %#v", lines)
	}
	want := []Line{
		{Kind: Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: Deletion, Text: "old", OldNumber: 11},
		{Kind: Addition, Text: "new", NewNumber: 21},
		{Kind: Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	for index := range want {
		if lines[index] != want[index] {
			t.Fatalf("line %d = %#v, want %#v", index, lines[index], want[index])
		}
	}
}

func TestParseHunkBodyRejectsUnknownPrefix(t *testing.T) {
	if _, err := parseHunkBody(1, 1, []byte("?unknown\n")); err == nil {
		t.Fatal("unknown diff prefix was accepted")
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
	root       string
	originHead string
	available  string
}

func (r *branchRunner) Run(_ context.Context, _ string, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case call == "symbolic-ref --quiet --short refs/remotes/origin/HEAD" && r.originHead != "":
		return []byte(r.originHead + "\n"), nil
	case len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify" && args[2] == "--quiet" && args[3] == r.available+"^{commit}" && r.available != "":
		return []byte("commit\n"), nil
	default:
		return nil, errors.New("not found")
	}
}

func fileNames(changes ChangeSet) []string {
	result := make([]string, len(changes.Files))
	for index, file := range changes.Files {
		result[index] = file.DisplayPath
	}
	return result
}

func hasLine(file File, kind LineKind, text string) bool {
	for _, hunk := range file.Hunks {
		for _, line := range hunk.Lines {
			if line.Kind == kind && strings.Contains(line.Text, text) {
				return true
			}
		}
	}
	return false
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func newRepository(t *testing.T) string {
	t.Helper()
	repository := t.TempDir()
	git(t, repository, "init", "-q")
	git(t, repository, "config", "user.email", "test@example.com")
	git(t, repository, "config", "user.name", "Test")
	return repository
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func writeFile(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
