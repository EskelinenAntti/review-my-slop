package gitdiff

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

	"github.com/eskelinenantti/review-my-slop/internal/patch"
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

type Loader struct {
	Runner Runner
}

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

func (l Loader) Load(ctx context.Context, dir string) (patch.Patch, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return patch.Patch{}, err
	}

	raw, err := l.diff(ctx, root)
	if err != nil {
		return patch.Patch{}, err
	}
	return l.buildPatch(ctx, root, "", raw)
}

func (l Loader) LoadBranch(ctx context.Context, dir, branch string) (patch.Patch, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return patch.Patch{}, err
	}
	baseBytes, err := l.runner().Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return patch.Patch{}, fmt.Errorf("find branch point with %s: %w", branch, err)
	}
	base := strings.TrimSpace(string(baseBytes))
	raw, err := l.diff(ctx, root, base)
	if err != nil {
		return patch.Patch{}, err
	}
	return l.buildPatch(ctx, root, branch, raw)
}

func (l Loader) DefaultBranch(ctx context.Context, dir string) (string, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return "", err
	}
	return l.defaultBranch(ctx, root), nil
}

func (l Loader) runner() Runner {
	if l.Runner != nil {
		return l.Runner
	}
	return ExecRunner{}
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

func (l Loader) buildPatch(ctx context.Context, root, base string, raw []byte) (patch.Patch, error) {
	runner := l.runner()
	tracked, err := l.loadTracked(ctx, runner, root, base, raw)
	if err != nil {
		return patch.Patch{}, err
	}
	untracked, err := l.loadUntracked(ctx, root, runner)
	if err != nil {
		return patch.Patch{}, err
	}
	files := append(tracked, untracked...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].DisplayPath < files[j].DisplayPath })

	return patch.Patch{
		Repository:  root,
		Fingerprint: fingerprint(base, raw, untracked),
		Files:       files,
	}, nil
}

func fingerprint(base string, raw []byte, untracked []patch.File) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte(base))
	_, _ = hash.Write(raw)
	for _, file := range untracked {
		_, _ = hash.Write([]byte(file.NewPath))
		_, _ = hash.Write([]byte(file.NewSource))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (l Loader) defaultBranch(ctx context.Context, root string) string {
	runner := l.runner()
	if out, err := runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}
	for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
		if _, err := runner.Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}"); err == nil {
			return candidate
		}
	}
	return ""
}

func (l Loader) loadTracked(ctx context.Context, runner Runner, root, base string, raw []byte) ([]patch.File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]patch.File, 0, len(parsed))
	for _, fd := range parsed {
		file, err := parseTrackedFile(ctx, runner, root, base, fd)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func parseTrackedFile(ctx context.Context, runner Runner, root, base string, fd *diff.FileDiff) (patch.File, error) {
	oldPath := cleanDiffPath(fd.OrigName)
	newPath := cleanDiffPath(fd.NewName)
	display := newPath
	if display == "" || display == "/dev/null" {
		display = oldPath
	}
	file := patch.File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: sanitizeText(display),
		Metadata:    sanitizeTexts(fd.Extended),
		OldSource:   readGitSource(ctx, runner, root, base, oldPath),
		NewSource:   readWorkingTree(root, newPath),
	}
	for _, h := range fd.Hunks {
		lines, err := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
		if err != nil {
			return patch.File{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(file.Hunks, patch.Hunk{Header: formatHunkHeader(h), Lines: lines})
	}
	return file, nil
}

func (l Loader) loadUntracked(ctx context.Context, root string, runner Runner) ([]patch.File, error) {
	out, err := runner.Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []patch.File
	for rawPath := range bytes.SplitSeq(out, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		file, include, readErr := loadUntrackedFile(root, string(rawPath))
		if readErr != nil {
			return nil, readErr
		}
		if include {
			files = append(files, file)
		}
	}
	return files, nil
}

func loadUntrackedFile(root, path string) (patch.File, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return patch.File{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return loadUntrackedSymlink(path, full)
	}
	if !info.Mode().IsRegular() {
		return patch.File{}, false, nil
	}
	return loadUntrackedRegular(path, full, info.Size())
}

func loadUntrackedSymlink(path, full string) (patch.File, bool, error) {
	target, err := os.Readlink(full)
	if err != nil {
		return patch.File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
	}
	return newUntrackedFile(path, sanitizeText(target)), true, nil
}

func loadUntrackedRegular(path, full string, size int64) (patch.File, bool, error) {
	display := sanitizeText(path)
	if size > maxFileBytes {
		return untrackedMetadataFile(path, display, "untracked file", "content omitted: file exceeds 2 MiB"), true, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return patch.File{}, false, fmt.Errorf("read untracked %q: %w", path, err)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return untrackedMetadataFile(path, display, "untracked binary file"), true, nil
	}
	return newUntrackedFile(path, sanitizeSource(string(content))), true, nil
}

func untrackedMetadataFile(path, display string, metadata ...string) patch.File {
	return patch.File{NewPath: path, DisplayPath: display, Metadata: metadata}
}

func newUntrackedFile(path, content string) patch.File {
	sourceLines := splitSourceLines(content)
	lines := make([]patch.Line, 0, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(lines, patch.Line{Kind: patch.Addition, Text: line, NewNumber: patch.LineNumber(i + 1)})
	}
	return patch.File{
		NewPath:     path,
		DisplayPath: sanitizeText(path),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []patch.Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]patch.Line, error) {
	rawLines := bytes.Split(body, []byte("\n"))
	lines := make([]patch.Line, 0, len(rawLines))
	for i, raw := range rawLines {
		if i == len(rawLines)-1 && len(raw) == 0 {
			continue
		}
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		prefix := raw[0]
		if prefix == '\\' {
			if string(raw) != `\ No newline at end of file` {
				return nil, fmt.Errorf("unexpected diff marker %q", raw)
			}
			// This marker belongs to the preceding line.
			continue
		}
		text := sanitizeText(string(raw[1:]))
		switch prefix {
		case ' ':
			lines = append(lines, patch.Line{Kind: patch.Context, Text: text, OldNumber: patch.LineNumber(oldLine), NewNumber: patch.LineNumber(newLine)})
			oldLine++
			newLine++
		case '+':
			lines = append(lines, patch.Line{Kind: patch.Addition, Text: text, NewNumber: patch.LineNumber(newLine)})
			newLine++
		case '-':
			lines = append(lines, patch.Line{Kind: patch.Deletion, Text: text, OldNumber: patch.LineNumber(oldLine)})
			oldLine++
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", prefix)
		}
	}
	return lines, nil
}

func readGitSource(ctx context.Context, runner Runner, root, revision, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	spec := ":" + path
	if revision != "" {
		spec = revision + ":" + path
	}
	out, err := runner.Run(ctx, root, "show", spec)
	return sanitizeSourceBytes(out, err)
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
	return sanitizeSourceBytes(out, err)
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

func sanitizeTexts(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = sanitizeText(value)
	}
	return result
}

func sanitizeSourceBytes(value []byte, err error) string {
	if err != nil || len(value) > maxFileBytes || bytes.IndexByte(value, 0) >= 0 {
		return ""
	}
	return sanitizeSource(string(value))
}

func sanitizeSource(value string) string {
	var result strings.Builder
	for _, r := range value {
		switch {
		case r == '\n' || r == '\t':
			result.WriteRune(r)
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

func sanitizeText(value string) string {
	return strings.ReplaceAll(sanitizeSource(value), "\n", `\n`)
}

func formatHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}
