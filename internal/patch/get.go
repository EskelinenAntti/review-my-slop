package patch

import (
	"context"
	"fmt"
	"os"
)

const (
	maxFileSizeMiB = 2
	maxFileBytes   = maxFileSizeMiB << 20
)

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
	loader := fileLoader{run: run, root: root, oldRevision: oldRevision}
	files, err := loader.load(ctx, raw)
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
