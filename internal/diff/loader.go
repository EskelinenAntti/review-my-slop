package diff

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
)

const maxFileBytes = 2 << 20

type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	output, err := command.Output()
	if err == nil {
		return output, nil
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
	rootBytes, err := l.gitRunner().Run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}

func (l Loader) Load(ctx context.Context, dir string) (ChangeSet, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return ChangeSet{}, err
	}
	raw, err := l.loadDiff(ctx, root)
	if err != nil {
		return ChangeSet{}, err
	}
	return l.assemble(ctx, root, "", raw, readIndex)
}

func (l Loader) LoadBranch(ctx context.Context, dir, branch string) (ChangeSet, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return ChangeSet{}, err
	}
	baseBytes, err := l.gitRunner().Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return ChangeSet{}, fmt.Errorf("find branch point with %s: %w", branch, err)
	}
	base := strings.TrimSpace(string(baseBytes))
	raw, err := l.loadDiff(ctx, root, base)
	if err != nil {
		return ChangeSet{}, err
	}
	readBase := func(ctx context.Context, runner Runner, repository, path string) string {
		return readRevision(ctx, runner, repository, base, path)
	}
	return l.assemble(ctx, root, branch, raw, readBase)
}

func (l Loader) DefaultBranch(ctx context.Context, dir string) (string, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return "", err
	}
	return l.defaultBranch(ctx, root), nil
}

func (l Loader) gitRunner() Runner {
	if l.Runner != nil {
		return l.Runner
	}
	return ExecRunner{}
}

func (l Loader) loadDiff(ctx context.Context, root string, revisions ...string) ([]byte, error) {
	args := []string{
		"-c", "core.quotepath=false",
		"-c", "diff.external=",
		"--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames",
		"--src-prefix=a/", "--dst-prefix=b/", "--unified=3",
	}
	args = append(args, revisions...)
	args = append(args, "--")
	return l.gitRunner().Run(ctx, root, args...)
}

type sourceReader func(context.Context, Runner, string, string) string

func (l Loader) assemble(ctx context.Context, root, base string, raw []byte, readOld sourceReader) (ChangeSet, error) {
	files, err := parseTracked(ctx, l.gitRunner(), root, raw, readOld)
	if err != nil {
		return ChangeSet{}, err
	}
	untracked, err := l.loadUntracked(ctx, root)
	if err != nil {
		return ChangeSet{}, err
	}
	files = append(files, untracked...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].DisplayPath < files[j].DisplayPath })

	hash := sha256.New()
	_, _ = hash.Write([]byte(base))
	_, _ = hash.Write(raw)
	for _, file := range untracked {
		_, _ = hash.Write([]byte(file.NewPath))
		_, _ = hash.Write([]byte(file.NewSource))
	}

	return ChangeSet{
		Repository:  root,
		Fingerprint: hex.EncodeToString(hash.Sum(nil)),
		Files:       files,
	}, nil
}

func (l Loader) defaultBranch(ctx context.Context, root string) string {
	runner := l.gitRunner()
	if output, err := runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(string(output))
	}
	for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
		if _, err := runner.Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}"); err == nil {
			return candidate
		}
	}
	return ""
}

func parseTracked(ctx context.Context, runner Runner, root string, raw []byte, readOld sourceReader) ([]File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, 0, len(parsed))
	for _, fileDiff := range parsed {
		oldPath := cleanDiffPath(fileDiff.OrigName)
		newPath := cleanDiffPath(fileDiff.NewName)
		displayPath := newPath
		if displayPath == "" || displayPath == "/dev/null" {
			displayPath = oldPath
		}
		file := File{
			OldPath:     oldPath,
			NewPath:     newPath,
			DisplayPath: visibleText(displayPath),
			Metadata:    visibleStrings(fileDiff.Extended),
		}
		file.OldSource = readOld(ctx, runner, root, oldPath)
		file.NewSource = readWorkingTree(root, newPath)
		for _, hunk := range fileDiff.Hunks {
			lines, parseErr := parseHunkBody(hunk.OrigStartLine, hunk.NewStartLine, hunk.Body)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: %w", displayPath, parseErr)
			}
			file.Hunks = append(file.Hunks, Hunk{
				Header: formatHunkHeader(hunk),
				Lines:  lines,
			})
		}
		files = append(files, file)
	}
	return files, nil
}

func (l Loader) loadUntracked(ctx context.Context, root string) ([]File, error) {
	output, err := l.gitRunner().Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []File
	for rawPath := range bytes.SplitSeq(output, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		path := string(rawPath)
		displayPath := visibleText(path)
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		info, statErr := os.Lstat(fullPath)
		if statErr != nil {
			return nil, fmt.Errorf("stat untracked %q: %w", path, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(fullPath)
			if readErr != nil {
				return nil, fmt.Errorf("read symlink %q: %w", path, readErr)
			}
			files = append(files, addedFile(path, visibleText(target)))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxFileBytes {
			files = append(files, File{
				NewPath:     path,
				DisplayPath: displayPath,
				Metadata:    []string{"untracked file", "content omitted: file exceeds 2 MiB"},
			})
			continue
		}
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return nil, fmt.Errorf("read untracked %q: %w", path, readErr)
		}
		if bytes.IndexByte(content, 0) >= 0 {
			files = append(files, File{
				NewPath:     path,
				DisplayPath: displayPath,
				Metadata:    []string{"untracked binary file"},
			})
			continue
		}
		files = append(files, addedFile(path, visibleSource(string(content))))
	}
	return files, nil
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, 0, len(sourceLines))
	for index, line := range sourceLines {
		lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(index + 1)})
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
	for index, raw := range rawLines {
		if index == len(rawLines)-1 && len(raw) == 0 {
			continue
		}
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := visibleText(string(raw[1:]))
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
			// "\\ No newline at end of file" belongs to the preceding line.
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func readIndex(ctx context.Context, runner Runner, root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	output, err := runner.Run(ctx, root, "show", ":"+path)
	if err != nil || len(output) > maxFileBytes || bytes.IndexByte(output, 0) >= 0 {
		return ""
	}
	return visibleSource(string(output))
}

func readRevision(ctx context.Context, runner Runner, root, revision, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	output, err := runner.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(output) > maxFileBytes || bytes.IndexByte(output, 0) >= 0 {
		return ""
	}
	return visibleSource(string(output))
}

func readWorkingTree(root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(fullPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return ""
	}
	output, err := os.ReadFile(fullPath)
	if err != nil || bytes.IndexByte(output, 0) >= 0 {
		return ""
	}
	return visibleSource(string(output))
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
	for _, char := range value {
		switch {
		case char == '\n' || char == '\t':
			result.WriteRune(char)
		case char == '\r':
			result.WriteString(`\r`)
		case char < 0x20 || char == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, char)
		default:
			result.WriteRune(char)
		}
	}
	return result.String()
}

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), "\n", `\n`)
}

func formatHunkHeader(hunk *diff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.OrigStartLine, hunk.OrigLines, hunk.NewStartLine, hunk.NewLines)
	if hunk.Section != "" {
		header += " " + hunk.Section
	}
	return header
}
