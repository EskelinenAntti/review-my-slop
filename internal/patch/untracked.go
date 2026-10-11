package patch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func loadUntracked(ctx context.Context, run runner, root string) (
	[]File,
	error,
) {
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
	var files []File
	for rawPath := range bytes.SplitSeq(out, []byte{0}) {
		files, err = appendUntrackedPath(ctx, root, rawPath, files)
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

func untrackedFile(root, path string) (File, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return File{}, false, fmt.Errorf("stat untracked %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
		}
		return addedFile(path, visibleText(target)), true, nil
	}
	if !info.Mode().IsRegular() {
		return File{}, false, nil
	}
	file, err := untrackedRegularFile(full, path, info.Size())
	return file, err == nil, err
}

func untrackedRegularFile(full, path string, size int64) (File, error) {
	if size > maxFileBytes {
		return File{
			NewPath:     path,
			DisplayPath: visibleText(path),
			Metadata: []string{
				"untracked file",
				"content omitted: file exceeds 2 MiB",
			},
		}, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return File{}, fmt.Errorf("read untracked %q: %w", path, err)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return File{
			NewPath:     path,
			DisplayPath: visibleText(path),
			Metadata:    []string{"untracked binary file"},
		}, nil
	}
	return addedFile(path, visibleSource(string(content))), nil
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, 0, len(sourceLines))
	for i, line := range sourceLines {
		lines = append(
			lines,
			Line{Kind: Addition, Text: line, NewNumber: LineNumber(i + 1)},
		)
	}
	return File{
		NewPath:     path,
		DisplayPath: visibleText(path),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func appendUntrackedPath(
	ctx context.Context,
	root string,
	rawPath []byte,
	files []File,
) (
	[]File,
	error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(rawPath) == 0 {
		return files, nil
	}
	file, include, err := untrackedFile(root, string(rawPath))
	if err != nil {
		return nil, err
	}
	if include {
		return append(files, file), nil
	}
	return files, nil
}
