package patch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
