package patch

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

const maxFileBytes = 2 << 20

type runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(
	ctx context.Context,
	dir string,
	args ...string,
) ([]byte, error) {
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
		return nil, fmt.Errorf(
			"git %s: %s",
			strings.Join(args, " "),
			strings.TrimSpace(string(exitErr.Stderr)),
		)
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
}

// Get retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an
// error.
func Get(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return Patch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	return load(ctx, directory, kind, execRunner{})
}

func load(
	ctx context.Context,
	directory string,
	kind Kind,
	run runner,
) (Patch, error) {
	if kind != Unstaged && kind != Branch {
		return Patch{}, fmt.Errorf("unknown patch kind %d", kind)
	}
	if err := ctx.Err(); err != nil {
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
	revisions, readBaseSource, err := resolveComparisonInputs(ctx, run, root, kind, branch)
	if err != nil {
		return Patch{}, err
	}
	raw, err := gitDiff(ctx, run, root, revisions...)
	if err != nil {
		return Patch{}, err
	}
	return assemblePatch(ctx, run, root, kind, branch, raw, readBaseSource)
}

func resolveComparisonInputs(
	ctx context.Context,
	run runner,
	root string,
	kind Kind,
	branch string,
) (revisions []string, readBaseSource sourceReader, err error) {
	if kind != Branch {
		return nil, readIndex, nil
	}
	if branch == "" {
		return nil, nil, errors.New("default branch is unavailable")
	}
	baseBytes, err := run.Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf(
			"find branch point with %s: %w",
			branch,
			err,
		)
	}
	base := strings.TrimSpace(string(baseBytes))
	readBaseSource = func(ctx context.Context, run runner, root, path string) string {
		return readRevision(ctx, run, root, base, path)
	}
	return []string{base}, readBaseSource, nil
}

func repositoryRoot(
	ctx context.Context,
	run runner,
	directory string,
) (string, error) {
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

func gitDiff(
	ctx context.Context,
	run runner,
	root string,
	revisions ...string,
) ([]byte, error) {
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

type sourceReader func(
	ctx context.Context,
	run runner,
	repoRoot, path string,
) string

func assemblePatch(
	ctx context.Context,
	run runner,
	root string,
	kind Kind,
	branch string,
	rawDiff []byte,
	readBaseSource sourceReader,
) (Patch, error) {
	files, err := parseTrackedDiffFiles(ctx, run, root, rawDiff, readBaseSource)
	if err != nil {
		return Patch{}, err
	}
	untracked, err := loadUntracked(ctx, run, root)
	if err != nil {
		return Patch{}, err
	}
	files = append(files, untracked...)
	slices.SortStableFunc(
		files,
		func(a, b File) int {
			return cmp.Compare(a.DisplayPath, b.DisplayPath)
		},
	)

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

func defaultBranch(
	ctx context.Context,
	run runner,
	root string,
) (string, error) {
	out, err := run.Run(
		ctx,
		root,
		"symbolic-ref",
		"--quiet",
		"--short",
		"refs/remotes/origin/HEAD",
	)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	for _, candidate := range []string{
		"origin/main",
		"main",
		"origin/master",
		"master",
	} {
		_, err := run.Run(
			ctx,
			root,
			"rev-parse",
			"--verify",
			"--quiet",
			candidate+"^{commit}",
		)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

func parseTrackedDiffFiles(
	ctx context.Context,
	run runner,
	root string,
	rawDiff []byte,
	readBaseSource sourceReader,
) ([]File, error) {
	if len(bytes.TrimSpace(rawDiff)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(rawDiff)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, 0, len(parsed))
	for _, fd := range parsed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := parseTrackedDiffFile(ctx, run, root, fd, readBaseSource)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func parseTrackedDiffFile(
	ctx context.Context,
	run runner,
	root string,
	fileDiff *diff.FileDiff,
	readBaseSource sourceReader,
) (File, error) {
	oldPath := normalizeDiffPath(fileDiff.OrigName)
	newPath := normalizeDiffPath(fileDiff.NewName)
	display := newPath
	if display == "" || display == "/dev/null" {
		display = oldPath
	}
	file := File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: escapeTextForDisplay(display),
		Metadata:    escapeMetadataForDisplay(fileDiff.Extended),
	}
	file.OldSource = readBaseSource(ctx, run, root, oldPath)
	file.NewSource = readWorkingTree(root, newPath)
	for _, h := range fileDiff.Hunks {
		lines, err := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(
			file.Hunks,
			Hunk{Header: formatHunkHeader(h), Lines: lines},
		)
	}
	return file, nil
}

func loadUntracked(
	ctx context.Context,
	run runner,
	root string,
) ([]File, error) {
	out, err := run.Run(
		ctx,
		root,
		"ls-files",
		"--others",
		"--exclude-standard",
		"-z",
	)
	if err != nil {
		return nil, err
	}
	return readUntrackedPaths(ctx, root, parseGitUntrackedPaths(out))
}

func readUntrackedPaths(
	ctx context.Context,
	repoRoot string,
	relativePaths []string,
) ([]File, error) {
	var files []File
	for _, path := range relativePaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, includeInPatch, err := readUntrackedFile(repoRoot, path)
		if err != nil {
			return nil, err
		}
		if includeInPatch {
			files = append(files, file)
		}
	}
	return files, nil
}

func parseGitUntrackedPaths(output []byte) []string {
	var paths []string
	for path := range bytes.SplitSeq(output, []byte{0}) {
		if len(path) > 0 {
			paths = append(paths, string(path))
		}
	}
	return paths
}

func readUntrackedFile(
	repoRoot, relativePath string,
) (file File, includeInPatch bool, err error) {
	absolutePath := filepath.Join(repoRoot, filepath.FromSlash(relativePath))
	info, err := os.Lstat(absolutePath)
	if err != nil {
		return File{}, false, fmt.Errorf("stat untracked %q: %w", relativePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return readUntrackedSymlink(absolutePath, relativePath)
	}
	if !info.Mode().IsRegular() {
		return File{}, false, nil
	}
	if info.Size() > maxFileBytes {
		return File{
			NewPath:     relativePath,
			DisplayPath: escapeTextForDisplay(relativePath),
			Metadata: []string{
				"untracked file",
				"content omitted: file exceeds 2 MiB",
			},
		}, true, nil
	}
	content, err := os.ReadFile(absolutePath)
	if err != nil {
		return File{}, false, fmt.Errorf("read untracked %q: %w", relativePath, err)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return File{
			NewPath:     relativePath,
			DisplayPath: escapeTextForDisplay(relativePath),
			Metadata:    []string{"untracked binary file"},
		}, true, nil
	}
	return addedFile(relativePath, escapeSourceForDisplay(string(content))), true, nil
}

func readUntrackedSymlink(
	absolutePath, relativePath string,
) (file File, includeInPatch bool, err error) {
	target, err := os.Readlink(absolutePath)
	if err != nil {
		return File{}, false, fmt.Errorf("read symlink %q: %w", relativePath, err)
	}
	return addedFile(relativePath, escapeTextForDisplay(target)), true, nil
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, 0, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(
			lines,
			Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)},
		)
	}
	return File{
		NewPath:     path,
		DisplayPath: escapeTextForDisplay(path),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]Line, error) {
	rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
	if len(body) == 0 {
		return []Line{}, nil
	}
	lines := make([]Line, 0, len(rawLines))
	for _, raw := range rawLines {
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := escapeTextForDisplay(string(raw[1:]))
		switch raw[0] {
		case ' ':
			lines = append(
				lines,
				Line{
					Kind:      Context,
					Text:      text,
					OldNumber: LineNumber(oldLine),
					NewNumber: LineNumber(newLine),
				},
			)
			oldLine++
			newLine++
		case '+':
			lines = append(
				lines,
				Line{
					Kind:      Addition,
					Text:      text,
					NewNumber: LineNumber(newLine),
				},
			)
			newLine++
		case '-':
			lines = append(
				lines,
				Line{
					Kind:      Deletion,
					Text:      text,
					OldNumber: LineNumber(oldLine),
				},
			)
			oldLine++
		case '\\':
			// "\ No newline at end of file" belongs to the preceding line.
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func readIndex(ctx context.Context, run runner, root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceForDisplay(string(out))
}

func readRevision(
	ctx context.Context,
	run runner,
	root, revision, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceForDisplay(string(out))
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
	return escapeSourceForDisplay(string(out))
}

func normalizeDiffPath(path string) string {
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

func escapeMetadataForDisplay(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = escapeTextForDisplay(value)
	}
	return result
}

func escapeSourceForDisplay(value string) string {
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
}

func escapeTextForDisplay(value string) string {
	return strings.ReplaceAll(escapeSourceForDisplay(value), "\n", `\n`)
}

func formatHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf(
		"@@ -%d,%d +%d,%d @@",
		h.OrigStartLine,
		h.OrigLines,
		h.NewStartLine,
		h.NewLines,
	)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}
