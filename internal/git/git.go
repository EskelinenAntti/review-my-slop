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
	"slices"
	"sort"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

const maxFileBytes = 2 << 20

type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

type Git struct {
	Repository Repository
	Runner     Runner
}
type Repository struct {
	Root          string
	DefaultBranch string
}

func New(ctx context.Context) (Git, error) {
	git := ExecRunner{}
	repo, err := NewRepository(ctx, git)
	if err != nil {
		return Git{}, err
	}
	return Git{
		Runner:     git,
		Repository: repo,
	}, nil
}

func NewRepository(ctx context.Context, git Runner) (Repository, error) {
	current, err := os.Getwd()
	if err != nil {
		return Repository{}, err
	}

	root, err := root(ctx, git, current)
	if err != nil {
		return Repository{}, fmt.Errorf("resolve repository root: %w", err)
	}
	defaultBranch := defaultBranch(ctx, git, root)

	return Repository{
		Root:          root,
		DefaultBranch: defaultBranch,
	}, nil
}

func root(ctx context.Context, git Runner, cwd string) (string, error) {
	rootBytes, err := git.Run(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	return root, err
}

func defaultBranch(ctx context.Context, git Runner, root string) string {
	if out, err := git.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}
	for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
		if _, err := git.Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}"); err == nil {
			return candidate
		}
	}
	return ""
}

func (g Git) LocalChanges(ctx context.Context) (Patch, error) {
	return g.changes(ctx, "")
}

func (g Git) BranchChanges(ctx context.Context) (Patch, error) {
	baseCommit, p, err := g.baseCommit(ctx)
	if err != nil {
		return p, err
	}
	return g.changes(ctx, baseCommit)
}

func (g Git) changes(ctx context.Context, baseCommit string) (Patch, error) {
	changes, err := g.changedFiles(ctx, baseCommit)
	if err != nil {
		return Patch{}, err
	}

	untracked, err := g.untrackedFiles(ctx)
	if err != nil {
		return Patch{}, err
	}
	files := append(changes, untracked...)
	sort.SliceStable(files, func(i, j int) bool {
		return files[i].DisplayPath < files[j].DisplayPath
	})
	return Patch{Repository: g.Repository.Root, Fingerprint: fingerprint(files), Files: files}, nil
}

func (g Git) baseCommit(ctx context.Context) (string, Patch, error) {
	baseBytes, err := g.Runner.Run(ctx, g.Repository.Root, "merge-base", g.Repository.DefaultBranch, "HEAD")
	if err != nil {
		return "", Patch{}, fmt.Errorf("find branch point with %s: %w", g.Repository.DefaultBranch, err)
	}
	baseCommit := strings.TrimSpace(string(baseBytes))
	return baseCommit, Patch{}, nil
}

func fingerprint(files []File) string {
	hash := sha256.New()
	for _, file := range files {
		_, _ = hash.Write([]byte(file.NewPath))
		_, _ = hash.Write([]byte(file.NewSource))
		_, _ = hash.Write([]byte(file.OldSource))
		_, _ = hash.Write([]byte(file.OldPath))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (g Git) changedFiles(ctx context.Context, baseCommit string) ([]File, error) {
	args := []string{
		"-c", "core.quotepath=false",
		"-c", "diff.external=",
		"--no-pager",
		"diff",
		"--no-ext-diff",
		"--no-color",
		"--find-renames",
		"--src-prefix=a/",
		"--dst-prefix=b/",
		"--unified=3",
	}
	if baseCommit != "" {
		args = append(args, baseCommit)
	}
	args = append(args, "--")
	rawDiff, err := g.Runner.Run(ctx, g.Repository.Root, args...)
	if err != nil {
		return nil, err
	}

	return g.parseTracked(ctx, rawDiff)
}

func (g Git) parseTracked(ctx context.Context, rawDiff []byte) ([]File, error) {
	if len(bytes.TrimSpace(rawDiff)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, 0, len(parsed))
	for _, fd := range parsed {
		oldPath := cleanDiffPath(fd.OrigName)
		newPath := cleanDiffPath(fd.NewName)
		display := newPath
		if display == "" || display == "/dev/null" {
			display = oldPath
		}
		file := File{
			OldPath:     oldPath,
			NewPath:     newPath,
			DisplayPath: escapeSingleLineText(display),
			Metadata:    visibleStrings(fd.Extended),
		}
		file.OldSource = g.readRevision(ctx, g.Repository.DefaultBranch, oldPath)
		file.NewSource = readWorkingTree(g.Repository.Root, newPath)
		for _, h := range fd.Hunks {
			lines, parseErr := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: %w", display, parseErr)
			}
			file.Hunks = append(file.Hunks, Hunk{
				Header: formatHunkHeader(h),
				Lines:  lines,
			})
		}
		files = append(files, file)
	}
	return files, nil
}

func (g Git) untrackedFiles(ctx context.Context) ([]File, error) {
	out, err := g.Runner.Run(ctx, g.Repository.Root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []File
	for rawPath := range bytes.SplitSeq(out, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		path := string(rawPath)
		display := escapeSingleLineText(path)
		full := filepath.Join(g.Repository.Root, filepath.FromSlash(path))
		info, statErr := os.Lstat(full)
		if statErr != nil {
			return nil, fmt.Errorf("stat untracked %q: %w", path, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			symlinkDestination, readErr := os.Readlink(full)
			if readErr != nil {
				return nil, fmt.Errorf("read symlink %q: %w", path, readErr)
			}
			files = append(files, addedFile(path, escapeSingleLineText(symlinkDestination)))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxFileBytes {
			files = append(files, File{
				NewPath:     path,
				DisplayPath: display,
				Metadata:    []string{"untracked file", "content omitted: file exceeds 2 MiB"},
			})
			continue
		}
		content, readErr := os.ReadFile(full)
		if readErr != nil {
			return nil, fmt.Errorf("read untracked %q: %w", path, readErr)
		}
		if bytes.IndexByte(content, 0) >= 0 {
			files = append(files, File{
				NewPath:     path,
				DisplayPath: display,
				Metadata:    []string{"untracked binary file"},
			})
			continue
		}
		files = append(files, addedFile(display, escapeMultiLineText(string(content))))
	}
	return files, nil
}

func addedFile(path, content string) File {
	sourceLines := Lines(content)
	lines := make([]Line, 0, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)})
	}
	return File{
		NewPath:     path,
		DisplayPath: escapeSingleLineText(path),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]Line, error) {
	rawLines := bytes.Split(body, []byte("\n"))
	lines := make([]Line, 0, len(rawLines))
	for i, raw := range rawLines {
		if i == len(rawLines)-1 && len(raw) == 0 {
			continue
		}
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := escapeSingleLineText(string(raw[1:]))
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
			// "\ No newline at end of file" belongs to the preceding line.
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func (g Git) readRevision(ctx context.Context, revision, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := g.Runner.Run(ctx, g.Repository.Root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeMultiLineText(string(out))
}

func readWorkingTree(root, path string) string {
	if path == "" || path == "/dev/null" {
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
	return escapeMultiLineText(string(out))
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

func Lines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func visibleStrings(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = escapeSingleLineText(value)
	}
	return result
}

func escapeMultiLineText(value string) string {
	return escapeInvisibleChars(value, '\n', '\t')
}

func escapeSingleLineText(value string) string {
	return escapeInvisibleChars(value, '\t')
}

func escapeInvisibleChars(value string, exceptions ...rune) string {
	var result strings.Builder
	for _, r := range value {
		switch {
		case slices.Contains(exceptions, r):
			result.WriteRune(r)
		case r == '\n':
			result.WriteString(`\n`)
		case r == '\t':
			result.WriteString(`\t`)
		case r == '\r':
			result.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, r)
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

func formatHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}
