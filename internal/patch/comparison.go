package patch

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (r gitRepository) comparisonBase(
	ctx context.Context,
	kind Kind,
	branch string,
) ([]string, baseSourceReader, error) {
	if kind != Branch {
		return nil, r.readIndexSource, nil
	}
	if branch == "" {
		return nil, nil, errors.New("default branch is unavailable")
	}
	baseBytes, err := r.run.Run(ctx, r.root, "merge-base", branch, "HEAD")
	if err != nil {
		return nil, nil, fmt.Errorf(
			"find branch point with %s: %w",
			branch,
			err,
		)
	}
	base := strings.TrimSpace(string(baseBytes))
	readBaseSource := func(ctx context.Context, path string) string {
		return r.readRevisionSource(ctx, base, path)
	}
	return []string{base}, readBaseSource, nil
}

func discoverDefaultBranch(
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
