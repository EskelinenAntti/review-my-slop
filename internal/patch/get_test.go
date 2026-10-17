package patch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestGetIncludesUnstagedAndUntrackedButNotStagedOnly(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	t.Helper()
	path := filepath.Join(repo, "modified.go")
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.NoError(t, os.WriteFile(path, []byte("package main\n\nfunc value() int { return 1 }\n"), 0o644))
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("before\n"), 0o644))
	}
	var args []string = []string{"add", "."}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	{
		var args []string = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	{
		t.Helper()
		path := filepath.Join(repo, "modified.go")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("package main\n\nfunc value() int { return 2 }\n"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("after\n"), 0o644))
	}
	{
		var args []string = []string{"add", "staged.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "new.py")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("def hello():\n    return 'world'\n"), 0o644))
	}

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Root, repo, "root = %q, kind = %v", got.Root,
		got.Kind,
	)
	testutil.RequireEqual(t, got.Kind, Unstaged, "root = %q, kind = %v",
		got.Root, got.Kind,
	)
	testutil.RequireEqual(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)
	testutil.RequireEqual(t, got.Files[0].DisplayPath, "modified.go",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)
	testutil.RequireEqual(t, got.Files[1].DisplayPath, "new.py",
		"unexpected files: %q, %q", got.Files[0].DisplayPath,
		got.Files[1].DisplayPath,
	)

	modified := got.Files[0]
	testutil.RequireEqual(t, func() bool {
		for _, hunk := range modified.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind == Deletion && strings.Contains(line.Text, "return 1") {
					return true
				}
			}
		}
		return false
	}(), true,
		"modified file lacks expected lines: %#v", modified.Hunks,
	)
	testutil.RequireEqual(t,
		func() bool {
			for _, hunk := range modified.Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == Addition && strings.Contains(line.Text, "return 2") {
						return true
					}
				}
			}
			return false
		}(), true, "modified file lacks expected lines: %#v", modified.Hunks)

	untracked := got.Files[1]
	testutil.RequireEqual(t, untracked.OldSource, "",
		"unexpected untracked sources: %#v", untracked,
	)
	testutil.RequireEqual(t, strings.Contains(untracked.NewSource, "def hello"),
		true, "unexpected untracked sources: %#v", untracked,
	)

	for _, hunk := range untracked.Hunks {
		for _, line := range hunk.Lines {
			testutil.RequireEqual(t, line.Kind, Addition,
				"unexpected untracked line: %#v", line,
			)
			testutil.RequireEqual(t, line.OldNumber, 0,
				"unexpected untracked line: %#v", line,
			)
			testutil.RequireEqual(t, line.NewNumber != 0, true,
				"unexpected untracked line: %#v", line,
			)
		}
	}
}

func TestAddedFileKeepsRawAndDisplayPathsSeparate(t *testing.T) {
	file := func() File {
		var path, content string = "odd\nname.go", "package main\n"
		sourceLines := func() []string {
			var content string = content
			content = strings.TrimSuffix(content, "\n")
			if content == "" {
				return nil
			}
			return strings.Split(content, "\n")
		}()
		lines := make([]Line, 0, len(sourceLines))
		for i, line := range sourceLines {
			lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)})
		}
		return File{NewPath: path, DisplayPath: strings.ReplaceAll(func() string {
			var result strings.Builder
			for _, r := range path {
				switch {
				case r == '\n' || r == '\t':
					_, _ = result.WriteRune(r)
				case r == '\r':
					_, _ = result.WriteString(`\r`)
				case r < 0x20 || r == 0x7f:
					fmt.Fprintf(&result, `\x%02x`, r)
				default:
					_, _ = result.WriteRune(r)
				}
			}
			return result.String()
		}(), "\n", `\n`), NewSource: content, Metadata: []string{"untracked file"}, Hunks: []Hunk{{Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)), Lines: lines}}}
	}()
	testutil.RequireEqual(t, file.NewPath, "odd\nname.go", "raw path = %q",
		file.NewPath,
	)
	testutil.RequireEqual(t, file.DisplayPath, `odd\nname.go`,
		"display path = %q", file.DisplayPath,
	)
}

func TestGetFromSubdirectoryKeepsRawUntrackedPaths(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	t.Helper()
	path := filepath.Join(repo, "nested/odd\nname.go")
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.NoError(t, os.WriteFile(path, []byte("package main\n"), 0o644))
	t.Chdir(filepath.Join(repo, "nested"))

	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Root, repo, "patch = %#v", got)
	testutil.RequireEqual(t, len(got.Files), 1, "patch = %#v", got)

	file := got.Files[0]
	testutil.RequireEqual(t, file.OldPath, "", "file paths = %#v", file)
	testutil.RequireEqual(t, file.NewPath, "nested/odd\nname.go",
		"file paths = %#v", file,
	)
	testutil.RequireEqual(t, file.DisplayPath, `nested/odd\nname.go`,
		"file paths = %#v", file,
	)
}

func TestGetReportsUnavailableBranchAndInvalidKind(t *testing.T) {
	t.Chdir(func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}())
	local, err := Get(context.Background(), Unstaged)
	testutil.RequireEqual(t, err, nil, "local patch = %#v, error = %v", local,
		err,
	)
	testutil.RequireEqual(t, local.Branch, "", "local patch = %#v, error = %v",
		local, err,
	)

	for _, test := range []struct {
		kind    Kind
		message string
	}{
		{Branch, "default branch is unavailable"},
		{Kind(255), "unknown patch kind"},
	} {
		got, err := Get(context.Background(), test.kind)
		testutil.RequireEqual(t, err != nil, true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
		testutil.RequireEqual(t, strings.Contains(err.Error(), test.message),
			true, "kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
		testutil.RequireEqual(t, reflect.DeepEqual(got, Patch{}), true,
			"kind %v: patch = %#v, error = %v", test.kind, got, err,
		)
	}
}

func TestGetOutsideRepositoryReturnsZeroPatch(t *testing.T) {
	t.Chdir(t.TempDir())
	got, err := Get(context.Background(), Unstaged)
	testutil.RequireEqual(t, err != nil, true, "patch = %#v, error = %v", got,
		err,
	)
	testutil.RequireEqual(t, reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestGetHonorsCanceledContext(t *testing.T) {
	t.Chdir(func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := Get(ctx, Unstaged)
	testutil.RequireEqual(t, errors.Is(err, context.Canceled), true,
		"patch = %#v, error = %v", got, err,
	)
	testutil.RequireEqual(t, reflect.DeepEqual(got, Patch{}), true,
		"patch = %#v, error = %v", got, err,
	)
}

func TestGetOmitsLargeUntrackedContents(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	var content string = strings.Repeat("x", maxFileBytes+1)
	t.Helper()
	path := filepath.Join(repo, "large.txt")
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 1, "files = %#v", got.Files)

	file := got.Files[0]
	testutil.RequireEqual(t, file.OldPath, "", "large file = %#v", file)
	testutil.RequireEqual(t, file.NewPath, "large.txt", "large file = %#v",
		file,
	)
	testutil.RequireEqual(t, file.NewSource, "", "large file = %#v", file)
	testutil.RequireEqual(t, len(file.Hunks), 0, "large file = %#v", file)
	testutil.RequireEqual(t,
		strings.Contains(
			strings.Join(file.Metadata, "\n"),
			"exceeds 2 MiB",
		), true, "large file = %#v", file)
}

func TestGetShowsBinaryMetadataWithoutBinaryDiffLines(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	t.Helper()
	path := filepath.Join(repo, "tracked.bin")
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.NoError(t, os.WriteFile(path, []byte("\x00old"), 0o644))
	var args []string = []string{"add", "tracked.bin"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	{
		var args []string = []string{"commit", "-m", "base"}
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
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("\x00new"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "untracked.bin")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("\x00content"), 0o644))
	}

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 2, "files = %d, want 2: %#v",
		len(got.Files), got.Files,
	)

	for _, file := range got.Files {
		testutil.CheckEqual(
			t,
			file.NewPath == file.DisplayPath &&
				(file.DisplayPath != "tracked.bin" ||
					file.OldPath == "tracked.bin"),
			true,
			"binary paths = %#v",
			file,
		)
		testutil.RequireEqual(t, len(file.Hunks), 0,
			"binary file %q has diff lines: %#v", file.DisplayPath, file.Hunks,
		)
		testutil.RequireEqual(
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
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	var args []string = []string{"branch", "-M", "main"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	t.Helper()
	path := filepath.Join(repo, "committed.txt")
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.NoError(t, os.WriteFile(path, []byte("base\n"), 0o644))
	{
		t.Helper()
		path := filepath.Join(repo, "mixed.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("base\n"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("base\n"), 0o644))
	}
	{
		var args []string = []string{"add", "."}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	{
		var args []string = []string{"switch", "-c", "feature"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "committed.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("committed on feature\n"), 0o644))
	}
	{
		var args []string = []string{"add", "committed.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "feature commit"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "staged.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("staged on feature\n"), 0o644))
	}
	{
		var args []string = []string{"add", "staged.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "mixed.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("unstaged on feature\n"), 0o644))
	}
	{
		t.Helper()
		path := filepath.Join(repo, "untracked.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("untracked on feature\n"), 0o644))
	}

	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, got.Root, repo, "patch metadata = %#v", got)
	testutil.RequireEqual(t, got.Kind, Branch, "patch metadata = %#v", got)
	testutil.RequireEqual(t, got.Branch, "main", "patch metadata = %#v", got)

	want := []string{
		"committed.txt",
		"mixed.txt",
		"staged.txt",
		"untracked.txt",
	}
	testutil.RequireEqual(t, len(got.Files), len(want),
		"files = %d, want %d: %#v", len(got.Files), len(want), got.Files,
	)

	for i, name := range want {
		testutil.RequireEqual(t, got.Files[i].DisplayPath, name,
			"file %d = %q, want %q", i, got.Files[i].DisplayPath, name,
		)
	}
	testutil.RequireEqual(t,
		func() bool {
			for _, hunk := range got.Files[0].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == Addition && strings.Contains(line.Text, "committed on feature") {
						return true
					}
				}
			}
			return false
		}(), true,
		"branch diff lacks expected changes: %#v", got.Files,
	)
	testutil.RequireEqual(t,
		func() bool {
			for _, hunk := range got.Files[1].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == Addition && strings.Contains(line.Text, "unstaged on feature") {
						return true
					}
				}
			}
			return false
		}(), true, "branch diff lacks expected changes: %#v", got.Files)
	testutil.RequireEqual(t,

		func() bool {
			for _, hunk := range got.Files[2].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == Addition && strings.Contains(line.Text, "staged on feature") {
						return true
					}
				}
			}
			return false
		}(), true, "branch diff lacks expected changes: %#v", got.Files)
	testutil.RequireEqual(t,

		func() bool {
			for _, hunk := range got.Files[3].Hunks {
				for _, line := range hunk.Lines {
					if line.Kind == Addition && strings.Contains(line.Text, "untracked on feature") {
						return true
					}
				}
			}
			return false
		}(), true, "branch diff lacks expected changes: %#v", got.Files)
	testutil.RequireEqual(t, got.Files[0].OldSource, "base\n",
		"unexpected branch sources: old=%q new=%q", got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
	testutil.RequireEqual(t, got.Files[0].NewSource, "committed on feature\n",
		"unexpected branch sources: old=%q new=%q", got.Files[0].OldSource,
		got.Files[0].NewSource,
	)
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
			commandRunner := &defaultBranchRunner{
				root:       root,
				originHEAD: tt.originHEAD,
				available:  tt.available,
			}
			got, err := func() (string, error) {
				var ctx context.Context = context.Background()
				out, err := runner(commandRunner).Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
				if ctx.Err() != nil {
					return "", ctx.Err()
				}
				if err == nil {
					return strings.TrimSpace(string(out)), nil
				}
				for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
					_, err := runner(commandRunner).Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
					if ctx.Err() != nil {
						return "", ctx.Err()
					}
					if err == nil {
						return candidate, nil
					}
				}
				return "", nil
			}()
			testutil.NoError(t, err)
			testutil.RequireEqual(t, got, tt.want,
				"default branch = %q, want %q", got, tt.want,
			)

			for _, call := range commandRunner.calls {
				testutil.RequireEqual(t,
					!strings.Contains(call, "for-each-ref"), true,
					"default branch discovery inspected refs: git %s", call,
				)
				testutil.RequireEqual(t, !strings.Contains(call, "merge-base"),
					true, "default branch discovery inspected refs: git %s",
					call,
				)
				testutil.RequireEqual(t, !strings.Contains(call, "rev-list"),
					true, "default branch discovery inspected refs: git %s",
					call,
				)
			}
		})
	}
}

func TestGetBranchKeepsPathsForAdditionsDeletionsAndRenames(t *testing.T) {
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	var args []string = []string{"branch", "-M", "main"}
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	t.Helper()
	path := filepath.Join(repo, "deleted.txt")
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	testutil.NoError(t, os.WriteFile(path, []byte("delete this\n"), 0o644))
	{
		t.Helper()
		path := filepath.Join(repo, "old.go")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("package main\n\nfunc example() {}\n"), 0o644))
	}
	{
		var args []string = []string{"add", "."}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "base"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"switch", "-c", "feature"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"mv", "old.go", "new.go"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"rm", "deleted.txt"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "added.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("new content\n"), 0o644))
	}
	{
		var args []string = []string{"add", "."}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		var args []string = []string{"commit", "-m", "changes"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	{
		t.Helper()
		path := filepath.Join(repo, "untracked.txt")
		testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		testutil.NoError(t, os.WriteFile(path, []byte("untracked content\n"), 0o644))
	}
	t.Chdir(repo)
	got, err := Get(context.Background(), Branch)
	testutil.NoError(t, err)

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
	testutil.RequireEqual(t, len(got.Files), len(want), "files = %#v",
		got.Files,
	)

	for i, paths := range want {
		file := got.Files[i]
		testutil.CheckEqual(
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
	repo := func() string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)
		var args []string = []string{"init", "-q"}
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		{
			var args []string = []string{"config", "user.email", "test@example.com"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		{
			var args []string = []string{"config", "user.name", "Test"}
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = repo
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
			}
		}
		return repo
	}()
	outside := filepath.Join(t.TempDir(), "secret")
	testutil.NoError(t, os.WriteFile(outside, []byte("do not read"), 0o600))
	testutil.NoError(t, os.Symlink(outside, filepath.Join(repo, "link")))

	t.Chdir(repo)
	got, err := Get(context.Background(), Unstaged)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(got.Files), 1, "files = %d, want 1",
		len(got.Files),
	)
	testutil.RequireEqual(t, got.Files[0].NewSource, outside,
		"symlink source = %q, want link target %q", got.Files[0].NewSource,
		outside,
	)
	testutil.RequireEqual(t, got.Files[0].OldPath, "", "symlink paths = %#v",
		got.Files[0],
	)
	testutil.RequireEqual(t, got.Files[0].NewPath, "link",
		"symlink paths = %#v", got.Files[0],
	)
	testutil.RequireEqual(t,
		!strings.Contains(got.Files[0].NewSource, "do not read"), true, "%s",
		"symlink target contents were read",
	)
}

func TestParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := func() ([]Line, error) {
		var (
			oldLine, newLine int32 = 10,
				20
			body []byte = []byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n")
		)
		rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
		if len(body) == 0 {
			return []Line{}, nil
		}
		lines := make([]Line, 0, len(rawLines))
		for _, raw := range rawLines {
			if len(raw) == 0 {
				return nil, errors.New("malformed empty diff line")
			}
			text := func() string {
				var value string = string(raw[1:])
				return strings.ReplaceAll(func() string {
					var result strings.Builder
					for _, r := range value {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}(), "\n", `\n`)
			}()
			switch raw[0] {
			case ' ':
				lines = append(lines, Line{Kind: Context, Text: text, OldNumber: LineNumber(oldLine), NewNumber: LineNumber(newLine)})
				oldLine++
				newLine++
			case '+':
				lines = append(lines, Line{Kind: Addition, Text: text, NewNumber: LineNumber(newLine)})
				newLine++
			case '-':
				lines = append(lines, Line{Kind: Deletion, Text: text, OldNumber: LineNumber(oldLine)})
				oldLine++
			case '\\':
			default:
				return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
			}
		}
		return lines, nil
	}()
	testutil.NoError(t, err)

	want := []Line{
		{Kind: Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: Deletion, Text: "old", OldNumber: 11},
		{Kind: Addition, Text: "new", NewNumber: 21},
		{Kind: Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	testutil.RequireEqual(t, len(lines), len(want), "lines = %#v", lines)

	for i := range want {
		testutil.RequireEqual(t, lines[i], want[i], "line %d = %#v, want %#v",
			i, lines[i], want[i],
		)
	}
}

func TestParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := func() ([]Line, error) {
		var (
			oldLine, newLine int32  = 1, 1
			body             []byte = []byte("+hello\x1b[2J\n")
		)
		rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
		if len(body) == 0 {
			return []Line{}, nil
		}
		lines := make([]Line, 0, len(rawLines))
		for _, raw := range rawLines {
			if len(raw) == 0 {
				return nil, errors.New("malformed empty diff line")
			}
			text := func() string {
				var value string = string(raw[1:])
				return strings.ReplaceAll(func() string {
					var result strings.Builder
					for _, r := range value {
						switch {
						case r == '\n' || r == '\t':
							_, _ = result.WriteRune(r)
						case r == '\r':
							_, _ = result.WriteString(`\r`)
						case r < 0x20 || r == 0x7f:
							fmt.Fprintf(&result, `\x%02x`, r)
						default:
							_, _ = result.WriteRune(r)
						}
					}
					return result.String()
				}(), "\n", `\n`)
			}()
			switch raw[0] {
			case ' ':
				lines = append(lines, Line{Kind: Context, Text: text, OldNumber: LineNumber(oldLine), NewNumber: LineNumber(newLine)})
				oldLine++
				newLine++
			case '+':
				lines = append(lines, Line{Kind: Addition, Text: text, NewNumber: LineNumber(newLine)})
				newLine++
			case '-':
				lines = append(lines, Line{Kind: Deletion, Text: text, OldNumber: LineNumber(oldLine)})
				oldLine++
			case '\\':
			default:
				return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
			}
		}
		return lines, nil
	}()
	testutil.NoError(t, err)
	testutil.RequireEqual(t, lines[0].Text, `hello\x1b[2J`, "text = %q",
		lines[0].Text,
	)
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(_ *testing.T, body []byte) {
		_, _ = func() ([]Line, error) {
			var oldLine, newLine int32 = 1, 1
			rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
			if len(body) == 0 {
				return []Line{}, nil
			}
			lines := make([]Line, 0, len(rawLines))
			for _, raw := range rawLines {
				if len(raw) == 0 {
					return nil, errors.New("malformed empty diff line")
				}
				text := func() string {
					var value string = string(raw[1:])
					return strings.ReplaceAll(func() string {
						var result strings.Builder
						for _, r := range value {
							switch {
							case r == '\n' || r == '\t':
								_, _ = result.WriteRune(r)
							case r == '\r':
								_, _ = result.WriteString(`\r`)
							case r < 0x20 || r == 0x7f:
								fmt.Fprintf(&result, `\x%02x`, r)
							default:
								_, _ = result.WriteRune(r)
							}
						}
						return result.String()
					}(), "\n", `\n`)
				}()
				switch raw[0] {
				case ' ':
					lines = append(lines, Line{Kind: Context, Text: text, OldNumber: LineNumber(oldLine), NewNumber: LineNumber(newLine)})
					oldLine++
					newLine++
				case '+':
					lines = append(lines, Line{Kind: Addition, Text: text, NewNumber: LineNumber(newLine)})
					newLine++
				case '-':
					lines = append(lines, Line{Kind: Deletion, Text: text, OldNumber: LineNumber(oldLine)})
					oldLine++
				case '\\':
				default:
					return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
				}
			}
			return lines, nil
		}()
	})
}
