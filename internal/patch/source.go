package patch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readIndex(ctx context.Context, run runner, root, path string) string {
	if path == "" || path == nullPath {
		return ""
	}
	out, err := run.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes ||
		bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return visibleSource(string(out))
}

func readRevision(
	ctx context.Context,
	run runner,
	root, revision, path string,
) string {
	if path == "" || path == nullPath {
		return ""
	}
	out, err := run.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(out) > maxFileBytes ||
		bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return visibleSource(string(out))
}

func readWorkingTree(root, path string) string {
	if path == "" || path == nullPath {
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
	return visibleSource(string(out))
}

func splitSourceLines(initialContent string) []string {
	content := initialContent

	content = strings.TrimSuffix(content, newline)
	if content == "" {
		return nil
	}
	return strings.Split(content, newline)
}

func visibleStrings(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = visibleText(value)
	}
	return result
}

func visibleSource(value string) string {
	var result strings.Builder
	for _, r := range value {
		switch {
		case r == '\n' || r == '\t':
			_, _ = result.WriteRune(r)
		case r == '\r':
			_, _ = result.WriteString(`\r`)
		case r < firstPrintableASCII || r == deleteASCII:
			fmt.Fprintf(&result, `\x%02x`, r)
		default:
			_, _ = result.WriteRune(r)
		}
	}
	return result.String()
}

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), newline, `\n`)
}
