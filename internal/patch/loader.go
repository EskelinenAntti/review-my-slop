package patch

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

type Runner func(ctx context.Context, dir string, args ...string) ([]byte, error)

func runGit(ctx context.Context, dir string, args ...string) ([]byte, error) {
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

func (l *Loader) Root(ctx context.Context, dir string) (string, error) {
	if l.Runner == nil {
		l.Runner = runGit
	}
	rootBytes, err := l.Runner(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}

func (l *Loader) Load(ctx context.Context, dir string) (Patch, error) {
	return l.load(ctx, dir, "", false)
}

func (l *Loader) LoadBranch(ctx context.Context, dir, branch string) (Patch, error) {
	return l.load(ctx, dir, branch, true)
}

func (l *Loader) load(ctx context.Context, dir, branch string, compare bool) (Patch, error) {
	root, err := l.Root(ctx, dir)
	if err != nil {
		return Patch{}, err
	}
	var revisions []string
	if compare {
		baseBytes, err := l.Runner(ctx, root, "merge-base", branch, "HEAD")
		if err != nil {
			return Patch{}, fmt.Errorf("find branch point with %s: %w", branch, err)
		}
		base := strings.TrimSpace(string(baseBytes))
		revisions = []string{base}
	}
	raw, err := l.diff(ctx, root, revisions...)
	if err != nil {
		return Patch{}, err
	}
	return l.build(ctx, root, branch, raw)
}

func (l *Loader) DefaultBranch(ctx context.Context, dir string) (string, error) {
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
	return l.Runner(ctx, root, args...)
}

func (l Loader) build(ctx context.Context, root, base string, raw []byte) (Patch, error) {
	files, err := parseTracked(ctx, l.Runner, root, raw, base)
	if err != nil {
		return Patch{}, err
	}
	untracked, err := l.loadUntracked(ctx, root)
	if err != nil {
		return Patch{}, err
	}
	files = append(files, untracked...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].DisplayPath < files[j].DisplayPath })

	hash := sha256.New()
	_, _ = hash.Write(append([]byte(base), raw...))
	for _, file := range untracked {
		_, _ = hash.Write(append([]byte(file.NewPath), []byte(file.NewSource)...))
	}

	return Patch{
		Repository:  root,
		Fingerprint: hex.EncodeToString(hash.Sum(nil)),
		Files:       files,
	}, nil
}

func (l Loader) defaultBranch(ctx context.Context, root string) string {
	if out, err := l.Runner(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}
	for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
		if _, err := l.Runner(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}"); err == nil {
			return candidate
		}
	}
	return ""
}

func parseTracked(ctx context.Context, runner Runner, root string, raw []byte, base string) ([]File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := []File{}
	for _, fd := range parsed {
		oldPath, newPath := cleanDiffPath(fd.OrigName), cleanDiffPath(fd.NewName)
		display := newPath
		if display == "" {
			display = oldPath
		}
		metadata := make([]string, len(fd.Extended))
		for index, value := range fd.Extended {
			metadata[index] = visibleText(value)
		}
		file := File{
			OldPath:     oldPath,
			NewPath:     newPath,
			DisplayPath: visibleText(display),
			Metadata:    metadata,
		}
		if oldPath != "" {
			if out, readErr := runner(ctx, root, "show", base+":"+oldPath); readErr == nil && len(out) <= maxFileBytes && bytes.IndexByte(out, 0) < 0 {
				file.OldSource = visibleSource(string(out))
			}
		}
		if newPath != "" {
			full := filepath.Join(root, filepath.FromSlash(newPath))
			if info, statErr := os.Lstat(full); statErr == nil && info.Mode().IsRegular() && info.Size() <= maxFileBytes {
				if out, readErr := os.ReadFile(full); readErr == nil && bytes.IndexByte(out, 0) < 0 {
					file.NewSource = visibleSource(string(out))
				}
			}
		}
		for _, h := range fd.Hunks {
			lines, parseErr := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: %w", display, parseErr)
			}
			header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
			if h.Section != "" {
				header += " " + h.Section
			}
			file.Hunks = append(file.Hunks, Hunk{
				Header: header,
				Lines:  lines,
			})
		}
		files = append(files, file)
	}
	return files, nil
}

func (l Loader) loadUntracked(ctx context.Context, root string) ([]File, error) {
	out, err := l.Runner(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []File
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
		mode, file := info.Mode(), File{NewPath: path, DisplayPath: visibleText(path)}
		switch {
		case mode&os.ModeSymlink != 0:
			target, readErr := os.Readlink(full)
			if readErr != nil {
				return nil, fmt.Errorf("read symlink %q: %w", path, readErr)
			}
			file = addedFile(path, visibleText(target))
		case !mode.IsRegular():
			continue
		case info.Size() > maxFileBytes:
			file.Metadata = []string{"untracked file", "content omitted: file exceeds 2 MiB"}
		default:
			content, readErr := os.ReadFile(full)
			if readErr != nil {
				return nil, fmt.Errorf("read untracked %q: %w", path, readErr)
			}
			if bytes.IndexByte(content, 0) >= 0 {
				file.Metadata = []string{"untracked binary file"}
			} else {
				file = addedFile(file.DisplayPath, visibleSource(string(content)))
			}
		}
		files = append(files, file)
	}
	return files, nil
}

func addedFile(path, content string) File {
	source := strings.TrimSuffix(content, "\n")
	lines := []Line{}
	if source != "" {
		for i, line := range strings.Split(source, "\n") {
			lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)})
		}
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
	lines := []Line{}
	for i, raw := range rawLines {
		if len(raw) == 0 {
			if i == len(rawLines)-1 {
				continue
			}
			return nil, errors.New("malformed empty diff line")
		}
		line := Line{Text: visibleText(string(raw[1:]))}
		switch raw[0] {
		case ' ':
			line.Kind, line.OldNumber, line.NewNumber = Context, LineNumber(oldLine), LineNumber(newLine)
			oldLine++
			newLine++
		case '+':
			line.Kind, line.NewNumber = Addition, LineNumber(newLine)
			newLine++
		case '-':
			line.Kind, line.OldNumber = Deletion, LineNumber(oldLine)
			oldLine++
		case '\\':
			// "\ No newline at end of file" belongs to the preceding line.
			continue
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func cleanDiffPath(path string) string {
	path = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(path), "a/"), "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func visibleSource(value string) string {
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

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), "\n", `\n`)
}
