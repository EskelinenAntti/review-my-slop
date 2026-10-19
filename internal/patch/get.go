package patch

import (
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

const (
	maxFileSizeMiB = 2
	maxFileBytes   = maxFileSizeMiB << 20
)

type runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

type execRunner struct{}

func (execRunner) Run(
	ctx context.Context,
	dir string,
	args ...string,
) ([]byte, error) {
	command := "git " + strings.Join(args, " ")
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_EXTERNAL_DIFF=",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("%s: %w", command, ctx.Err())
	}
	if err == nil {
		return out, nil
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return nil, fmt.Errorf(
			"%s: %s",
			command,
			strings.TrimSpace(string(exitErr.Stderr)),
		)
	}
	return nil, fmt.Errorf("%s: %w", command, err)
}

// Get retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an
// error.
func Get(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := currentWorkingDirectory()
	if err != nil {
		return Patch{}, err
	}
	return load(ctx, directory, kind, execRunner{})
}

// RepositoryRoot resolves the current working directory's Git repository root.
func RepositoryRoot(ctx context.Context) (string, error) {
	directory, err := currentWorkingDirectory()
	if err != nil {
		return "", err
	}
	return discoverRepositoryRoot(ctx, execRunner{}, directory)
}

func currentWorkingDirectory() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	return directory, nil
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
	root, err := discoverRepositoryRoot(ctx, run, directory)
	if err != nil {
		return Patch{}, err
	}
	branch, oldRevision, err := comparison(ctx, run, root, kind)
	if err != nil {
		return Patch{}, err
	}
	raw, err := gitDiff(ctx, run, root, oldRevision)
	if err != nil {
		return Patch{}, err
	}
	files, err := loadFiles(ctx, run, root, raw, oldRevision)
	if err != nil {
		return Patch{}, err
	}
	return Patch{
		Root:   root,
		Kind:   kind,
		Branch: branch,
		Files:  files,
	}, nil
}

func comparison(
	ctx context.Context,
	run runner,
	root string,
	kind Kind,
) (branch, revision string, err error) {
	if kind != Branch {
		return "", "", nil
	}
	branch, err = defaultBranch(ctx, run, root)
	if err != nil {
		return "", "", fmt.Errorf("discover default branch: %w", err)
	}
	if branch == "" {
		return "", "", errors.New("default branch is unavailable")
	}
	baseBytes, err := run.Run(ctx, root, "merge-base", branch, "HEAD")
	if err != nil {
		return "", "", fmt.Errorf(
			"find branch point with %s: %w",
			branch,
			err,
		)
	}
	revision = strings.TrimSpace(string(baseBytes))
	return branch, revision, nil
}

func discoverRepositoryRoot(
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
	base string,
) ([]byte, error) {
	args := []string{
		"-c", "core.quotepath=false",
		"-c", "diff.external=",
		"--no-pager", "diff", "--no-ext-diff", "--no-color", "--find-renames",
		"--src-prefix=a/", "--dst-prefix=b/", "--unified=3",
	}
	if base != "" {
		args = append(args, base)
	}
	args = append(args, "--")
	return run.Run(ctx, root, args...)
}

func loadFiles(
	ctx context.Context,
	run runner,
	root string,
	raw []byte,
	oldRevision string,
) ([]File, error) {
	files, err := parseTracked(ctx, run, root, raw, oldRevision)
	if err != nil {
		return nil, err
	}
	untracked, err := loadUntracked(ctx, run, root)
	if err != nil {
		return nil, err
	}
	files = append(files, untracked...)
	slices.SortStableFunc(
		files,
		func(a, b File) int {
			return cmp.Compare(a.DisplayPath, b.DisplayPath)
		},
	)

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return files, nil
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
