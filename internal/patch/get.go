package patch

import (
	"bytes"
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

func (execRunner) Run(ctx context.Context, dir string, args ...string) (
	[]byte,
	error,
) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf(
			"git %s: %w",
			strings.Join(args, space),
			ctx.Err(),
		)
	}
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf(
			"git %s: %s",
			strings.Join(args, space),
			strings.TrimSpace(string(exitErr.Stderr)),
		)
	}
	return nil, fmt.Errorf("git %s: %w", strings.Join(args, space), err)
}

// Get retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an
// error.
func Get(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return Patch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	return loadPatch(ctx, directory, kind, execRunner{})
}

func loadPatch(ctx context.Context, directory string, kind Kind, run runner) (
	Patch,
	error,
) {
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
	revisions, readOld, err := comparisonSource(ctx, run, root, branch, kind)
	if err != nil {
		return Patch{}, err
	}

	raw, err := gitDiff(ctx, run, root, revisions...)
	if err != nil {
		return Patch{}, err
	}
	return build(ctx, run, root, kind, branch, raw, readOld)
}

func repositoryRoot(ctx context.Context, run runner, directory string) (
	string,
	error,
) {
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
) (
	[]byte,
	error,
) {
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

func build(
	ctx context.Context,
	run runner,
	root string,
	kind Kind,
	branch string,
	raw []byte,
	readOld sourceReader,
) (
	Patch,
	error,
) {
	files, err := parseTracked(ctx, run, root, raw, readOld)
	if err != nil {
		return Patch{}, err
	}
	untracked, err := loadUntracked(ctx, run, root)
	if err != nil {
		return Patch{}, err
	}
	files = append(files, untracked...)
	slices.SortStableFunc(files, func(a, b File) int {
		return strings.Compare(a.DisplayPath, b.DisplayPath)
	})

	if err := ctx.Err(); err != nil {
		return Patch{}, err
	}

	return Patch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
}

func defaultBranch(ctx context.Context, run runner, root string) (
	string,
	error,
) {
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

func parseTracked(
	ctx context.Context,
	run runner,
	root string,
	raw []byte,
	readOld sourceReader,
) (
	[]File,
	error,
) {
	if len(bytes.TrimSpace(raw)) == zero {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, zero, len(parsed))
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

func loadUntracked(ctx context.Context, run runner, root string) (
	[]File,
	error,
) {
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
	var files []File
	for rawPath := range bytes.SplitSeq(out, []byte{zero}) {
		files, err = appendUntrackedPath(ctx, root, rawPath, files)
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func untrackedFile(root, path string) (File, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return File{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != zero {
		target, err := os.Readlink(full)
		if err != nil {
			return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
		}
		return addedFile(path, visibleText(target)), true, nil
	}
	if !info.Mode().IsRegular() {
		return File{}, false, nil
	}
	file, err := untrackedRegularFile(full, path, info.Size())
	return file, err == nil, err
}

func untrackedRegularFile(full, path string, size int64) (File, error) {
	if size > maxFileBytes {
		return File{
			NewPath:     path,
			DisplayPath: visibleText(path),
			Metadata: []string{
				"untracked file",
				"content omitted: file exceeds 2 MiB",
			},
		}, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return File{}, fmt.Errorf("read untracked %q: %w", path, err)
	}
	if bytes.IndexByte(content, zero) >= zero {
		return File{
			NewPath:     path,
			DisplayPath: visibleText(path),
			Metadata:    []string{"untracked binary file"},
		}, nil
	}
	return addedFile(path, visibleSource(string(content))), nil
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, zero, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(
			lines,
			Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + one)},
		)
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

func parseHunkBody(initialOldLine, initialNewLine int32, body []byte) (
	[]Line,
	error,
) {
	oldLine, newLine := initialOldLine, initialNewLine
	rawLines := bytes.Split(body, []byte(newline))
	lines := make([]Line, zero, len(rawLines))
	for i, raw := range rawLines {
		if isTrailingEmptyLine(i, raw, len(rawLines)) {
			continue
		}
		line, include, err := parseDiffLine(raw, &oldLine, &newLine)
		if err != nil {
			return nil, err
		}
		if include {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func parseDiffLine(raw []byte, oldLine, newLine *int32) (Line, bool, error) {
	if len(raw) == zero {
		return Line{}, false, errors.New("malformed empty diff line")
	}
	line := Line{Text: visibleText(string(raw[one:]))}
	switch raw[zero] {
	case ' ':
		line.Kind, line.OldNumber, line.NewNumber = Context, LineNumber(
			*oldLine,
		), LineNumber(*newLine)
		*oldLine++
		*newLine++
	case '+':
		line.Kind, line.NewNumber = Addition, LineNumber(*newLine)
		*newLine++
	case '-':
		line.Kind, line.OldNumber = Deletion, LineNumber(*oldLine)
		*oldLine++
	case '\\':
		// The no-newline marker belongs to the preceding line.
		return Line{}, false, nil
	default:
		return Line{}, false, fmt.Errorf(
			"unexpected diff prefix %q",
			raw[zero],
		)
	}
	return line, true, nil
}

func readIndex(ctx context.Context, run runner, root, path string) string {
	if path == "" || path == nullPath {
		return ""
	}
	out, err := run.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes ||
		bytes.IndexByte(out, zero) >= zero {
		return ""
	}
	return visibleSource(string(out))
}

func readRevision(
	ctx context.Context,
	run runner,
	root, revision, path string,
) string {
	if path == "" || path == nullPath {
		return ""
	}
	out, err := run.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes ||
		bytes.IndexByte(out, zero) >= zero {
		return ""
	}
	return visibleSource(string(out))
}

func readWorkingTree(root, path string) string {
	if path == "" || path == nullPath {
		return ""
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return ""
	}
	out, err := os.ReadFile(full)
	if err != nil || bytes.IndexByte(out, zero) >= zero {
		return ""
	}
	return visibleSource(string(out))
}

func cleanDiffPath(initialPath string) string {
	path := initialPath

	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == nullPath {
		return ""
	}
	return path
}

func splitSourceLines(initialContent string) []string {
	content := initialContent

	content = strings.TrimSuffix(content, newline)
	if content == "" {
		return nil
	}
	return strings.Split(content, newline)
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
		switch {
		case r == '\n' || r == '\t':
			_, _ = result.WriteRune(r)
		case r == '\r':
			_, _ = result.WriteString(`\r`)
		case r < firstPrintableASCII || r == deleteASCII:
			fmt.Fprintf(&result, `\x%02x`, r)
		default:
			_, _ = result.WriteRune(r)
		}
	}
	return result.String()
}

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), newline, `\n`)
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
		header += space + h.Section
	}
	return header
}

func comparisonSource(
	ctx context.Context,
	run runner,
	root, branch string,
	kind Kind,
) (
	[]string,
	sourceReader,
	error,
) {
	readOld := readIndex
	var revisions []string
	if kind == Branch {
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
		revisions = []string{base}
		readOld = func(
			ctx context.Context,
			run runner,
			root,
			path string,
		) string {
			return readRevision(ctx, run, root, base, path)
		}
	}
	return revisions, readOld, nil
}

func trackedFile(
	ctx context.Context,
	run runner,
	root string,
	fd *diff.FileDiff,
	readOld sourceReader,
) (
	File,
	error,
) {
	oldPath := cleanDiffPath(fd.OrigName)
	newPath := cleanDiffPath(fd.NewName)
	display := newPath
	if display == "" || display == nullPath {
		display = oldPath
	}
	file := File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: visibleText(display),
		Metadata:    visibleStrings(fd.Extended),
	}
	file.OldSource = readOld(ctx, run, root, oldPath)
	file.NewSource = readWorkingTree(root, newPath)
	for _, h := range fd.Hunks {
		lines, parseErr := parseHunkBody(
			h.OrigStartLine,
			h.NewStartLine,
			h.Body,
		)
		if parseErr != nil {
			return File{}, fmt.Errorf("%s: %w", display, parseErr)
		}
		file.Hunks = append(file.Hunks, Hunk{
			Header: formatHunkHeader(h),
			Lines:  lines,
		})
	}
	return file, nil
}

func appendUntrackedPath(
	ctx context.Context,
	root string,
	rawPath []byte,
	files []File,
) (
	[]File,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(rawPath) == zero {
		return files, nil
	}
	file, include, err := untrackedFile(root, string(rawPath))
	if err != nil {
		return nil, err
	}
	if include {
		return append(files, file), nil
	}
	return files, nil
}

func isTrailingEmptyLine(index int, raw []byte, count int) bool {
	return index == count-one && len(raw) == zero
}
