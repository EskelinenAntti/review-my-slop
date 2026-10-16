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
	repo := createRepositoryWithUnstagedChanges(t)

	t.Chdir(repo)
	got, err := GetPatch(context.Background(), Unstaged)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(
		t,
		got.Root,
		repo,
		"root = %q, kind = %v",
		got.Root,
		got.Kind,
	)
	testutil.RequireValuesEqual(t, got.Kind, Unstaged, "root = %q, kind = %v",
		got.Root, got.Kind,
	)
	testutil.RequireValuesEqual(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)
	testutil.RequireValuesEqual(t, got.Files[0].DisplayPath, "modified.go",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)
	testutil.RequireValuesEqual(t, got.Files[1].DisplayPath, "new.py",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)

	modified := got.Files[0]
	testutil.RequireValuesEqual(
		t,
		containsKind(modified, Deletion, "return 1"),
		true,
		"modified file lacks expected lines: %#v",
		modified.Hunks,
	)
	testutil.RequireValuesEqual(t,
		containsKind(
			modified,
			Addition,
			"return 2",
		), true, "modified file lacks expected lines: %#v", modified.Hunks)

	untracked := got.Files[1]
	testutil.RequireValuesEqual(t, untracked.OldSource, "",
		"unexpected untracked sources: %#v", untracked,
	)
	testutil.RequireValuesEqual(
		t,
		strings.Contains(untracked.NewSource, "def hello"),
		true,
		"unexpected untracked sources: %#v",
		untracked,
	)

	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			testutil.RequireValuesEqual(t, line.Kind, Addition,
				"unexpected untracked line: %#v", line,
			)
			testutil.RequireValuesEqual(t, line.OldNumber, 0,
				"unexpected untracked line: %#v", line,
			)
			testutil.RequireValuesEqual(t, line.NewNumber != 0, true,
				"unexpected untracked line: %#v", line,
			)
		}
	}
}

func createRepositoryWithUnstagedChanges(t *testing.T) string {
	t.Helper()
	repo := newRepository(t)
	writeFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 1 }\n",
	)
	writeFile(t, repo, "staged.txt", "before\n")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-m", "base")
	writeFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 2 }\n",
	)
	writeFile(t, repo, "staged.txt", "after\n")
	git(t, repo, "add", "staged.txt")
	writeFile(t, repo, "new.py", "def hello():\n    return 'world'\n")
	return repo
}

func TestAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := makeFileFromText("odd\nname.go", "package main\n")
	testutil.RequireValuesEqual(
		t,
		file.NewPath,
		"odd\nname.go",
		"raw path = %q",
		file.NewPath,
	)
	testutil.RequireValuesEqual(t, file.DisplayPath, `odd\nname.go`,
		"display path = %q", file.DisplayPath,
	)
}

func TestGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "nested/odd\nname.go", "package main\n")
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := GetPatch(context.Background(), Unstaged)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, got.Root, repo, "patch = %#v", got)
	testutil.RequireValuesEqual(t, len(got.Files), 1, "patch = %#v", got)

	file := got.Files[0]
	testutil.RequireValuesEqual(t, file.OldPath, "", "file paths = %#v", file)
	testutil.RequireValuesEqual(t, file.NewPath, "nested/odd\nname.go",
		"file paths = %#v", file,
	)
	testutil.RequireValuesEqual(t, file.DisplayPath, `nested/odd\nname.go`,
		"file paths = %#v", file,
	)
}

func TestGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(newRepository(t))
	local, err := GetPatch(context.Background(), Unstaged)
	testutil.RequireValuesEqual(
		t,
		err,
		nil,
		"local patch = %#v, error = %v",
		local,
		err,
	)
	testutil.RequireValuesEqual(
		t,
		local.Branch,
		"",
		"local patch = %#v, error = %v",
		local,
		err,
	)

	for _, test := range []struct {
		kind    Kind
		message string
	}{
		{Branch, "default branch is unavailable"},
		{Kind(255), "unknown patch kind"},
	} {
		got, err := GetPatch(context.Background(), test.kind)
		testutil.RequireValuesEqual(t, err != nil, true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
		testutil.RequireValuesEqual(
			t,
			strings.Contains(err.Error(), test.message),
			true,
			"kind %v: patch = %#v, error = %v",
			test.kind,
			got,
			err,
		)
		testutil.RequireValuesEqual(t, reflect.DeepEqual(got, Patch{}), true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
	}
}

func TestGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := GetPatch(context.Background(), Unstaged)
	testutil.RequireValuesEqual(
		t,
		err != nil,
		true,
		"patch = %#v, error = %v",
		got,
		err,
	)
	testutil.RequireValuesEqual(t, reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(newRepository(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := GetPatch(ctx, Unstaged)
	testutil.RequireValuesEqual(t, errors.Is(err, context.Canceled), true,
		"patch = %#v, error = %v", got, err,
	)
	testutil.RequireValuesEqual(t, reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "large.txt", strings.Repeat("x", maxFileBytes+1))
	t.Chdir(repo)
	got, err := GetPatch(context.Background(), Unstaged)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(got.Files), 1, "files = %#v", got.Files)

	file := got.Files[0]
	testutil.RequireValuesEqual(t, file.OldPath, "", "large file = %#v", file)
	testutil.RequireValuesEqual(
		t,
		file.NewPath,
		"large.txt",
		"large file = %#v",
		file,
	)
	testutil.RequireValuesEqual(t, file.NewSource, "", "large file = %#v", file)
	testutil.RequireValuesEqual(t, len(file.Hunks), 0, "large file = %#v", file)
	testutil.RequireValuesEqual(t,
		strings.Contains(
			strings.Join(file.Metadata, "\n"),
			"exceeds 2 MiB",
		), true, "large file = %#v", file)
}

func TestGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "tracked.bin", "\x00old")
	git(t, repo, "add", "tracked.bin")
	git(t, repo, "commit", "-m", "base")
	writeFile(t, repo, "tracked.bin", "\x00new")
	writeFile(t, repo, "untracked.bin", "\x00content")

	t.Chdir(repo)
	got, err := GetPatch(context.Background(), Unstaged)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)

	for _, file := range got.Files {
		testutil.CheckValuesEqual(
			t,
			file.NewPath == file.DisplayPath &&
				(file.DisplayPath != "tracked.bin" ||
					file.OldPath == "tracked.bin"),
			true,
			"binary paths = %#v",
			file,
		)
		testutil.RequireValuesEqual(t, len(file.Hunks), 0,
			"binary file %q has diff lines: %#v", file.DisplayPath, file.Hunks,
		)
		testutil.RequireValuesEqual(
			t,
			strings.Contains(
				strings.ToLower(strings.Join(file.Metadata, "\n")),
				"binary",
			),
			true,
			"binary file %q lacks metadata: %#v",
			file.DisplayPath,
			file.Metadata,
		)
	}
}

func TestGetBranchIncludesCommittedStagedUnstagedAndUntrackedChanges(
	t *testing.T,
) {
	repo := createBranchWithEveryChangeKind(t)

	t.Chdir(repo)
	got, err := GetPatch(context.Background(), Branch)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, got.Root, repo, "patch metadata = %#v", got)
	testutil.RequireValuesEqual(
		t,
		got.Kind,
		Branch,
		"patch metadata = %#v",
		got,
	)
	testutil.RequireValuesEqual(
		t,
		got.Branch,
		"main",
		"patch metadata = %#v",
		got,
	)

	want := []string{
		"committed.txt",
		"mixed.txt",
		"staged.txt",
		"untracked.txt",
	}
	testutil.RequireValuesEqual(t, len(got.Files), len(want),
		"files = %d, want %d: %#v", len(got.Files), len(want), got.Files,
	)

	for i, name := range want {
		testutil.RequireValuesEqual(t, got.Files[i].DisplayPath, name,
			"file %d = %q, want %q", i, got.Files[i].DisplayPath, name,
		)
	}
	testutil.RequireValuesEqual(t,
		containsKind(got.Files[0], Addition, "committed on feature"), true,
		"branch diff lacks expected changes: %#v", got.Files,
	)
	testutil.RequireValuesEqual(t,
		containsKind(
			got.Files[1],
			Addition,
			"unstaged on feature",
		), true, "branch diff lacks expected changes: %#v", got.Files)
	testutil.RequireValuesEqual(t,

		containsKind(
			got.Files[2],
			Addition,
			"staged on feature",
		), true, "branch diff lacks expected changes: %#v", got.Files)
	testutil.RequireValuesEqual(t,

		containsKind(
			got.Files[3],
			Addition,
			"untracked on feature",
		), true, "branch diff lacks expected changes: %#v", got.Files)
	testutil.RequireValuesEqual(t, got.Files[0].OldSource, "base\n",
		"unexpected branch sources: old=%q new=%q", got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
	testutil.RequireValuesEqual(
		t,
		got.Files[0].NewSource,
		"committed on feature\n",
		"unexpected branch sources: old=%q new=%q",
		got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
}

func createBranchWithEveryChangeKind(t *testing.T) string {
	t.Helper()
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
	return repo
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
		{
			name:      "origin master",
			available: "origin/master",
			want:      "origin/master",
		},
		{name: "local master", available: "master", want: "master"},
		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			runner := &defaultBranchRunner{
				root:       root,
				originHEAD: tt.originHEAD,
				available:  tt.available,
			}
			got, err := findDefaultBranch(context.Background(), runner, root)
			testutil.RequireNoError(t, err)
			testutil.RequireValuesEqual(t, got, tt.want,
				"default branch = %q, want %q", got, tt.want,
			)

			for _, call := range runner.calls {
				testutil.RequireValuesEqual(t,
					!strings.Contains(call, "for-each-ref"), true,
					"default branch discovery inspected refs: git %s", call,
				)
				testutil.RequireValuesEqual(
					t,
					!strings.Contains(call, "merge-base"),
					true,
					"default branch discovery inspected refs: git %s",
					call,
				)
				testutil.RequireValuesEqual(
					t,
					!strings.Contains(call, "rev-list"),
					true,
					"default branch discovery inspected refs: git %s",
					call,
				)
			}
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
	got, err := GetPatch(context.Background(), Branch)
	testutil.RequireNoError(t, err)

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
	testutil.RequireValuesEqual(t, len(got.Files), len(want), "files = %#v",
		got.Files,
	)

	for i, paths := range want {
		file := got.Files[i]
		testutil.CheckValuesEqual(
			t,
			file.DisplayPath == paths.display &&
				file.OldPath == paths.old && file.NewPath == paths.new,
			true,
			"file %d paths = %#v, want %#v",
			i,
			file,
			paths,
		)
	}
}

type defaultBranchRunner struct {
	root       string
	originHEAD string
	available  string
	calls      []string
}

func (r *defaultBranchRunner) Run(
	_ context.Context,
	_ string,
	args ...string,
) ([]byte, error) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case call == "symbolic-ref --quiet --short refs/remotes/origin/HEAD" &&
		r.originHEAD != "":
		return []byte(r.originHEAD + "\n"), nil
	case call == "rev-parse --verify --quiet "+r.available+"^{commit}" &&
		r.available != "":
		return []byte("commit\n"), nil
	default:
		return nil, exec.ErrNotFound
	}
}

func TestGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	testutil.RequireNoError(
		t,
		os.WriteFile(outside, []byte("do not read"), 0o600),
	)
	testutil.RequireNoError(t, os.Symlink(outside, filepath.Join(repo, "link")))

	t.Chdir(repo)
	got, err := GetPatch(context.Background(), Unstaged)
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, len(got.Files), 1, "files = %d, want 1",
		len(got.Files),
	)
	testutil.RequireValuesEqual(t, got.Files[0].NewSource, outside,
		"symlink source = %q, want link target %q", got.Files[0].NewSource,
		outside,
	)
	testutil.RequireValuesEqual(
		t,
		got.Files[0].OldPath,
		"",
		"symlink paths = %#v",
		got.Files[0],
	)
	testutil.RequireValuesEqual(t, got.Files[0].NewPath, "link",
		"symlink paths = %#v", got.Files[0],
	)
	testutil.RequireValuesEqual(t,
		!strings.Contains(got.Files[0].NewSource, "do not read"), true, "%s",
		"symlink target contents were read",
	)
}

func TestParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := parseChangedLines(
		10,
		20,
		[]byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"),
	)
	testutil.RequireNoError(t, err)

	want := []Line{
		{Kind: Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: Deletion, Text: "old", OldNumber: 11},
		{Kind: Addition, Text: "new", NewNumber: 21},
		{Kind: Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	testutil.RequireValuesEqual(t, len(lines), len(want), "lines = %#v", lines)

	for i := range want {
		testutil.RequireValuesEqual(
			t,
			lines[i],
			want[i],
			"line %d = %#v, want %#v",
			i,
			lines[i],
			want[i],
		)
	}
}

func TestParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := parseChangedLines(1, 1, []byte("+hello\x1b[2J\n"))
	testutil.RequireNoError(t, err)
	testutil.RequireValuesEqual(t, lines[0].Text, `hello\x1b[2J`, "text = %q",
		lines[0].Text,
	)
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(_ *testing.T, body []byte) {
		_, _ = parseChangedLines(1, 1, body)
	})
}

func containsKind(file File, kind LineKind, text string) bool {
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
	testutil.RequireNoError(t, err)

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
	testutil.RequireNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.RequireNoError(t, os.WriteFile(path, []byte(content), 0o644))
}
