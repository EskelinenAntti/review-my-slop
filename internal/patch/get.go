package patch

import (
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
