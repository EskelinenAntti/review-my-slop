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

// GetPatch retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an
// error.
func GetPatch(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return Patch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	return loadPatch(ctx, directory, kind, execRunner{})
}

func loadPatch(
	ctx context.Context,
	directory string,
	kind Kind,
	runCommand runner,
) (Patch, error) {
	if kind != Unstaged && kind != Branch {
		return Patch{}, fmt.Errorf("unknown patch kind %d", kind)
	}
	if err := ctx.Err(); err != nil {
		return Patch{}, err
	}
	root, err := findRepositoryRoot(ctx, runCommand, directory)
	if err != nil {
		return Patch{}, err
	}
	branch, err := findDefaultBranch(ctx, runCommand, root)
	if err != nil {
		return Patch{}, fmt.Errorf("discover default branch: %w", err)
	}
	revisions, readOld, err := findComparisonBase(
		ctx,
		runCommand,
		root,
		kind,
		branch,
	)
	if err != nil {
		return Patch{}, err
	}
	raw, err := readChangeText(ctx, runCommand, root, revisions...)
	if err != nil {
		return Patch{}, err
	}
	return assemblePatch(ctx, runCommand, root, kind, branch, raw, readOld)
}

func findComparisonBase(
	ctx context.Context,
	runCommand runner,
	root string,
	kind Kind,
	branch string,
) ([]string, sourceReader, error) {
	if kind != Branch {
		return nil, readStagedFile, nil
	}
	if branch == "" {
		return nil, nil, errors.New("default branch is unavailable")
	}
	baseBytes, err := runCommand.Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf(
			"find branch point with %s: %w",
			branch,
			err,
		)
	}
	base := strings.TrimSpace(string(baseBytes))
	readOld := func(
		ctx context.Context,
		runCommand runner,
		root, path string,
	) string {
		return readFileAtRevision(ctx, runCommand, root, base, path)
	}
	return []string{base}, readOld, nil
}

func findRepositoryRoot(
	ctx context.Context,
	runCommand runner,
	directory string,
) (string, error) {
	rootBytes, err := runCommand.Run(
		ctx,
		directory,
		"rev-parse",
		"--show-toplevel",
	)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}

func readChangeText(
	ctx context.Context,
	runCommand runner,
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
	return runCommand.Run(ctx, root, args...)
}

type sourceReader func(context.Context, runner, string, string) string

func assemblePatch(
	ctx context.Context,
	runCommand runner,
	root string,
	kind Kind,
	branch string,
	raw []byte,
	readOld sourceReader,
) (Patch, error) {
	files, err := parseChangedFiles(ctx, runCommand, root, raw, readOld)
	if err != nil {
		return Patch{}, err
	}
	untracked, err := loadNewFiles(ctx, runCommand, root)
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

func findDefaultBranch(
	ctx context.Context,
	runCommand runner,
	root string,
) (string, error) {
	out, err := runCommand.Run(
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
		_, err := runCommand.Run(
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

func parseChangedFiles(
	ctx context.Context,
	runCommand runner,
	root string,
	raw []byte,
	readOld sourceReader,
) ([]File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, 0, len(parsed))
	for _, fd := range parsed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := parseChangedFile(ctx, runCommand, root, fd, readOld)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func parseChangedFile(
	ctx context.Context,
	runCommand runner,
	root string,
	fd *diff.FileDiff,
	readOld sourceReader,
) (File, error) {
	oldPath := normalizeChangedFilePath(fd.OrigName)
	newPath := normalizeChangedFilePath(fd.NewName)
	display := newPath
	if display == "" || display == "/dev/null" {
		display = oldPath
	}
	file := File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: makeTextSafeToShow(display),
		Metadata:    makeStringsSafeToShow(fd.Extended),
	}
	file.OldSource = readOld(ctx, runCommand, root, oldPath)
	file.NewSource = readWorkingTree(root, newPath)
	for _, h := range fd.Hunks {
		lines, err := parseChangedLines(h.OrigStartLine, h.NewStartLine, h.Body)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(
			file.Hunks,
			Hunk{Header: formatChangedLinesHeader(h), Lines: lines},
		)
	}
	return file, nil
}

func loadNewFiles(
	ctx context.Context,
	runCommand runner,
	root string,
) ([]File, error) {
	out, err := runCommand.Run(
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
	return readNewFilePaths(ctx, root, newFilePaths(out))
}

func readNewFilePaths(
	ctx context.Context,
	root string,
	paths []string,
) ([]File, error) {
	var files []File
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, include, err := readNewFile(root, path)
		if err != nil {
			return nil, err
		}
		if include {
			files = append(files, file)
		}
	}
	return files, nil
}

func newFilePaths(out []byte) []string {
	var paths []string
	for path := range bytes.SplitSeq(out, []byte{0}) {
		if len(path) > 0 {
			paths = append(paths, string(path))
		}
	}
	return paths
}

func readNewFile(root, path string) (File, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return File{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return readNewLink(full, path)
	}
	if !info.Mode().IsRegular() {
		return File{}, false, nil
	}
	if info.Size() > maxFileBytes {
		return File{
			NewPath:     path,
			DisplayPath: makeTextSafeToShow(path),
			Metadata: []string{
				"untracked file",
				"content omitted: file exceeds 2 MiB",
			},
		}, true, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read untracked %q: %w", path, err)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return File{
			NewPath:     path,
			DisplayPath: makeTextSafeToShow(path),
			Metadata:    []string{"untracked binary file"},
		}, true, nil
	}
	return makeFileFromText(
		path,
		makeSourceSafeToShow(string(content)),
	), true, nil
}

func readNewLink(full, path string) (File, bool, error) {
	target, err := os.Readlink(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
	}
	return makeFileFromText(path, makeTextSafeToShow(target)), true, nil
}

func makeFileFromText(path, content string) File {
	sourceLines := splitSourceIntoLines(content)
	lines := make([]Line, 0, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(
			lines,
			Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)},
		)
	}
	return File{
		NewPath:     path,
		DisplayPath: makeTextSafeToShow(path),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func parseChangedLines(oldLine, newLine int32, body []byte) ([]Line, error) {
	rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
	if len(body) == 0 {
		return []Line{}, nil
	}
	lines := make([]Line, 0, len(rawLines))
	for _, raw := range rawLines {
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := makeTextSafeToShow(string(raw[1:]))
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

func readStagedFile(
	ctx context.Context,
	runCommand runner,
	root, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := runCommand.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return makeSourceSafeToShow(string(out))
}

func readFileAtRevision(
	ctx context.Context,
	runCommand runner,
	root, revision, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := runCommand.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return makeSourceSafeToShow(string(out))
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
	return makeSourceSafeToShow(string(out))
}

func normalizeChangedFilePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func splitSourceIntoLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func makeStringsSafeToShow(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = makeTextSafeToShow(value)
	}
	return result
}

func makeSourceSafeToShow(value string) string {
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

func makeTextSafeToShow(value string) string {
	return strings.ReplaceAll(makeSourceSafeToShow(value), "\n", `\n`)
}

func formatChangedLinesHeader(h *diff.Hunk) string {
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
