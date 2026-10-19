package patch

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
)

func readGitSource(
	ctx context.Context,
	run runner,
	root, revision, path string,
) string {
	if path == "" {
		return ""
	}
	source := ":" + path
	if revision != "" {
		source = revision + ":" + path
	}
	out, err := run.Run(ctx, root, "show", source)
	if err != nil {
		return ""
	}
	return safeSource(out)
}

func readWorkingTree(root, path string) string {
	if path == "" {
		return ""
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return ""
	}
	out, oversized, err := readLimitedFile(full, info)
	if err != nil || oversized {
		return ""
	}
	return safeSource(out)
}

func readLimitedFile(path string, info os.FileInfo) ([]byte, bool, error) {
	if info.Size() > maxFileBytes {
		return nil, true, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(content) > maxFileBytes {
		return nil, true, nil
	}
	return content, false, nil
}

func safeSource(source []byte) string {
	if len(source) > maxFileBytes || containsBinaryData(source) {
		return ""
	}
	return visibleSource(string(source))
}

func containsBinaryData(source []byte) bool {
	return bytes.IndexByte(source, 0) >= 0
}
