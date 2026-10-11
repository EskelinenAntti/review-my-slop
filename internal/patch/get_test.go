package patch

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestGetIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "modified.go", "package main\n\nfunc value() int { return 1 }\n")
	writeFile(t, repo, "staged.txt", "before\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")

	writeFile(t, repo, "modified.go", "package main\n\nfunc value() int { return 2 }\n")
	writeFile(t, repo, "staged.txt", "after\n")
	git(t, repo, "add", "staged.txt")
	writeFile(t, repo, "new.py", "def hello():\n    return 'world'\n")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.Require(t, err == nil, err)

	testutil.Requiref(t, got.Root == repo && got.Kind == Unstaged, "root = %q, kind = %v", got.Root, got.Kind)
	testutil.Requiref(t, len(got.Files) == 2, "files = %d, want 2: %#v", len(got.Files), got.Files)
	testutil.Requiref(t, got.Files[0].DisplayPath == "modified.go" && got.Files[1].DisplayPath == "new.py", "unexpected files: %q, %q", got.Files[0].DisplayPath, got.Files[1].DisplayPath)
	modified := got.Files[0]
	testutil.Requiref(t, containsKind(modified, Deletion, "return 1") &&
		containsKind(modified, Addition, "return 2"), "modified file lacks expected lines: %#v", modified.Hunks)
	untracked := got.Files[1]
	testutil.Requiref(t, untracked.OldSource == "" && strings.Contains(untracked.NewSource, "def hello"), "unexpected untracked sources: %#v", untracked)
	assertUntrackedLines(t, untracked)
}

func TestAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := addedFile("odd\nname.go", "package main\n")
	testutil.Requiref(t, file.NewPath == "odd\nname.go", "raw path = %q", file.NewPath)
	testutil.Requiref(t, file.DisplayPath == `odd\nname.go`, "display path = %q", file.DisplayPath)
}

func TestGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "nested/odd\nname.go", "package main\n")
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := Get(context.Background(), Unstaged)
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, got.Root == repo && len(got.Files) == 1, "patch = %#v", got)
	file := got.Files[0]
	testutil.Requiref(t, file.OldPath == "" && file.NewPath == "nested/odd\nname.go" && file.DisplayPath == `nested/odd\nname.go`, "file paths = %#v", file)
}

func TestGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(newRepository(t))
	local, err := Get(context.Background(), Unstaged)
	testutil.Requiref(t, err == nil && local.Branch == "", "local patch = %#v, error = %v", local, err)
	for _, test := range []struct {
		kind    Kind
		message string
	}{
		{Branch, "default branch is unavailable"},
		{Kind(255), "unknown patch kind"},
	} {
		got, err := Get(context.Background(), test.kind)
		testutil.Requiref(t, err != nil && strings.Contains(err.Error(), test.message) && reflect.DeepEqual(got, Patch{}), "kind %v: patch = %#v, error = %v", test.kind, got, err)
	}
}

func TestGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := Get(context.Background(), Unstaged)
	testutil.Requiref(t, err != nil && reflect.DeepEqual(got, Patch{}), "patch = %#v, error = %v", got, err)
}

func TestGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(newRepository(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := Get(ctx, Unstaged)
	testutil.Requiref(t, errors.Is(err, context.Canceled) && reflect.DeepEqual(got, Patch{}), "patch = %#v, error = %v", got, err)
}

func TestGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "large.txt", strings.Repeat("x", maxFileBytes+1))
	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(got.Files) == 1, "files = %#v", got.Files)
	file := got.Files[0]
	testutil.Requiref(t, file.OldPath == "" && file.NewPath == "large.txt" && file.NewSource == "" && len(file.Hunks) == 0 &&
		strings.Contains(strings.Join(file.Metadata, "\n"), "exceeds 2 MiB"), "large file = %#v", file)
}

func TestGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "tracked.bin", "\x00old")
	git(t, repo, "add", "tracked.bin")
	git(t, repo, "commit", "-m", "base")
	writeFile(t, repo, "tracked.bin", "\x00new")
	writeFile(t, repo, "untracked.bin", "\x00content")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(got.Files) == 2, "files = %d, want 2: %#v", len(got.Files), got.Files)
	for _, file := range got.Files {
		testutil.Assertf(t, file.NewPath == file.DisplayPath && (file.DisplayPath != "tracked.bin" || file.OldPath == "tracked.bin"), "binary paths = %#v", file)
		testutil.Requiref(t, len(file.Hunks) == 0, "binary file %q has diff lines: %#v", file.DisplayPath, file.Hunks)
		testutil.Requiref(t, strings.Contains(strings.ToLower(strings.Join(file.Metadata, "\n")), "binary"), "binary file %q lacks metadata: %#v", file.DisplayPath, file.Metadata)
	}
}

func TestGetBranchIncludesCommittedStagedUnstagedAndUntrackedChanges(t *testing.T) {
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

	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	testutil.Require(t, err == nil, err)

	testutil.Requiref(t, got.Root == repo && got.Kind == Branch && got.Branch == "main", "patch metadata = %#v", got)

	want := []string{"committed.txt", "mixed.txt", "staged.txt", "untracked.txt"}
	testutil.Requiref(t, len(got.Files) == len(want), "files = %d, want %d: %#v", len(got.Files), len(want), got.Files)
	for i, name := range want {
		testutil.Requiref(t, got.Files[i].DisplayPath == name, "file %d = %q, want %q", i, got.Files[i].DisplayPath, name)
	}
	testutil.Requiref(t, containsKind(got.Files[0], Addition, "committed on feature") &&
		containsKind(got.Files[1], Addition, "unstaged on feature") &&
		containsKind(got.Files[2], Addition, "staged on feature") &&
		containsKind(got.Files[3], Addition, "untracked on feature"), "branch diff lacks expected changes: %#v", got.Files)
	testutil.Requiref(t, got.Files[0].OldSource == "base\n" && got.Files[0].NewSource == "committed on feature\n", "unexpected branch sources: old=%q new=%q", got.Files[0].OldSource, got.Files[0].NewSource)
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
			got, err := defaultBranch(context.Background(), runner, root)
			testutil.Require(t, err == nil, err)
			testutil.Requiref(t, got == tt.want, "default branch = %q, want %q", got, tt.want)
			assertBranchDiscoveryCalls(t, runner.calls)
		})
	}
}

func TestGetBranchKeepsPathsForAdditionsDeletionsAndRenames(t *testing.T) {
	repo := newRepository(t)
	git(t, repo, "branch", "-M", "main")
	writeFile(t, repo, "deleted.txt", "delete this\n")
	writeFile(t, repo, "old.go", "package main\n\nfunc example() {}\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")
	git(t, repo, "switch", "-c", "feature")
	git(t, repo, "mv", "old.go", "new.go")
	git(t, repo, "rm", "deleted.txt")
	writeFile(t, repo, "added.txt", "new content\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "changes")
	writeFile(t, repo, "untracked.txt", "untracked content\n")
	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	testutil.Require(t, err == nil, err)
	want := []struct {
		display string
		old     string
		new     string
	}{
		{"added.txt", "", "added.txt"},
		{"deleted.txt", "deleted.txt", ""},
		{"new.go", "old.go", "new.go"},
		{"untracked.txt", "", "untracked.txt"},
	}
	testutil.Requiref(t, len(got.Files) == len(want), "files = %#v", got.Files)
	for i, paths := range want {
		file := got.Files[i]
		testutil.Assertf(t, file.DisplayPath == paths.display && file.OldPath == paths.old && file.NewPath == paths.new, "file %d paths = %#v, want %#v", i, file, paths)
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

func TestGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")

	testutil.NoError(t, os.WriteFile(outside, []byte("do not read"), 0o600))

	testutil.NoError(t, os.Symlink(outside, filepath.Join(repo, "link")))

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(got.Files) == 1, "files = %d, want 1", len(got.Files))
	testutil.Requiref(t, got.Files[0].NewSource == outside, "symlink source = %q, want link target %q", got.Files[0].NewSource, outside)
	testutil.Requiref(t, got.Files[0].OldPath == "" && got.Files[0].NewPath == "link", "symlink paths = %#v", got.Files[0])
	testutil.Require(t, !strings.Contains(got.Files[0].NewSource, "do not read"), "symlink target contents were read")
}

func TestParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := parseHunkBody(10, 20, []byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"))
	testutil.Require(t, err == nil, err)
	want := []Line{
		{Kind: Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: Deletion, Text: "old", OldNumber: 11},
		{Kind: Addition, Text: "new", NewNumber: 21},
		{Kind: Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	testutil.Requiref(t, len(lines) == len(want), "lines = %#v", lines)
	for i := range want {
		testutil.Requiref(t, lines[i] == want[i], "line %d = %#v, want %#v", i, lines[i], want[i])
	}
}

func TestParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := parseHunkBody(1, 1, []byte("+hello\x1b[2J\n"))
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, lines[0].Text == `hello\x1b[2J`, "text = %q", lines[0].Text)
}

func TestParseHunkBodyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       []Line
		wantError  bool
	}{
		{name: "empty", want: []Line{}},
		{name: "unterminated", body: "+added", want: []Line{{Kind: Addition, Text: "added", NewNumber: 1}}},
		{name: "terminated", body: "+added\n", want: []Line{{Kind: Addition, Text: "added", NewNumber: 1}}},
		{name: "newline marker", body: "\\ No newline at end of file\n", want: []Line{}},
		{name: "empty diff line", body: "+added\n\n", wantError: true},
		{name: "unknown prefix", body: "?added\n", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHunkBody(1, 1, []byte(tc.body))
			testutil.Requiref(t, (err != nil) == tc.wantError, "error=%v, wantError=%v", err, tc.wantError)
			testutil.Requiref(t, reflect.DeepEqual(got, tc.want), "lines=%#v, want %#v", got, tc.want)
		})
	}
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(t *testing.T, body []byte) {
		_, _ = parseHunkBody(1, 1, body)
	})
}

func containsKind(file File, kind LineKind, text string) bool {
	for _, hunk := range file.Hunks {
		if hunkContainsKind(hunk, kind, text) {
			return true
		}
	}
	return false
}

func hunkContainsKind(hunk Hunk, kind LineKind, text string) bool {
	for _, line := range hunk.Lines {
		if line.Kind == kind && strings.Contains(line.Text, text) {
			return true
		}
	}
	return false
}

func newRepository(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	testutil.Require(t, err == nil, err)
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
		testutil.Requiref(t, err == nil, "git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)

	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))

	testutil.NoError(t, os.WriteFile(path, []byte(content), 0o644))

}

func assertUntrackedLines(t *testing.T, file File) {
	t.Helper()
	for _, hunk := range file.Hunks {
		assertAddedHunk(t, hunk)
	}
}

func assertAddedHunk(t *testing.T, hunk Hunk) {
	t.Helper()
	for _, line := range hunk.Lines {
		testutil.Requiref(t, line.Kind == Addition && line.OldNumber == 0 && line.NewNumber != 0, "unexpected untracked line: %#v", line)
	}
}

func assertBranchDiscoveryCalls(t *testing.T, calls []string) {
	t.Helper()
	for _, call := range calls {
		testutil.Requiref(t, !strings.Contains(call, "for-each-ref") && !strings.Contains(call, "merge-base") && !strings.Contains(call, "rev-list"), "default branch discovery inspected refs: git %s", call)
	}
}
