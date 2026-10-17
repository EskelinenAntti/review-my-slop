package patch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sourcegraph/go-diff/diff"
)

func TestPatchGetIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := createGitTestRepository(t)
	writeGitTestFile(t, repo, "modified.go",
		"package main\n\nfunc value() int { return 1 }\n")
	writeGitTestFile(t, repo, "staged.txt", "before\n")
	runGitTestCommand(t, repo, "add", ".")
	runGitTestCommand(t, repo, "commit", "-m", "base")
	writeGitTestFile(t, repo, "modified.go",
		"package main\n\nfunc value() int { return 2 }\n")
	writeGitTestFile(t, repo, "staged.txt", "after\n")
	runGitTestCommand(t, repo, "add", "staged.txt")
	writeGitTestFile(t, repo, "new.py", "def hello():\n    return 'world'\n")

	t.Chdir(repo)
	got, err := loadPatch(context.Background(), Unstaged)
	requireNoError(t, err)
	requireEqual(t, got.Root, repo, "root = %q, kind = %v", got.Root, got.Kind)
	requireEqual(t, got.Kind, Unstaged, "root = %q, kind = %v",
		got.Root, got.Kind,
	)
	requireEqual(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)
	requireEqual(t, got.Files[0].DisplayPath, "modified.go",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)
	requireEqual(t, got.Files[1].DisplayPath, "new.py",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)

	modified := got.Files[0]
	requireEqual(t, patchContainsLine(
		modified,
		Deletion,
		"return 1",
	), true,
		"modified file lacks expected lines: %#v", modified.Hunks,
	)
	requireEqual(t, patchContainsLine(
		modified,
		Addition,
		"return 2",
	), true,
		"modified file lacks expected lines: %#v", modified.Hunks)

	untracked := got.Files[1]
	requireEqual(t, untracked.OldSource, "",
		"unexpected untracked sources: %#v", untracked,
	)
	requireEqual(t, strings.Contains(untracked.NewSource, "def hello"),
		true, "unexpected untracked sources: %#v", untracked,
	)

	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			requireEqual(t, line.Kind, Addition,
				"unexpected untracked line: %#v", line,
			)
			requireEqual(t, line.OldNumber, 0,
				"unexpected untracked line: %#v", line,
			)
			requireEqual(t, line.NewNumber != 0, true,
				"unexpected untracked line: %#v", line,
			)
		}
	}
}

func TestPatchAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := func() File {
		var path, content = "odd\nname.go", "package main\n"
		sourceLines := func() []string {
			var content = content
			content = strings.TrimSuffix(content, "\n")
			if content == "" {
				return nil
			}
			return strings.Split(content, "\n")
		}()
		lines := make([]Line, 0, len(sourceLines))
		for i, line := range sourceLines {
			lines = append(lines, Line{
				Kind:      Addition,
				Text:      line,
				NewNumber: LineNumber(i + 1),
			})
		}
		return File{NewPath: path, DisplayPath: strings.ReplaceAll(
			escapeDisplayText(path),
			"\n",
			`\n`,
		), NewSource: content, Metadata: []string{
			"untracked file",
		}, Hunks: []Hunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(
			lines,
		)), Lines: lines}}}
	}()
	requireEqual(t, file.NewPath, "odd\nname.go", "raw path = %q", file.NewPath)
	requireEqual(t, file.DisplayPath, `odd\nname.go`,
		"display path = %q", file.DisplayPath,
	)
}

func TestPatchGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := createGitTestRepository(t)
	t.Helper()
	path := filepath.Join(repo, "nested/odd\nname.go")
	requireNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	requireNoError(t, os.WriteFile(path, []byte("package main\n"), 0o644))
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := loadPatch(context.Background(), Unstaged)
	requireNoError(t, err)
	requireEqual(t, got.Root, repo, "patch = %#v", got)
	requireEqual(t, len(got.Files), 1, "patch = %#v", got)

	file := got.Files[0]
	requireEqual(t, file.OldPath, "", "file paths = %#v", file)
	requireEqual(t, file.NewPath, "nested/odd\nname.go",
		"file paths = %#v", file,
	)
	requireEqual(t, file.DisplayPath, `nested/odd\nname.go`,
		"file paths = %#v", file,
	)
}

func TestPatchGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(createGitTestRepository(t))
	local, err := loadPatch(context.Background(), Unstaged)
	requireEqual(t, err, nil, "local patch = %#v, error = %v", local, err)
	requireEqual(t, local.Branch, "", "local patch = %#v, error = %v",
		local, err,
	)

	for _, test := range []struct {
		kind    Kind
		message string
	}{
		{Branch, "default branch is unavailable"},
		{Kind(255), "unknown patch kind"},
	} {
		got, err := loadPatch(context.Background(), test.kind)
		requireEqual(t, err != nil, true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
		requireEqual(t, strings.Contains(err.Error(), test.message),
			true, "kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
		requireEqual(t, reflect.DeepEqual(got, Patch{}), true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
	}
}

func TestPatchGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := loadPatch(context.Background(), Unstaged)
	requireEqual(t, err != nil, true, "patch = %#v, error = %v", got, err)
	requireEqual(t, reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestPatchGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(createGitTestRepository(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := loadPatch(ctx, Unstaged)
	requireEqual(t, errors.Is(err, context.Canceled), true,
		"patch = %#v, error = %v", got, err,
	)
	requireEqual(t, reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestPatchGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := createGitTestRepository(t)
	var content = strings.Repeat("x", maxFileBytes+1)
	t.Helper()
	path := filepath.Join(repo, "large.txt")
	requireNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	requireNoError(t, os.WriteFile(path, []byte(content), 0o644))
	t.Chdir(repo)
	got, err := loadPatch(context.Background(), Unstaged)
	requireNoError(t, err)
	requireEqual(t, len(got.Files), 1, "files = %#v", got.Files)

	file := got.Files[0]
	requireEqual(t, file.OldPath, "", "large file = %#v", file)
	requireEqual(t, file.NewPath, "large.txt", "large file = %#v", file)
	requireEqual(t, file.NewSource, "", "large file = %#v", file)
	requireEqual(t, len(file.Hunks), 0, "large file = %#v", file)
	requireEqual(t,
		strings.Contains(
			strings.Join(file.Metadata, "\n"),
			"exceeds 2 MiB",
		), true, "large file = %#v", file)
}

func TestPatchGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := createGitTestRepository(t)
	t.Helper()
	path := filepath.Join(repo, "tracked.bin")
	requireNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	requireNoError(t, os.WriteFile(path, []byte("\x00old"), 0o644))
	var args = []string{"add", "tracked.bin"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	{
		var args = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "tracked.bin")
		requireNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		requireNoError(t, os.WriteFile(path, []byte("\x00new"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "untracked.bin")
		requireNoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		requireNoError(t, os.WriteFile(path, []byte("\x00content"), 0o644))
	}

	t.Chdir(repo)
	got, err := loadPatch(context.Background(), Unstaged)
	requireNoError(t, err)
	requireEqual(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)

	for _, file := range got.Files {
		checkEqual(
			t,
			file.NewPath == file.DisplayPath &&
				(file.DisplayPath != "tracked.bin" ||
					file.OldPath == "tracked.bin"),
			true,
			"binary paths = %#v",
			file,
		)
		requireEqual(t, len(file.Hunks), 0,
			"binary file %q has diff lines: %#v", file.DisplayPath, file.Hunks,
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

func TestPatchGetBranchIncludesCommittedStagedUnstagedAndUntrackedChanges(
	t *testing.T,
) {
	repo := createBranchChangesTestRepository(t)

	t.Chdir(repo)
	got, err := loadPatch(context.Background(), Branch)
	requireNoError(t, err)
	requireEqual(t, got.Root, repo, "patch metadata = %#v", got)
	requireEqual(t, got.Kind, Branch, "patch metadata = %#v", got)
	requireEqual(t, got.Branch, "main", "patch metadata = %#v", got)

	want := []string{
		"committed.txt",
		"mixed.txt",
		"staged.txt",
		"untracked.txt",
	}
	requireEqual(t, len(got.Files), len(want),
		"files = %d, want %d: %#v", len(got.Files), len(want), got.Files,
	)

	for i, name := range want {
		requireEqual(t, got.Files[i].DisplayPath, name,
			"file %d = %q, want %q", i, got.Files[i].DisplayPath, name,
		)
	}
	requireEqual(
		t,
		patchContainsLine(got.Files[0], Addition, "committed on feature"),
		true,
		"branch diff lacks expected changes: %#v", got.Files,
	)
	requireEqual(
		t,
		patchContainsLine(got.Files[1], Addition, "unstaged on feature"),
		true, "branch diff lacks expected changes: %#v", got.Files,
	)
	requireEqual(
		t,
		patchContainsLine(got.Files[2], Addition, "staged on feature"),
		true, "branch diff lacks expected changes: %#v", got.Files,
	)
	requireEqual(
		t,
		patchContainsLine(got.Files[3], Addition, "untracked on feature"),
		true, "branch diff lacks expected changes: %#v", got.Files,
	)
	requireEqual(t, got.Files[0].OldSource, "base\n",
		"unexpected branch sources: old=%q new=%q", got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
	requireEqual(t, got.Files[0].NewSource, "committed on feature\n",
		"unexpected branch sources: old=%q new=%q", got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
}

func TestPatchDefaultBranchFallbacks(t *testing.T) {
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
			commandRunner := &patchDefaultBranchRunner{
				root:       root,
				originHEAD: tt.originHEAD,
				available:  tt.available,
			}
			got, err := findDefaultBranchWithRunner(
				context.Background(), commandRunner, root,
			)
			requireNoError(t, err)
			requireEqual(t, got, tt.want,
				"default branch = %q, want %q", got, tt.want,
			)

			for _, call := range commandRunner.calls {
				requireEqual(t,
					!strings.Contains(call, "for-each-ref"), true,
					"default branch discovery inspected refs: git %s", call,
				)
				requireEqual(t, !strings.Contains(call, "merge-base"),
					true, "default branch discovery inspected refs: git %s",
					call,
				)
				requireEqual(t, !strings.Contains(call, "rev-list"),
					true, "default branch discovery inspected refs: git %s",
					call,
				)
			}
		})
	}
}

func TestPatchGetBranchKeepsPathsForAdditionsDeletionsAndRenames(t *testing.T) {
	repo := createBranchPathChangesTestRepository(t)
	t.Chdir(repo)
	got, err := loadPatch(context.Background(), Branch)
	requireNoError(t, err)

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
	requireEqual(t, len(got.Files), len(want), "files = %#v", got.Files)

	for i, paths := range want {
		file := got.Files[i]
		checkEqual(
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

type patchDefaultBranchRunner struct {
	root       string
	originHEAD string
	available  string
	calls      []string
}

func (r *patchDefaultBranchRunner) Run(
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

func TestPatchGetDoesNotFollowUntrackedSymlink(t *testing.T) {
	repo := createGitTestRepository(t)
	outside := filepath.Join(t.TempDir(), "secret")
	requireNoError(t, os.WriteFile(outside, []byte("do not read"), 0o600))
	requireNoError(t, os.Symlink(outside, filepath.Join(repo, "link")))

	t.Chdir(repo)
	got, err := loadPatch(context.Background(), Unstaged)
	requireNoError(t, err)
	requireEqual(t, len(got.Files), 1, "files = %d, want 1", len(got.Files))
	requireEqual(t, got.Files[0].NewSource, outside,
		"symlink source = %q, want link target %q", got.Files[0].NewSource,
		outside,
	)
	requireEqual(t, got.Files[0].OldPath, "", "symlink paths = %#v",
		got.Files[0],
	)
	requireEqual(t, got.Files[0].NewPath, "link",
		"symlink paths = %#v", got.Files[0],
	)
	requireEqual(t,
		!strings.Contains(got.Files[0].NewSource, "do not read"), true, "%s",
		"symlink target contents were read",
	)
}

func TestPatchParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := parseDiffHunkLines(&diff.Hunk{
		OrigStartLine: 10,
		NewStartLine:  20,
		Body: []byte(
			" context\n-old\n+new\n same\n\\ No newline at end of file\n",
		),
	})
	requireNoError(t, err)

	want := []Line{
		{Kind: Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: Deletion, Text: "old", OldNumber: 11},
		{Kind: Addition, Text: "new", NewNumber: 21},
		{Kind: Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	requireEqual(t, len(lines), len(want), "lines = %#v", lines)

	for i := range want {
		requireEqual(t, lines[i], want[i], "line %d = %#v, want %#v",
			i, lines[i], want[i],
		)
	}
}

func TestPatchParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := parseDiffHunkLines(&diff.Hunk{
		OrigStartLine: 1,
		NewStartLine:  1,
		Body:          []byte("+hello\x1b[2J\n"),
	})
	requireNoError(t, err)
	requireEqual(t, lines[0].Text, `hello\x1b[2J`, "text = %q", lines[0].Text)
}
