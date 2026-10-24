package patch

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	maxFileSizeMiB = 2
	maxFileBytes   = maxFileSizeMiB << 20
)

// repository binds Git commands and working-tree reads to a discovered root.
type repository struct {
	root string
	run  runner
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
	repo := repository{root: root, run: run}
	branch, oldRevision, err := repo.comparison(ctx, kind)
	if err != nil {
		return Patch{}, err
	}
	files, err := repo.loadFiles(ctx, oldRevision)
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

func (r repository) gitDiff(
	ctx context.Context,
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
	return r.run.Run(ctx, r.root, args...)
}

func (r repository) loadFiles(
	ctx context.Context,
	oldRevision string,
) ([]File, error) {
	raw, err := r.gitDiff(ctx, oldRevision)
	if err != nil {
		return nil, err
	}
	files, err := r.parseTracked(ctx, raw, oldRevision)
	if err != nil {
		return nil, err
	}
	untracked, err := r.loadUntracked(ctx)
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
