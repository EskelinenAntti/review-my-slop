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
)

func TestGetIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := newRepository(t)
	writeFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 1 }\n",
	)
	writeFile(t, repo, fixtureStagedTxt, "before\n")
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)

	writeFile(
		t,
		repo,
		"modified.go",
		"package main\n\nfunc value() int { return 2 }\n",
	)
	writeFile(t, repo, fixtureStagedTxt, "after\n")
	git(t, repo, gitAddCommand, fixtureStagedTxt)
	writeFile(t, repo, "new.py", "def hello():\n    return 'world'\n")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)

	assertUnstagedFileList(t, got, repo)
	modified := got.Files[zero]
	requireEqual(
		t,
		containsKind(modified, Deletion, "return 1") &&
			containsKind(modified, Addition, "return 2"),
		true,
		"modified file lacks expected lines: %#v",
		modified.Hunks,
	)
	untracked := got.Files[one]
	requireEqual(
		t,
		untracked.OldSource == "" &&
			strings.Contains(untracked.NewSource, "def hello"),
		true,
		"unexpected untracked sources: %#v",
		untracked,
	)
	assertUntrackedLines(t, untracked)
}

func TestAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := addedFile("odd\nname.go", "package main\n")
	requireEqual(
		t,
		file.NewPath, "odd\nname.go",
		"raw path = %q",
		file.NewPath,
	)
	requireEqual(
		t,
		file.DisplayPath, `odd\nname.go`,
		"display path = %q",
		file.DisplayPath,
	)
}

func TestGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "nested/odd\nname.go", "package main\n")
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		got.Root == repo && len(got.Files) == one, true,
		"patch = %#v",
		got,
	)
	file := got.Files[zero]
	requireEqual(
		t,
		file.OldPath == "" && file.NewPath == "nested/odd\nname.go" &&
			file.DisplayPath == `nested/odd\nname.go`,
		true,
		"file paths = %#v",
		file,
	)
}

func TestGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(newRepository(t))
	local, err := Get(context.Background(), Unstaged)
	requireEqual(
		t,
		err == nil && local.Branch == "", true,
		"local patch = %#v, error = %v",
		local,
		err,
	)
	for _, test := range []struct {
		kind    Kind
		message string
	}{
		{Branch, "default branch is unavailable"},
		{Kind(unknownPatchKind), "unknown patch kind"},
	} {
		got, err := Get(context.Background(), test.kind)
		requireEqual(
			t,
			err != nil && strings.Contains(err.Error(), test.message) &&
				reflect.DeepEqual(got, Patch{}),
			true,
			"kind %v: patch = %#v, error = %v",
			test.kind,
			got,
			err,
		)
	}
}

func TestGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := Get(context.Background(), Unstaged)
	requireEqual(
		t,
		err != nil && reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v",
		got,
		err,
	)
}

func TestGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(newRepository(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := Get(ctx, Unstaged)
	requireEqual(
		t,
		errors.Is(err, context.Canceled) && reflect.DeepEqual(got, Patch{}),
		true,
		"patch = %#v, error = %v",
		got,
		err,
	)
}

func TestGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, "large.txt", strings.Repeat("x", maxFileBytes+one))
	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(t, len(got.Files), one, "files = %#v", got.Files)
	file := got.Files[zero]
	requireEqual(
		t,
		file.OldPath == "" && file.NewPath == "large.txt" &&
			file.NewSource == "" &&
			len(file.Hunks) == zero &&
			strings.Contains(
				strings.Join(file.Metadata, "\n"),
				"exceeds 2 MiB",
			),
		true,
		"large file = %#v",
		file,
	)
}

func TestGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := newRepository(t)
	writeFile(t, repo, fixtureTrackedBin, "\x00old")
	git(t, repo, gitAddCommand, fixtureTrackedBin)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)
	writeFile(t, repo, fixtureTrackedBin, "\x00new")
	writeFile(t, repo, "untracked.bin", "\x00content")

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(got.Files), two,
		"files = %d, want 2: %#v",
		len(got.Files),
		got.Files,
	)
	for _, file := range got.Files {
		checkEqual(
			t,
			file.NewPath == file.DisplayPath &&
				(file.DisplayPath != fixtureTrackedBin ||
					file.OldPath == fixtureTrackedBin),
			true,
			"binary paths = %#v",
			file,
		)
		requireEqual(
			t,
			len(file.Hunks), zero,
			"binary file %q has diff lines: %#v",
			file.DisplayPath,
			file.Hunks,
		)
		requireEqual(
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
	repo := branchRepository(t)
	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	requireEqual(t, err == nil, true, fixtureV, err)

	requireEqual(
		t,
		got.Root == repo && got.Kind == Branch && got.Branch == fixtureMain,
		true,
		"patch metadata = %#v",
		got,
	)

	want := []string{
		fixtureCommittedTxt,
		fixtureMixedTxt,
		fixtureStagedTxt,
		fixtureUntrackedTxt,
	}
	requireEqual(
		t,
		len(got.Files), len(want),
		"files = %d, want %d: %#v",
		len(got.Files),
		len(want),
		got.Files,
	)
	for i, name := range want {
		requireEqual(
			t,
			got.Files[i].DisplayPath, name,
			"file %d = %q, want %q",
			i,
			got.Files[i].DisplayPath,
			name,
		)
	}
	requireEqual(
		t,
		containsKind(got.Files[zero], Addition, "committed on feature") &&
			containsKind(got.Files[one], Addition, "unstaged on feature") &&
			containsKind(got.Files[two], Addition, "staged on feature") &&
			containsKind(got.Files[three], Addition, "untracked on feature"),
		true,
		"branch diff lacks expected changes: %#v",
		got.Files,
	)
	requireEqual(
		t,
		got.Files[zero].OldSource == fixtureBaseContent &&
			got.Files[zero].NewSource == "committed on feature\n",
		true,
		"unexpected branch sources: old=%q new=%q",
		got.Files[zero].OldSource,
		got.Files[zero].NewSource,
	)
}

func TestDefaultBranchFallbacks(t *testing.T) {
	tests := []struct {
		name       string
		originHEAD string
		available  string
		want       string
	}{
		{
			name:       "origin HEAD",
			originHEAD: "origin/trunk",
			want:       "origin/trunk",
		},

		{name: "origin main", available: "origin/main", want: "origin/main"},

		{name: "local main", available: fixtureMain, want: fixtureMain},

		{
			name:      "origin master",
			available: "origin/master",
			want:      "origin/master",
		},

		{name: "local master", available: "master", want: "master"},

		{name: "none"},
	}
	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				root := t.TempDir()
				runner := &defaultBranchRunner{
					root:       root,
					originHEAD: tt.originHEAD,
					available:  tt.available,
				}
				got, err := defaultBranch(context.Background(), runner, root)
				requireEqual(t, err == nil, true, fixtureV, err)
				requireEqual(
					t,
					got, tt.want,
					"default branch = %q, want %q",
					got,
					tt.want,
				)
				for _, call := range runner.calls {
					requireEqual(
						t,
						!strings.Contains(call, "for-each-ref") &&
							!strings.Contains(call, "merge-base") &&
							!strings.Contains(call, "rev-list"),
						true,
						"default branch discovery inspected refs: git %s",
						call,
					)
				}
			},
		)
	}
}

func TestGetBranchKeepsPathsForAdditionsDeletionsAndRenames(t *testing.T) {
	repo := newRepository(t)
	git(t, repo, "branch", "-M", fixtureMain)
	writeFile(t, repo, fixtureDeletedTxt, "delete this\n")
	writeFile(t, repo, fixtureOldGo, "package main\n\nfunc example() {}\n")
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)
	git(t, repo, "switch", "-c", "feature")
	git(t, repo, "mv", fixtureOldGo, fixtureNewGo)
	git(t, repo, "rm", fixtureDeletedTxt)
	writeFile(t, repo, fixtureAddedTxt, "new content\n")
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, "changes")
	writeFile(t, repo, fixtureUntrackedTxt, "untracked content\n")
	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	requireEqual(t, err == nil, true, fixtureV, err)
	want := []struct {
		display string
		old     string
		new     string
	}{
		{fixtureAddedTxt, "", fixtureAddedTxt},
		{fixtureDeletedTxt, fixtureDeletedTxt, ""},
		{fixtureNewGo, fixtureOldGo, fixtureNewGo},
		{fixtureUntrackedTxt, "", fixtureUntrackedTxt},
	}
	requireEqual(t, len(got.Files), len(want), "files = %#v", got.Files)
	for i, paths := range want {
		file := got.Files[i]
		checkEqual(
			t,
			file.DisplayPath == paths.display && file.OldPath == paths.old &&
				file.NewPath == paths.new,
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

func (r *defaultBranchRunner) Run(_ context.Context, _ string, args ...string) (

	[]byte,

	error,
) {
	call := strings.Join(args, " ")
	r.calls = append(r.calls, call)
	switch {
	case call == "rev-parse --show-toplevel":
		return []byte(r.root + "\n"), nil
	case call == "symbolic-ref --quiet --short refs/remotes/origin/HEAD" &&
		r.originHEAD != "":
		return []byte(r.originHEAD + "\n"), nil
	case r.matchesAvailableBranch(args):
		return []byte("commit\n"), nil
	default:
		return nil, exec.ErrNotFound
	}
}

func TestGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := newRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	{
		err := os.WriteFile(outside, []byte("do not read"), privateFileMode)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		err := os.Symlink(outside, filepath.Join(repo, "link"))
		requireEqual(t, err == nil, true, fixtureV, err)
	}

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(t, len(got.Files), one, "files = %d, want 1", len(got.Files))
	requireEqual(
		t,
		got.Files[zero].NewSource, outside,
		"symlink source = %q, want link target %q",
		got.Files[zero].NewSource,
		outside,
	)
	requireEqual(
		t,
		got.Files[zero].OldPath == "" && got.Files[zero].NewPath == "link",
		true,
		"symlink paths = %#v",
		got.Files[zero],
	)
	requireEqual(
		t,
		!strings.Contains(got.Files[zero].NewSource, "do not read"),
		true,
		fixtureV,
		"symlink target contents were read",
	)
}

func TestParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := parseHunkBody(
		ten,
		minimumWidth,
		[]byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"),
	)
	requireEqual(t, err == nil, true, fixtureV, err)
	want := []Line{
		{
			Kind:      Context,
			Text:      "context",
			OldNumber: ten,
			NewNumber: minimumWidth,
		},

		{Kind: Deletion, Text: "old", OldNumber: deletedSourceLine},

		{Kind: Addition, Text: "new", NewNumber: addedSourceLine},

		{
			Kind:      Context,
			Text:      "same",
			OldNumber: trailingOldSourceLine,
			NewNumber: trailingNewSourceLine,
		},
	}
	requireEqual(t, len(lines), len(want), "lines = %#v", lines)
	for i := range want {
		requireEqual(
			t,
			lines[i], want[i],
			"line %d = %#v, want %#v",
			i,
			lines[i],
			want[i],
		)
	}
}

func TestParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := parseHunkBody(one, one, []byte("+hello\x1b[2J\n"))
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		lines[zero].Text, `hello\x1b[2J`,
		"text = %q",
		lines[zero].Text,
	)
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(_ *testing.T, body []byte) {
		_, _ = parseHunkBody(one, one, body)
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
	requireEqual(t, err == nil, true, fixtureV, err)
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
		requireEqual(
			t,
			err == nil, true,
			"git %s: %v\n%s",
			strings.Join(args, " "),
			err,
			out,
		)
	}
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	{
		err := os.MkdirAll(filepath.Dir(path), executableDirMode)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	{
		err := os.WriteFile(path, []byte(content), publicFileMode)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
}

func (r *defaultBranchRunner) matchesAvailableBranch(args []string) bool {
	return len(args) == four && args[zero] == "rev-parse" &&
		args[one] == "--verify" &&
		args[two] == "--quiet" &&
		args[three] == r.available+"^{commit}" &&
		r.available != ""
}

func assertUntrackedLines(t *testing.T, untracked File) {
	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			requireEqual(
				t,
				line.Kind == Addition && line.OldNumber == zero &&
					line.NewNumber != zero,
				true,
				"unexpected untracked line: %#v",
				line,
			)
		}
	}
}

func branchRepository(t *testing.T) string {
	repo := newRepository(t)
	git(t, repo, "branch", "-M", fixtureMain)
	writeFile(t, repo, fixtureCommittedTxt, fixtureBaseContent)
	writeFile(t, repo, fixtureMixedTxt, fixtureBaseContent)
	writeFile(t, repo, fixtureStagedTxt, fixtureBaseContent)
	git(t, repo, gitAddCommand, currentDirectory)
	git(t, repo, gitCommitCommand, gitMessageFlag, fixtureBaseCommit)

	git(t, repo, "switch", "-c", "feature")
	writeFile(t, repo, fixtureCommittedTxt, "committed on feature\n")
	git(t, repo, gitAddCommand, fixtureCommittedTxt)
	git(t, repo, gitCommitCommand, gitMessageFlag, "feature commit")
	writeFile(t, repo, fixtureStagedTxt, "staged on feature\n")
	git(t, repo, gitAddCommand, fixtureStagedTxt)
	writeFile(t, repo, fixtureMixedTxt, "unstaged on feature\n")
	writeFile(t, repo, fixtureUntrackedTxt, "untracked on feature\n")

	return repo
}

func assertUnstagedFileList(t *testing.T, got Patch, repo string) {
	requireEqual(
		t,
		got.Root == repo && got.Kind == Unstaged, true,
		"root = %q, kind = %v",
		got.Root,
		got.Kind,
	)
	requireEqual(
		t,
		len(got.Files), two,
		"files = %d, want 2: %#v",
		len(got.Files),
		got.Files,
	)
	requireEqual(
		t,
		got.Files[zero].DisplayPath == "modified.go" &&
			got.Files[one].DisplayPath == "new.py",
		true,
		"unexpected files: %q, %q",
		got.Files[zero].DisplayPath,
		got.Files[one].DisplayPath,
	)
}
