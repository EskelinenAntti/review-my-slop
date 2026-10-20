package patch

import (
	"bytes"
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"
)

func loadUntracked(
	ctx context.Context,
	run runner,
	root string,
) ([]File, error) {
	out, err := run.Run(
		ctx,
		root,
		"ls-files",
		"--others",
		"--exclude-standard",
		"-z",
	)
	if err != nil {
		return nil, err
	}
	return readUntrackedFiles(ctx, root, out)
}

func readUntrackedFiles(
	ctx context.Context,
	root string,
	output []byte,
) ([]File, error) {
	files := make([]File, 0)
	for path := range untrackedPaths(output) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, include, err := readUntrackedFile(root, path)
		if err != nil {
			return nil, err
		}
		if include {
			files = append(files, file)
		}
	}
	return files, nil
}

func untrackedPaths(output []byte) iter.Seq[string] {
	return func(yield func(string) bool) {
		for path := range bytes.SplitSeq(output, []byte{0}) {
			if len(path) > 0 && !yield(string(path)) {
				return
			}
		}
	}
}

func readUntrackedFile(root, path string) (File, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return File{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return readUntrackedSymlink(full, path)
	}
	if !info.Mode().IsRegular() {
		return File{}, false, nil
	}
	content, oversized, err := readLimitedFile(full, info)
	if err != nil {
		return File{}, false, fmt.Errorf("read untracked %q: %w", path, err)
	}
	if oversized {
		return untrackedFile(
			path,
			"untracked file",
			fmt.Sprintf(
				"content omitted: file exceeds %d MiB",
				maxFileSizeMiB,
			),
		), true, nil
	}
	return parseUntrackedContent(path, content), true, nil
}

func readUntrackedSymlink(full, path string) (File, bool, error) {
	target, err := os.Readlink(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
	}
	return addedFile(path, visibleText(target)), true, nil
}
