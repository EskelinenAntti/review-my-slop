package patch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return ""
	}
	out, err := os.ReadFile(full)
	if err != nil {
		return ""
	}
	return safeSource(out)
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

func splitSourceLines(content string) []string {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
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
	result.Grow(len(value))
	for _, r := range value {
		switch {
		case r == '\n' || r == '\t':
			_, _ = result.WriteRune(r)
		case r == '\r':
			_, _ = result.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, r)
		default:
			_, _ = result.WriteRune(r)
		}
	}
	return result.String()
}

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), "\n", `\n`)
}
