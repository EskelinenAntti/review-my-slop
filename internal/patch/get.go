package patch

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
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

// fileLoader shares the repository and comparison source across file loading.
// Context remains operation-scoped so cancellation is explicit at each step.
type fileLoader struct {
	run         runner
	root        string
	oldRevision string
}

func (l fileLoader) load(ctx context.Context, raw []byte) ([]File, error) {
	files, err := l.loadTracked(ctx, raw)
	if err != nil {
		return nil, err
	}
	untracked, err := loadUntracked(ctx, l.run, l.root)
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

func (l fileLoader) loadTracked(
	ctx context.Context,
	raw []byte,
) ([]File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, 0, len(parsed))
	for _, fd := range parsed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := parseTrackedFile(fd)
		if err != nil {
			return nil, err
		}
		file.OldSource = readGitSource(
			ctx, l.run, l.root, l.oldRevision, file.OldPath,
		)
		file.NewSource = readWorkingTree(l.root, file.NewPath)
		files = append(files, file)
	}
	return files, nil
}
