package patch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

const maxFileBytes = 2 << 20

type runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), ctx.Err())
	}
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// Get retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an error.
func Get(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return Patch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	return get(ctx, directory, kind, execRunner{})
}

func get(ctx context.Context, directory string, kind Kind, run runner) (Patch, error) {
	if err := validateRequest(ctx, kind); err != nil {
		return Patch{}, err
	}
	root, err := repositoryRoot(ctx, run, directory)
	if err != nil {
		return Patch{}, err
	}
	branch, err := defaultBranch(ctx, run, root)
	if err != nil {
		return Patch{}, fmt.Errorf("discover default branch: %w", err)
	}
	return loadDiff(ctx, run, root, kind, branch)
}

func loadDiff(ctx context.Context, run runner, root string, kind Kind, branch string) (Patch, error) {
	revisions, readOld, err := diffSource(ctx, run, root, kind, branch)
	if err != nil {
		return Patch{}, err
	}
	raw, err := gitDiff(ctx, run, root, revisions...)
	if err != nil {
		return Patch{}, err
	}
	return build(ctx, run, root, kind, branch, raw, readOld)
}

func diffSource(ctx context.Context, run runner, root string, kind Kind, branch string) ([]string, sourceReader, error) {
	if kind != Branch {
		return nil, readIndex, nil
	}
	if branch == "" {
		return nil, nil, errors.New("default branch is unavailable")
	}
	baseBytes, err := run.Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf("find branch point with %s: %w", branch, err)
	}
	base := strings.TrimSpace(string(baseBytes))
	return []string{base}, func(ctx context.Context, run runner, root, path string) string {
		return readRevision(ctx, run, root, base, path)
	}, nil
}

func repositoryRoot(ctx context.Context, run runner, directory string) (string, error) {
	rootBytes, err := run.Run(ctx, directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}

func gitDiff(ctx context.Context, run runner, root string, revisions ...string) ([]byte, error) {
	args := []string{
		"-c", "core.quotepath=false",
		"-c", "diff.external=",
		"--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames",
		"--src-prefix=a/", "--dst-prefix=b/", "--unified=3",
	}
	args = append(args, revisions...)
	args = append(args, "--")
	return run.Run(ctx, root, args...)
}

type sourceReader func(context.Context, runner, string, string) string

func build(ctx context.Context, run runner, root string, kind Kind, branch string, raw []byte, readOld sourceReader) (Patch, error) {
	files, err := parseTracked(ctx, run, root, raw, readOld)
	if err != nil {
		return Patch{}, err
	}
	untracked, err := loadUntracked(ctx, run, root)
	if err != nil {
		return Patch{}, err
	}
	files = append(files, untracked...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].DisplayPath < files[j].DisplayPath })

	if err := ctx.Err(); err != nil {
		return Patch{}, err
	}

	return Patch{
		Root:   root,
		Kind:   kind,
		Branch: branch,
		Files:  files,
	}, nil
}

func defaultBranch(ctx context.Context, run runner, root string) (string, error) {
	out, err := run.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	return fallbackBranch(ctx, run, root)
}

func fallbackBranch(ctx context.Context, run runner, root string) (string, error) {
	for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
		_, err := run.Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

func parseTracked(ctx context.Context, run runner, root string, raw []byte, readOld sourceReader) ([]File, error) {
	parsed, err := parseFileDiffs(raw)
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		return nil, nil
	}
	return trackedFiles(ctx, run, root, parsed, readOld)
}

func trackedFiles(ctx context.Context, run runner, root string, parsed []*diff.FileDiff, readOld sourceReader) ([]File, error) {
	files := make([]File, 0, len(parsed))
	for _, fd := range parsed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := trackedFile(ctx, run, root, fd, readOld)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func loadUntracked(ctx context.Context, run runner, root string) ([]File, error) {
	out, err := run.Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	return untrackedPaths(ctx, root, out)
}

func untrackedPaths(ctx context.Context, root string, out []byte) ([]File, error) {
	var files []File
	for rawPath := range bytes.SplitSeq(out, []byte{0}) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := appendUntracked(&files, root, string(rawPath)); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, 0, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)})
	}
	return File{
		NewPath:     path,
		DisplayPath: visibleText(path),
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
		if trailingEmptyLine(i, raw, len(rawLines)) {
			continue
		}
		if err := appendDiffLine(&lines, raw, &oldLine, &newLine); err != nil {
			return nil, err
		}
	}
	return lines, nil
}

func trailingEmptyLine(index int, raw []byte, count int) bool {
	return index == count-1 && len(raw) == 0
}

func parseDiffLine(raw []byte, oldLine, newLine *int32) (*Line, error) {
	if len(raw) == 0 {
		return nil, errors.New("malformed empty diff line")
	}
	// No-newline marker belongs to the preceding line.
	if raw[0] == '\\' {
		return nil, nil
	}
	return numberedDiffLine(raw[0], visibleText(string(raw[1:])), oldLine, newLine)
}

func numberedDiffLine(prefix byte, text string, oldLine, newLine *int32) (*Line, error) {
	switch prefix {
	case ' ':
		line := Line{Kind: Context, Text: text, OldNumber: LineNumber(*oldLine), NewNumber: LineNumber(*newLine)}
		*oldLine++
		*newLine++
		return &line, nil
	case '+':
		line := Line{Kind: Addition, Text: text, NewNumber: LineNumber(*newLine)}
		*newLine++
		return &line, nil
	case '-':
		line := Line{Kind: Deletion, Text: text, OldNumber: LineNumber(*oldLine)}
		*oldLine++
		return &line, nil
	}
	return nil, fmt.Errorf("unexpected diff prefix %q", prefix)
}

func readIndex(ctx context.Context, run runner, root, path string) string {
	if missingPath(path) {
		return ""
	}
	out, err := run.Run(ctx, root, "show", ":"+path)
	if !readableContent(out, err) {
		return ""
	}
	return visibleSource(string(out))
}

func readRevision(ctx context.Context, run runner, root, revision, path string) string {
	if missingPath(path) {
		return ""
	}
	out, err := run.Run(ctx, root, "show", revision+":"+path)
	if !readableContent(out, err) {
		return ""
	}
	return visibleSource(string(out))
}

func readWorkingTree(root, path string) string {
	if missingPath(path) {
		return ""
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if !readableFile(info, err) {
		return ""
	}
	out, err := os.ReadFile(full)
	if !textContent(out, err) {
		return ""
	}
	return visibleSource(string(out))
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

func visibleStrings(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = visibleText(value)
	}
	return result
}

func visibleSource(value string) string {
	var result strings.Builder
	for _, r := range value {
		writeVisibleRune(&result, r)
	}
	return result.String()
}

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), "\n", `\n`)
}

func formatHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}

func validKind(kind Kind) bool { return kind == Unstaged || kind == Branch }

func trackedFile(ctx context.Context, run runner, root string, fd *diff.FileDiff, readOld sourceReader) (File, error) {
	oldPath, newPath := cleanDiffPath(fd.OrigName), cleanDiffPath(fd.NewName)
	display := newPath
	if missingPath(display) {
		display = oldPath
	}
	file := File{OldPath: oldPath, NewPath: newPath, DisplayPath: visibleText(display), Metadata: visibleStrings(fd.Extended)}
	file.OldSource = readOld(ctx, run, root, oldPath)
	file.NewSource = readWorkingTree(root, newPath)
	for _, h := range fd.Hunks {
		lines, err := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(file.Hunks, Hunk{Header: formatHunkHeader(h), Lines: lines})
	}
	return file, nil
}

func untrackedFile(root, path string) (File, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return File{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return untrackedSymlink(full, path)
	}
	if !info.Mode().IsRegular() {
		return File{}, false, nil
	}
	return untrackedRegular(full, path, info.Size())
}

func untrackedSymlink(full, path string) (File, bool, error) {
	target, err := os.Readlink(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
	}
	return addedFile(path, visibleText(target)), true, nil
}

func untrackedRegular(full, path string, size int64) (File, bool, error) {
	if size > maxFileBytes {
		return omittedFile(path, "untracked file", "content omitted: file exceeds 2 MiB"), true, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read untracked %q: %w", path, err)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return omittedFile(path, "untracked binary file"), true, nil
	}
	return addedFile(path, visibleSource(string(content))), true, nil
}

func omittedFile(path string, metadata ...string) File {
	return File{NewPath: path, DisplayPath: visibleText(path), Metadata: metadata}
}

func missingPath(path string) bool { return path == "" || path == "/dev/null" }
func readableContent(out []byte, err error) bool {
	return err == nil && len(out) <= maxFileBytes && bytes.IndexByte(out, 0) < 0
}

func readableFile(info os.FileInfo, err error) bool {
	return err == nil && info.Mode().IsRegular() && info.Size() <= maxFileBytes
}

func writeVisibleRune(result *strings.Builder, r rune) {
	switch r {
	case '\n', '\t':
		result.WriteRune(r)
	case '\r':
		result.WriteString(`\r`)
	default:
		writeVisibleCharacter(result, r)
	}
}
func writeVisibleCharacter(result *strings.Builder, r rune) {
	if r < 0x20 || r == 0x7f {
		fmt.Fprintf(result, `\x%02x`, r)
		return
	}
	result.WriteRune(r)
}

func validateRequest(ctx context.Context, kind Kind) error {
	if !validKind(kind) {
		return fmt.Errorf("unknown patch kind %d", kind)
	}
	return ctx.Err()
}

func parseFileDiffs(raw []byte) ([]*diff.FileDiff, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	return parsed, nil
}

func appendUntracked(files *[]File, root, path string) error {
	if path == "" {
		return nil
	}
	file, include, err := untrackedFile(root, path)
	if err != nil {
		return err
	}
	if include {
		*files = append(*files, file)
	}
	return nil
}

func appendDiffLine(lines *[]Line, raw []byte, oldLine, newLine *int32) error {
	line, err := parseDiffLine(raw, oldLine, newLine)
	if err != nil {
		return err
	}
	if line != nil {
		*lines = append(*lines, *line)
	}
	return nil
}

func textContent(out []byte, err error) bool { return err == nil && bytes.IndexByte(out, 0) < 0 }
