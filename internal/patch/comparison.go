package patch

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (r repository) comparison(
	ctx context.Context,
	kind Kind,
) (branch, revision string, err error) {
	if kind != Branch {
		return "", "", nil
	}
	branch, err = r.defaultBranch(ctx)
	if err != nil {
		return "", "", fmt.Errorf("discover default branch: %w", err)
	}
	if branch == "" {
		return "", "", errors.New("default branch is unavailable")
	}
	baseBytes, err := r.run.Run(ctx, r.root, "merge-base", branch, "HEAD")
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

func (r repository) defaultBranch(ctx context.Context) (string, error) {
	out, err := r.run.Run(
		ctx,
		r.root,
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
		_, err := r.run.Run(
			ctx,
			r.root,
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
