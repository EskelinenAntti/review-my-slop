package diff

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Runner executes Git in a repository.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) ([]byte, error)
}

// ExecRunner runs Git through the operating system.
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

// Loader reads the changes Git reports for a repository.
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
