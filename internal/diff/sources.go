package diff

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxFileBytes = 2 << 20

func (l Loader) loadUntracked(ctx context.Context, root string) ([]File, error) {
	output, err := l.gitRunner().Run(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	var files []File
	for rawPath := range bytes.SplitSeq(output, []byte{0}) {
		if len(rawPath) == 0 {
			continue
		}
		path := string(rawPath)
		displayPath := visibleText(path)
		fullPath := filepath.Join(root, filepath.FromSlash(path))
		info, statErr := os.Lstat(fullPath)
		if statErr != nil {
			return nil, fmt.Errorf("stat untracked %q: %w", path, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, readErr := os.Readlink(fullPath)
			if readErr != nil {
				return nil, fmt.Errorf("read symlink %q: %w", path, readErr)
			}
			files = append(files, addedFile(path, visibleText(target)))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if info.Size() > maxFileBytes {
			files = append(files, File{
				NewPath:     path,
				DisplayPath: displayPath,
				Metadata:    []string{"untracked file", "content omitted: file exceeds 2 MiB"},
			})
			continue
		}
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			return nil, fmt.Errorf("read untracked %q: %w", path, readErr)
		}
		if bytes.IndexByte(content, 0) >= 0 {
			files = append(files, File{
				NewPath:     path,
				DisplayPath: displayPath,
				Metadata:    []string{"untracked binary file"},
			})
			continue
		}
		files = append(files, addedFile(path, visibleSource(string(content))))
	}
	return files, nil
}

func addedFile(path, content string) File {
	sourceLines := splitSourceLines(content)
	lines := make([]Line, 0, len(sourceLines))
	for index, line := range sourceLines {
		lines = append(lines, Line{Kind: Addition, Text: line, NewNumber: LineNumber(index + 1)})
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

func readIndex(ctx context.Context, runner Runner, root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	output, err := runner.Run(ctx, root, "show", ":"+path)
	if err != nil || len(output) > maxFileBytes || bytes.IndexByte(output, 0) >= 0 {
		return ""
	}
	return visibleSource(string(output))
}

func readRevision(ctx context.Context, runner Runner, root, revision, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	output, err := runner.Run(ctx, root, "show", revision+":"+path)
	if err != nil || len(output) > maxFileBytes || bytes.IndexByte(output, 0) >= 0 {
		return ""
	}
	return visibleSource(string(output))
}

func readWorkingTree(root, path string) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(fullPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return ""
	}
	output, err := os.ReadFile(fullPath)
	if err != nil || bytes.IndexByte(output, 0) >= 0 {
		return ""
	}
	return visibleSource(string(output))
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
	for _, char := range value {
		switch {
		case char == '\n' || char == '\t':
			result.WriteRune(char)
		case char == '\r':
			result.WriteString(`\r`)
		case char < 0x20 || char == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, char)
		default:
			result.WriteRune(char)
		}
	}
	return result.String()
}

func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), "\n", `\n`)
}
