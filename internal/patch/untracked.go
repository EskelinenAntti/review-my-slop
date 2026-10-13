package patch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readUntrackedFiles(
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
	return readUntrackedFilesAtPaths(ctx, root, splitNULSeparatedPaths(out))
}

func readUntrackedFilesAtPaths(
	ctx context.Context,
	root string,
	paths []string,
) ([]File, error) {
	var files []File
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

func splitNULSeparatedPaths(out []byte) []string {
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
	if info.Size() > maxFileBytes {
		return File{
			NewPath:     path,
			DisplayPath: escapeDisplayText(path),
			Metadata: []string{
				"untracked file",
				"content omitted: file exceeds 2 MiB",
			},
		}, true, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read untracked %q: %w", path, err)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return File{
			NewPath:     path,
			DisplayPath: escapeDisplayText(path),
			Metadata:    []string{"untracked binary file"},
		}, true, nil
	}
	return buildAddedFile(path, escapeSourceText(string(content))), true, nil
}

func readUntrackedSymlink(full, path string) (File, bool, error) {
	target, err := os.Readlink(full)
	if err != nil {
		return File{}, false, fmt.Errorf("read symlink %q: %w", path, err)
	}
	return buildAddedFile(path, escapeDisplayText(target)), true, nil
}

func splitSourceLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func buildAddedFile(path, content string) File {
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
		DisplayPath: escapeDisplayText(path),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}
