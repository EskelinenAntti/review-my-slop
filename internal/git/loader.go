// Package git owns repository discovery and the conversion from Git output to
// the review domain model.
package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sourcegraph/go-diff/diff"

	domain "github.com/eskelinenantti/review-my-slop/internal/diff"
)

const maxFileBytes = 2 << 20

// Runner is the small external-effect boundary used by Loader.
type Runner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

// ExecRunner runs Git with a non-interactive environment.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := command.Output()
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// Loader reads local worktree changes and branch comparisons.
type Loader struct {
	Runner Runner
}

// NewLoader returns a loader using runner, or the real Git runner when runner
// is nil. Keeping this choice in the constructor makes the effect boundary
// explicit for callers and tests.
func NewLoader(runner Runner) Loader {
	if runner == nil {
		runner = ExecRunner{}
	}
	return Loader{Runner: runner}
}

func (l Loader) runner() Runner {
	if l.Runner != nil {
		return l.Runner
	}
	return ExecRunner{}
}

// Root resolves dir to its repository root.
func (l Loader) Root(ctx context.Context, dir string) (string, error) {
	rootBytes, err := l.runner().Run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}

// Load reads unstaged changes plus untracked files. Staged-only changes are
// intentionally omitted because the command reviews the current worktree.
func (l Loader) Load(ctx context.Context, dir string) (domain.ChangeSet, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return domain.ChangeSet{}, err
	}
	raw, err := l.diff(ctx, root)
	if err != nil {
		return domain.ChangeSet{}, err
	}
	return l.build(ctx, root, "", raw, readIndex)
}

// LoadBranch compares the working tree and committed history at branch's
// merge base. The worktree can still contain staged, unstaged, and untracked
// changes on top of that history.
func (l Loader) LoadBranch(ctx context.Context, dir, branch string) (domain.ChangeSet, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return domain.ChangeSet{}, err
	}
	baseBytes, err := l.runner().Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return domain.ChangeSet{}, fmt.Errorf("find branch point with %s: %w", branch, err)
	}
	base := strings.TrimSpace(string(baseBytes))
	raw, err := l.diff(ctx, root, base)
	if err != nil {
		return domain.ChangeSet{}, err
	}
	readBase := func(ctx context.Context, runner Runner, repository, path string) string {
		return readRevision(ctx, runner, repository, base, path)
	}
	return l.build(ctx, root, branch, raw, readBase)
}

// DefaultBranch finds the configured remote default branch without scanning
// all refs. The fallback order keeps repositories without origin/HEAD useful.
func (l Loader) DefaultBranch(ctx context.Context, dir string) (string, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return "", err
	}
	return l.defaultBranch(ctx, root), nil
}

func (l Loader) diff(ctx context.Context, root string, revisions ...string) ([]byte, error) {
	args := []string{
		"-c", "core.quotepath=false",
		"-c", "diff.external=",
		"--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames",
		"--src-prefix=a/", "--dst-prefix=b/", "--unified=3",
	}
	args = append(args, revisions...)
	args = append(args, "--")
	return l.runner().Run(ctx, root, args...)
}

type sourceReader func(context.Context, Runner, string, string) string

func (l Loader) build(ctx context.Context, root, base string, raw []byte, readOld sourceReader) (domain.ChangeSet, error) {
	files, err := parseTracked(ctx, l.runner(), root, raw, readOld)
	if err != nil {
		return domain.ChangeSet{}, err
	}
	untracked, err := l.loadUntracked(ctx, root)
	if err != nil {
		return domain.ChangeSet{}, err
	}
	files = append(files, untracked...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path() < files[j].Path() })

	return domain.ChangeSet{
		Repository:  root,
		Fingerprint: fingerprintForFiles(base, raw, untracked),
		Files:       files,
	}, nil
}

func fingerprintForFiles(base string, raw []byte, untracked []domain.File) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(base))
	_, _ = hash.Write(raw)
	for _, file := range untracked {
		_, _ = hash.Write([]byte(file.NewPath))
		_, _ = hash.Write([]byte(file.NewSource))
		for _, metadata := range file.Metadata {
			_, _ = hash.Write([]byte(metadata))
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (l Loader) defaultBranch(ctx context.Context, root string) string {
	if out, err := l.runner().Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}
	for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
		if _, err := l.runner().Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}"); err == nil {
			return candidate
		}
	}
	return ""
}

func parseTracked(ctx context.Context, runner Runner, root string, raw []byte, readOld sourceReader) ([]domain.File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]domain.File, 0, len(parsed))
	for _, fileDiff := range parsed {
		oldPath := cleanDiffPath(fileDiff.OrigName)
		newPath := cleanDiffPath(fileDiff.NewName)
		display := newPath
		if display == "" {
			display = oldPath
		}
		file := domain.File{
			OldPath:  oldPath,
			NewPath:  newPath,
			Metadata: append([]string(nil), fileDiff.Extended...),
		}
		file.OldSource = readOld(ctx, runner, root, oldPath)
		file.NewSource = readWorkingTree(root, newPath)
		for _, hunk := range fileDiff.Hunks {
			lines, parseErr := parseHunkBody(hunk.OrigStartLine, hunk.NewStartLine, hunk.Body)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: %w", display, parseErr)
			}
			file.Hunks = append(file.Hunks, domain.Hunk{
				Header: formatHunkHeader(hunk),
				Lines:  lines,
			})
		}
		files = append(files, file)
	}
	return files, nil
}

func (l Loader) loadUntracked(ctx context.Context, root string) ([]domain.File, error) {
	out, err := l.runner().Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []domain.File
	for rawPath := range bytes.SplitSeq(out, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		path := string(rawPath)
		full := filepath.Join(root, filepath.FromSlash(path))
		info, statErr := os.Lstat(full)
		if statErr != nil {
			return nil, fmt.Errorf("stat untracked %q: %w", path, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(full)
			if readErr != nil {
				return nil, fmt.Errorf("read symlink %q: %w", path, readErr)
			}
			files = append(files, addedFile(path, target, []string{"untracked symlink"}))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxFileBytes {
			files = append(files, domain.File{
				NewPath:  path,
				Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"},
			})
			continue
		}
		content, readErr := os.ReadFile(full)
		if readErr != nil {
			return nil, fmt.Errorf("read untracked %q: %w", path, readErr)
		}
		if bytes.IndexByte(content, 0) >= 0 {
			files = append(files, domain.File{
				NewPath:  path,
				Metadata: []string{"untracked binary file"},
			})
			continue
		}
		files = append(files, addedFile(path, string(content), []string{"untracked file"}))
	}
	return files, nil
}

func addedFile(path, content string, metadata []string) domain.File {
	sourceLines := splitSourceLines(content)
	lines := make([]domain.Line, 0, len(sourceLines))
	for index, line := range sourceLines {
		lines = append(lines, domain.Line{Kind: domain.Addition, Text: line, NewNumber: domain.LineNumber(index + 1)})
	}
	return domain.File{
		NewPath:   path,
		NewSource: content,
		Metadata:  append([]string(nil), metadata...),
		Hunks: []domain.Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]domain.Line, error) {
	rawLines := bytes.Split(body, []byte("\n"))
	lines := make([]domain.Line, 0, len(rawLines))
	for index, raw := range rawLines {
		if index == len(rawLines)-1 && len(raw) == 0 {
			continue
		}
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		switch raw[0] {
		case ' ':
			lines = append(lines, domain.Line{Kind: domain.Context, Text: string(raw[1:]), OldNumber: domain.LineNumber(oldLine), NewNumber: domain.LineNumber(newLine)})
			oldLine++
			newLine++
		case '+':
			lines = append(lines, domain.Line{Kind: domain.Addition, Text: string(raw[1:]), NewNumber: domain.LineNumber(newLine)})
			newLine++
		case '-':
			lines = append(lines, domain.Line{Kind: domain.Deletion, Text: string(raw[1:]), OldNumber: domain.LineNumber(oldLine)})
			oldLine++
		case '\\':
			// "\\ No newline at end of file" belongs to the preceding line.
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func readIndex(ctx context.Context, runner Runner, root, path string) string {
	if path == "" {
		return ""
	}
	out, err := runner.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return string(out)
}

func readRevision(ctx context.Context, runner Runner, root, revision, path string) string {
	if path == "" {
		return ""
	}
	out, err := runner.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return string(out)
}

func readWorkingTree(root, path string) string {
	if path == "" {
		return ""
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return ""
	}
	out, err := os.ReadFile(full)
	if err != nil || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return string(out)
}

func cleanDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func splitSourceLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func formatHunkHeader(hunk *diff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.OrigStartLine, hunk.OrigLines, hunk.NewStartLine, hunk.NewLines)
	if hunk.Section != "" {
		header += " " + hunk.Section
	}
	return header
}
