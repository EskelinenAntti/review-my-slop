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

type sourceReader func(context.Context, runner, string, string) string

// Load reads local or branch changes from the current Git repository.
func Load(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return Patch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	if err := validatePatchLoad(ctx, kind); err != nil {
		return Patch{}, err
	}
	root, err := resolveRepositoryRoot(ctx, directory)
	if err != nil {
		return Patch{}, err
	}
	branch, err := findDefaultBranch(ctx, root)
	if err != nil {
		return Patch{}, fmt.Errorf("discover default branch: %w", err)
	}
	return loadRepositoryPatch(ctx, kind, root, branch)
}

func validatePatchLoad(ctx context.Context, kind Kind) error {
	if kind != Unstaged && kind != Branch {
		return fmt.Errorf("unknown patch kind %d", kind)
	}
	return ctx.Err()
}

func loadRepositoryPatch(
	ctx context.Context,
	kind Kind,
	root, branch string,
) (Patch, error) {
	revisions, readOld, err := branchComparisonInputs(ctx, branch, kind, root)
	if err != nil {
		return Patch{}, err
	}
	raw, err := runPatchDiff(ctx, revisions, root)
	if err != nil {
		return Patch{}, err
	}
	files, err := parsePatchDiff(ctx, raw, readOld, root)
	if err != nil {
		return Patch{}, err
	}
	return finishPatchLoad(ctx, kind, root, branch, files)
}

func finishPatchLoad(
	ctx context.Context,
	kind Kind,
	root, branch string,
	files []File,
) (Patch, error) {
	untracked, err := loadUntrackedFiles(ctx, root)
	if err != nil {
		return Patch{}, err
	}
	files = append(files, untracked...)
	slices.SortStableFunc(files, comparePatchFilesByPath)
	if err := ctx.Err(); err != nil {
		return Patch{}, err
	}
	return Patch{Root: root, Kind: kind, Branch: branch, Files: files}, nil
}

func comparePatchFilesByPath(a, b File) int {
	return cmp.Compare(a.DisplayPath, b.DisplayPath)
}

func loadUntrackedFiles(ctx context.Context, root string,

) ([]File, error) {
	out, err := runner(execRunner{}).Run(
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
	for _, path := range splitNULPaths(out) {
		file, include, err := loadUntrackedEntry(ctx, path, root)
		if err != nil {
			return nil, err
		}
		if include {
			files = append(files, file)
		}
	}
	return files, nil
}

func loadUntrackedEntry(
	ctx context.Context,
	path, root string,
) (File, bool, error) {
	if err := ctx.Err(); err != nil {
		return File{}, false, err
	}
	return loadOneUntrackedFile(path, root)
}

func splitNULPaths(output []byte) []string {
	var paths []string
	for path := range bytes.SplitSeq(output, []byte{0}) {
		if len(path) > 0 {
			paths = append(paths, string(path))
		}
	}
	return paths
}

func branchComparisonInputs(
	ctx context.Context,
	branch string,
	kind Kind,
	root string,

) ([]string, sourceReader, error) {
	if kind != Branch {
		return nil, readIndexSource, nil
	}
	if branch == "" {
		return nil, nil, errors.New("default branch is unavailable")
	}
	baseBytes, err := runner(
		execRunner{},
	).Run(ctx, root, "merge-base", branch, "HEAD")
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
		run runner,
		root, path string,
	) string {
		return readBranchBaseSource(ctx, run, root, base, path)
	}
	return []string{base}, readOld, nil
}

func readBranchBaseSource(
	ctx context.Context, run runner, root, base, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", base+":"+path)
	if err != nil || len(out) > maxFileBytes ||
		bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceText(string(out))
}

func resolveRepositoryRoot(ctx context.Context, directory string,

) (string, error) {
	rootBytes, err := runner(execRunner{}).Run(
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

func findDefaultBranch(ctx context.Context, root string,

) (string, error) {
	return findDefaultBranchWithRunner(ctx, execRunner{}, root)
}

func findDefaultBranchWithRunner(
	ctx context.Context,
	runner runner,
	root string,
) (string, error) {
	out, err := runner.Run(
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
		_, err := runner.Run(
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

func runPatchDiff(ctx context.Context, revisions []string, root string,

) ([]byte, error) {
	args := []string{
		"-c",
		"core.quotepath=false",
		"-c",
		"diff.external=",
		"--no-pager",
		"diff",
		"--no-ext-diff",
		"--no-color",
		"--find-renames",
		"--src-prefix=a/",
		"--dst-prefix=b/",
		"--unified=3",
	}
	args = append(args, revisions...)
	args = append(args, "--")
	return runner(execRunner{}).Run(ctx, root, args...)
}
