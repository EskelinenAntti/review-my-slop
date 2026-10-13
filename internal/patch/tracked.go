package patch

import (
	"bytes"
	"context"
	"fmt"

	"github.com/sourcegraph/go-diff/diff"
)

func (r gitRepository) parseTrackedFiles(
	ctx context.Context,
	raw []byte,
	readBaseSource baseSourceReader,
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
		file, err := r.parseTrackedDiffFile(ctx, fd, readBaseSource)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func (r gitRepository) parseTrackedDiffFile(
	ctx context.Context,
	fd *diff.FileDiff,
	readBaseSource baseSourceReader,
) (File, error) {
	oldPath := cleanDiffPath(fd.OrigName)
	newPath := cleanDiffPath(fd.NewName)
	display := newPath
	if display == "" || display == "/dev/null" {
		display = oldPath
	}
	file := File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: escapeDisplayText(display),
		Metadata:    escapeDisplayTexts(fd.Extended),
	}
	file.OldSource = readBaseSource(ctx, oldPath)
	file.NewSource = readWorkingTreeSource(r.root, newPath)
	for _, hunk := range fd.Hunks {
		lines, err := parseUnifiedHunkBody(
			hunk.OrigStartLine,
			hunk.NewStartLine,
			hunk.Body,
		)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(file.Hunks, Hunk{
			Header: formatUnifiedHunkHeader(hunk),
			Lines:  lines,
		})
	}
	return file, nil
}
