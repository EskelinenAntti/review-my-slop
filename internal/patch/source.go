package patch

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
)

func (r gitRepository) readIndexSource(
	ctx context.Context,
	path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := r.run.Run(ctx, r.root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceText(string(out))
}

func (r gitRepository) readRevisionSource(
	ctx context.Context,
	revision, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := r.run.Run(ctx, r.root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceText(string(out))
}

func readWorkingTreeSource(root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return ""
	}
	out, err := os.ReadFile(full)
	if err != nil || bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceText(string(out))
}
