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
	root, err := validatedRoot(ctx, run, directory, kind)
	if err != nil {
		return Patch{}, err
	}
	branch, err := defaultBranch(ctx, run, root)
	if err != nil {
		return Patch{}, fmt.Errorf("discover default branch: %w", err)
	}
	revisions, readOld, err := comparisonSource(ctx, run, root, kind, branch)
	if err != nil {
		return Patch{}, err
	}
	raw, err := gitDiff(ctx, run, root, revisions...)
	if err != nil {
		return Patch{}, err
	}
	return build(ctx, run, root, kind, branch, raw, readOld)
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
		branch, err := verifiedBranch(ctx, run, root, candidate)
		if err != nil || branch != "" {
			return branch, err
		}
	}
	return "", nil
}

func verifiedBranch(ctx context.Context, run runner, root, candidate string) (string, error) {
	_, err := run.Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err == nil {
		return candidate, nil
	}
	return "", nil
}

func parseTracked(ctx context.Context, run runner, root string, raw []byte, readOld sourceReader) ([]File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	return trackedFiles(ctx, run, root, parsed, readOld)
}

func trackedFiles(ctx context.Context, run runner, root string, parsed []*diff.FileDiff, readOld sourceReader) ([]File, error) {
	files := make([]File, 0, len(parsed))
	for _, fd := range parsed {
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
	return untrackedFiles(ctx, root, out)
}

func untrackedFiles(ctx context.Context, root string, paths []byte) ([]File, error) {
	var files []File
	for rawPath := range bytes.SplitSeq(paths, []byte{0}) {
		file, err := untrackedFileContext(ctx, root, string(rawPath))
		if err != nil {
			return nil, err
		}
		files = appendUntracked(files, file)
	}
	return files, nil
}

func appendUntracked(files []File, file *File) []File {
	if file != nil {
		return append(files, *file)
	}
	return files
}

func untrackedFile(root, path string) (*File, error) {
	if path == "" {
		return nil, nil
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return nil, fmt.Errorf("stat untracked %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return untrackedSymlink(full, path)
	}
	return untrackedRegular(full, path, info)
}

func untrackedSymlink(full, path string) (*File, error) {
	target, err := os.Readlink(full)
	if err != nil {
		return nil, fmt.Errorf("read symlink %q: %w", path, err)
	}
	file := addedFile(path, visibleText(target))
	return &file, nil
}

func untrackedRegular(full, path string, info os.FileInfo) (*File, error) {
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	if info.Size() > maxFileBytes {
		return &File{NewPath: path, DisplayPath: visibleText(path), Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"}}, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read untracked %q: %w", path, err)
	}
	return untrackedContent(path, content), nil
}

func untrackedContent(path string, content []byte) *File {
	if bytes.IndexByte(content, 0) >= 0 {
		return &File{NewPath: path, DisplayPath: visibleText(path), Metadata: []string{"untracked binary file"}}
	}
	file := addedFile(path, visibleSource(string(content)))
	return &file
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
	// A final newline terminates the last diff line; it is not an empty line.
	rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
	if len(body) == 0 {
		return []Line{}, nil
	}
	return parseHunkLines(oldLine, newLine, rawLines)
}

func parseHunkLines(oldLine, newLine int32, rawLines [][]byte) ([]Line, error) {
	lines := make([]Line, 0, len(rawLines))
	for _, raw := range rawLines {
		line, err := parseHunkLine(raw, &oldLine, &newLine)
		if err != nil {
			return nil, err
		}
		lines = appendHunkLine(lines, line)
	}
	return lines, nil
}

func parseHunkLine(raw []byte, oldLine, newLine *int32) (*Line, error) {
	if len(raw) == 0 {
		return nil, errors.New("malformed empty diff line")
	}
	line := &Line{Text: visibleText(string(raw[1:]))}
	switch raw[0] {
	case ' ':
		line.Kind, line.OldNumber, line.NewNumber = Context, LineNumber(*oldLine), LineNumber(*newLine)
		*oldLine++
		*newLine++
	case '+':
		line.Kind, line.NewNumber = Addition, LineNumber(*newLine)
		*newLine++
	case '-':
		line.Kind, line.OldNumber = Deletion, LineNumber(*oldLine)
		*oldLine++
	case '\\':
		return nil, nil // "\\ No newline at end of file" belongs to the preceding line.
	default:
		return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
	}
	return line, nil
}

func readIndex(ctx context.Context, run runner, root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return visibleSource(string(out))
}

func readRevision(ctx context.Context, run runner, root, revision, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return visibleSource(string(out))
}

func readWorkingTree(root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	return readRegularSource(filepath.Join(root, filepath.FromSlash(path)))
}

func readRegularSource(full string) string {
	info, err := os.Lstat(full)
	if !readableFile(info, err) {
		return ""
	}
	out, err := os.ReadFile(full)
	if err != nil || bytes.IndexByte(out, 0) >= 0 {
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

func validatedRoot(ctx context.Context, run runner, directory string, kind Kind) (string, error) {
	if kind != Unstaged && kind != Branch {
		return "", fmt.Errorf("unknown patch kind %d", kind)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return repositoryRoot(ctx, run, directory)
}

func comparisonSource(ctx context.Context, run runner, root string, kind Kind, branch string) ([]string, sourceReader, error) {
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

func trackedFile(ctx context.Context, run runner, root string, fd *diff.FileDiff, readOld sourceReader) (File, error) {
	if err := ctx.Err(); err != nil {
		return File{}, err
	}
	oldPath, newPath := cleanDiffPath(fd.OrigName), cleanDiffPath(fd.NewName)
	display := newPath
	if display == "" || display == "/dev/null" {
		display = oldPath
	}
	file := File{OldPath: oldPath, NewPath: newPath, DisplayPath: visibleText(display), Metadata: visibleStrings(fd.Extended)}
	file.OldSource = readOld(ctx, run, root, oldPath)
	file.NewSource = readWorkingTree(root, newPath)
	hunks, err := trackedHunks(fd.Hunks, display)
	file.Hunks = hunks
	return file, err
}

func trackedHunks(hunks []*diff.Hunk, display string) ([]Hunk, error) {
	var result []Hunk
	for _, h := range hunks {
		lines, err := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", display, err)
		}
		result = append(result, Hunk{Header: formatHunkHeader(h), Lines: lines})
	}
	return result, nil
}

func readableFile(info os.FileInfo, err error) bool {
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Size() <= maxFileBytes
}

func untrackedFileContext(ctx context.Context, root, path string) (*File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return untrackedFile(root, path)
}

func appendHunkLine(lines []Line, line *Line) []Line {
	if line != nil {
		return append(lines, *line)
	}
	return lines
}

func writeVisibleRune(result *strings.Builder, r rune) {
	switch {
	case r == '\n' || r == '\t':
		result.WriteRune(r)
	case r == '\r':
		result.WriteString(`\r`)
	case r < 0x20 || r == 0x7f:
		fmt.Fprintf(result, `\x%02x`, r)
	default:
		result.WriteRune(r)
	}
}
