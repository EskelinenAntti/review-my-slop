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
	"strconv"
	"strings"

	"github.com/sourcegraph/go-diff/diff"

	"github.com/eskelinenantti/review-my-slop/internal/review"
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

func (l Loader) StageAll(ctx context.Context, root string) error {
	if l.Runner == nil {
		l.Runner = ExecRunner{}
	}
	if _, err := l.Runner.Run(ctx, root, "add", "--all", "--"); err != nil {
		return fmt.Errorf("stage local changes: %w", err)
	}
	return nil
}

func (l Loader) Root(ctx context.Context, dir string) (string, error) {
	if l.Runner == nil {
		l.Runner = ExecRunner{}
	}
	rootBytes, err := l.Runner.Run(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(rootBytes)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}

func (l Loader) Load(ctx context.Context, dir string) (review.Diff, error) {
	if l.Runner == nil {
		l.Runner = ExecRunner{}
	}
	root, err := l.Root(ctx, dir)
	if err != nil {
		return review.Diff{}, err
	}

	raw, err := l.diff(ctx, root)
	if err != nil {
		return review.Diff{}, err
	}
	return l.build(ctx, root, "", raw, readIndex)
}

func (l Loader) LoadBranch(ctx context.Context, dir, parent string) (review.Diff, error) {
	if l.Runner == nil {
		l.Runner = ExecRunner{}
	}
	root, err := l.Root(ctx, dir)
	if err != nil {
		return review.Diff{}, err
	}
	baseBytes, err := l.Runner.Run(ctx, root, "merge-base", parent, "HEAD")
	if err != nil {
		return review.Diff{}, fmt.Errorf("find branch point with %s: %w", parent, err)
	}
	base := strings.TrimSpace(string(baseBytes))
	raw, err := l.diff(ctx, root, base)
	if err != nil {
		return review.Diff{}, err
	}
	readBase := func(ctx context.Context, runner Runner, root, path string) string {
		return readRevision(ctx, runner, root, base, path)
	}
	return l.build(ctx, root, parent, raw, readBase)
}

func (l Loader) ParentBranches(ctx context.Context, dir string) ([]string, error) {
	if l.Runner == nil {
		l.Runner = ExecRunner{}
	}
	root, err := l.Root(ctx, dir)
	if err != nil {
		return nil, err
	}
	currentBytes, err := l.Runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return nil, nil
	}
	current := strings.TrimSpace(string(currentBytes))
	upstream := ""
	if upstreamBytes, upstreamErr := l.Runner.Run(ctx, root, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); upstreamErr == nil {
		upstream = strings.TrimSpace(string(upstreamBytes))
	}
	defaultBranch := l.defaultBranch(ctx, root)
	refsBytes, err := l.Runner.Run(ctx, root, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}

	type candidate struct {
		name     string
		base     string
		distance int
		priority int
	}
	byBase := make(map[string]candidate)
	for name := range strings.FieldsSeq(string(refsBytes)) {
		if name == current || name == upstream || name == "origin/HEAD" || strings.HasSuffix(name, "/HEAD") {
			continue
		}
		if name != defaultBranch {
			if _, ancestorErr := l.Runner.Run(ctx, root, "merge-base", "--is-ancestor", name, "HEAD"); ancestorErr != nil {
				continue
			}
		}
		baseBytes, mergeErr := l.Runner.Run(ctx, root, "merge-base", name, "HEAD")
		if mergeErr != nil {
			continue
		}
		base := strings.TrimSpace(string(baseBytes))
		distanceBytes, countErr := l.Runner.Run(ctx, root, "rev-list", "--count", base+"..HEAD")
		if countErr != nil {
			continue
		}
		distance, parseErr := strconv.Atoi(strings.TrimSpace(string(distanceBytes)))
		if parseErr != nil {
			continue
		}
		next := candidate{name: name, base: base, distance: distance, priority: branchPriority(name, defaultBranch)}
		if previous, ok := byBase[base]; !ok ||
			next.priority < previous.priority ||
			next.priority == previous.priority && next.name < previous.name {
			byBase[base] = next
		}
	}
	candidates := make([]candidate, 0, len(byBase))
	for _, item := range byBase {
		candidates = append(candidates, item)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance != candidates[j].distance {
			return candidates[i].distance < candidates[j].distance
		}
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority < candidates[j].priority
		}
		return candidates[i].name < candidates[j].name
	})
	parents := make([]string, len(candidates))
	for i, item := range candidates {
		parents[i] = item.name
	}
	return parents, nil
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
	return l.Runner.Run(ctx, root, args...)
}

type sourceReader func(context.Context, Runner, string, string) string

func (l Loader) build(ctx context.Context, root, base string, raw []byte, readOld sourceReader) (review.Diff, error) {
	files, err := parseTracked(ctx, l.Runner, root, raw, readOld)
	if err != nil {
		return review.Diff{}, err
	}
	untracked, err := l.loadUntracked(ctx, root)
	if err != nil {
		return review.Diff{}, err
	}
	files = append(files, untracked...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Display < files[j].Display })

	hash := sha256.New()
	_, _ = hash.Write([]byte(base))
	_, _ = hash.Write(raw)
	for _, file := range untracked {
		_, _ = hash.Write([]byte(file.Display))
		_, _ = hash.Write([]byte(file.NewSource))
	}

	return review.Diff{
		Repository:  root,
		Fingerprint: hex.EncodeToString(hash.Sum(nil)),
		Base:        base,
		Files:       files,
	}, nil
}

func (l Loader) defaultBranch(ctx context.Context, root string) string {
	if out, err := l.Runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		return strings.TrimSpace(string(out))
	}
	for _, candidate := range []string{"origin/main", "main", "origin/master", "master"} {
		if _, err := l.Runner.Run(ctx, root, "rev-parse", "--verify", "--quiet", candidate+"^{commit}"); err == nil {
			return candidate
		}
	}
	return ""
}

func branchPriority(name, defaultBranch string) int {
	if name == defaultBranch {
		return 0
	}
	if !strings.Contains(name, "/") {
		return 1
	}
	return 2
}

func parseTracked(ctx context.Context, runner Runner, root string, raw []byte, readOld sourceReader) ([]review.File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]review.File, 0, len(parsed))
	for _, fd := range parsed {
		oldPath := cleanDiffPath(fd.OrigName)
		newPath := cleanDiffPath(fd.NewName)
		display := newPath
		if display == "" || display == "/dev/null" {
			display = oldPath
		}
		file := review.File{
			OldPath:  oldPath,
			NewPath:  newPath,
			Display:  visibleText(display),
			Language: display,
			Metadata: visibleStrings(fd.Extended),
		}
		file.OldSource = readOld(ctx, runner, root, oldPath)
		file.NewSource = readWorkingTree(root, newPath)
		for _, h := range fd.Hunks {
			lines, parseErr := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: %w", display, parseErr)
			}
			file.Hunks = append(file.Hunks, review.Hunk{
				Header: formatHunkHeader(h),
				Lines:  lines,
			})
		}
		file.Binary = len(fd.Hunks) == 0 && hasBinaryMetadata(fd.Extended)
		files = append(files, file)
	}
	return files, nil
}

func (l Loader) loadUntracked(ctx context.Context, root string) ([]review.File, error) {
	out, err := l.Runner.Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []review.File
	for rawPath := range bytes.SplitSeq(out, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		path := string(rawPath)
		display := visibleText(path)
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
			files = append(files, addedFile(display, visibleText(target)))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxFileBytes {
			files = append(files, review.File{
				NewPath:  path,
				Display:  display,
				Language: path,
				Metadata: []string{"untracked file", "content omitted: file exceeds 2 MiB"},
				Binary:   true,
			})
			continue
		}
		content, readErr := os.ReadFile(full)
		if readErr != nil {
			return nil, fmt.Errorf("read untracked %q: %w", path, readErr)
		}
		if bytes.IndexByte(content, 0) >= 0 {
			files = append(files, review.File{
				NewPath:  path,
				Display:  display,
				Language: path,
				Metadata: []string{"untracked binary file"},
				Binary:   true,
			})
			continue
		}
		files = append(files, addedFile(display, visibleSource(string(content))))
	}
	return files, nil
}

func addedFile(path, content string) review.File {
	sourceLines := splitSourceLines(content)
	lines := make([]review.Line, 0, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(lines, review.Line{Kind: review.LineAdded, Text: line, NewNumber: i + 1})
	}
	return review.File{
		NewPath:   path,
		Display:   path,
		Language:  path,
		NewSource: content,
		Metadata:  []string{"untracked file"},
		Hunks: []review.Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]review.Line, error) {
	rawLines := bytes.Split(body, []byte("\n"))
	lines := make([]review.Line, 0, len(rawLines))
	for i, raw := range rawLines {
		if i == len(rawLines)-1 && len(raw) == 0 {
			continue
		}
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := visibleText(string(raw[1:]))
		switch raw[0] {
		case ' ':
			lines = append(lines, review.Line{Kind: review.LineContext, Text: text, OldNumber: int(oldLine), NewNumber: int(newLine)})
			oldLine++
			newLine++
		case '+':
			lines = append(lines, review.Line{Kind: review.LineAdded, Text: text, NewNumber: int(newLine)})
			newLine++
		case '-':
			lines = append(lines, review.Line{Kind: review.LineRemoved, Text: text, OldNumber: int(oldLine)})
			oldLine++
		case '\\':
			// "\ No newline at end of file" belongs to the preceding line.
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
	out, err := runner.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return visibleSource(string(out))
}

func readRevision(ctx context.Context, runner Runner, root, revision, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := runner.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return visibleSource(string(out))
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

func hasBinaryMetadata(lines []string) bool {
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "binary") {
			return true
		}
	}
	return false
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

func formatHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.OrigStartLine, h.OrigLines, h.NewStartLine, h.NewLines)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}
