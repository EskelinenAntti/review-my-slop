package patch

import (
	"bytes"
	"context"
	"fmt"
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
	return readUntrackedPaths(ctx, root, untrackedPaths(out))
}

func readUntrackedPaths(
	ctx context.Context,
	root string,
	paths []string,
) ([]File, error) {
	files := make([]File, 0, len(paths))
	for _, path := range paths {
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

func untrackedPaths(out []byte) []string {
	var paths []string
	for path := range bytes.SplitSeq(out, []byte{0}) {
		if len(path) > 0 {
			paths = append(paths, string(path))
		}
	}
	return paths
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
	if containsBinaryData(content) {
		return untrackedFile(path, "untracked binary file"), true, nil
	}
	return addedFile(path, visibleSource(string(content))), true, nil
}

func readUntrackedSymlink(full, path string) (File, bool, error) {
	target, err := os.Readlink(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
	}
	return addedFile(path, visibleText(target)), true, nil
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, len(sourceLines))
	for i, line := range sourceLines {
		lines[i] = Line{
			Kind:      Addition,
			Text:      line,
			NewNumber: LineNumber(i + 1),
		}
	}
	file := untrackedFile(path, "untracked file")
	file.NewSource = content
	file.Hunks = []Hunk{{
		Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
		Lines:  lines,
	}}
	return file
}

func untrackedFile(path string, metadata ...string) File {
	return File{
		NewPath:     path,
		DisplayPath: visibleText(path),
		Metadata:    metadata,
	}
}
