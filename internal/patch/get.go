package patch

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
)

// Get retrieves a complete snapshot from the current working directory.
// Unknown kinds and unavailable branch comparisons return a zero Patch and an
// error.
func Get(ctx context.Context, kind Kind) (Patch, error) {
	directory, err := os.Getwd()
	if err != nil {
		return Patch{}, fmt.Errorf("resolve working directory: %w", err)
	}
	return loadPatchFromDirectory(ctx, directory, kind, execRunner{})
}

func loadPatchFromDirectory(
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
	root, err := repositoryRoot(ctx, run, directory)
	if err != nil {
		return Patch{}, err
	}
	branch, err := discoverDefaultBranch(ctx, run, root)
	if err != nil {
		return Patch{}, fmt.Errorf("discover default branch: %w", err)
	}
	repository := gitRepository{run: run, root: root}
	return repository.loadPatch(ctx, kind, branch)
}

func (r gitRepository) loadPatch(
	ctx context.Context,
	kind Kind,
	branch string,
) (Patch, error) {
	revisions, readBaseSource, err := r.comparisonBase(ctx, kind, branch)
	if err != nil {
		return Patch{}, err
	}
	raw, err := runGitDiff(ctx, r.run, r.root, revisions...)
	if err != nil {
		return Patch{}, err
	}
	files, err := r.loadPatchFiles(ctx, raw, readBaseSource)
	if err != nil {
		return Patch{}, err
	}
	return Patch{Root: r.root, Kind: kind, Branch: branch, Files: files}, nil
}

type baseSourceReader func(context.Context, string) string

func (r gitRepository) loadPatchFiles(
	ctx context.Context,
	raw []byte,
	readBaseSource baseSourceReader,
) ([]File, error) {
	files, err := r.parseTrackedFiles(ctx, raw, readBaseSource)
	if err != nil {
		return nil, err
	}
	untracked, err := readUntrackedFiles(ctx, r.run, r.root)
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
