package patch

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/sourcegraph/go-diff/diff"
)

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
