package patch

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readIndexSource(
	ctx context.Context,
	run runner,
	root, path string,
) string {
	if path == "" || path == "/dev/null" {
		return ""
	}
	out, err := run.Run(ctx, root, "show", ":"+path)
	if err != nil || len(out) > maxFileBytes ||
		bytes.IndexByte(out, 0) >= 0 {
		return ""
	}
	return escapeSourceText(string(out))
}

func escapeSourceText(source string) string {
	var result strings.Builder
	for _, r := range source {
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

func loadOneUntrackedFile(path string, root string,

) (File, bool, error) {
	full := filepath.Join(root, filepath.FromSlash(path))
	info, err := os.Lstat(full)
	if err != nil {
		return File{}, false, fmt.Errorf(
			"stat untracked %q: %w",
			path,
			err,
		)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return File{}, false, fmt.Errorf(
				"read symlink %q: %w",
				path,
				err,
			)
		}
		return makeUntrackedSymlinkFile(path, target), true, nil
	}
	if !info.Mode().IsRegular() {
		return File{}, false, nil
	}
	return loadRegularUntrackedFile(path, full, info)
}

func loadRegularUntrackedFile(
	path, full string,
	info os.FileInfo,
) (File, bool, error) {
	if info.Size() > maxFileBytes {
		return omittedUntrackedFile(path), true, nil
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return File{}, false, fmt.Errorf(
			"read untracked %q: %w",
			path,
			err,
		)
	}
	if bytes.IndexByte(content, 0) >= 0 {
		return untrackedBinaryFile(path), true, nil
	}
	return makeUntrackedTextFile(content, path), true, nil
}

func omittedUntrackedFile(path string) File {
	file := untrackedMetadataFile(path, "untracked file")
	file.Metadata = append(file.Metadata, "content omitted: file exceeds 2 MiB")
	return file
}

func untrackedBinaryFile(path string) File {
	return untrackedMetadataFile(path, "untracked binary file")
}

func untrackedMetadataFile(path, description string) File {
	return File{
		NewPath:     path,
		DisplayPath: strings.ReplaceAll(escapeDisplayText(path), "\n", `\n`),
		Metadata:    []string{description},
	}
}

func makeUntrackedSymlinkFile(path string, target string,

) File {
	content := strings.ReplaceAll(escapeSourceText(target), "\n", `\n`)
	return makeUntrackedSourceFile(path, content)
}

func makeUntrackedTextFile(raw []byte, path string,

) File {
	return makeUntrackedSourceFile(path, escapeSourceText(string(raw)))
}

func makeUntrackedSourceFile(path, content string) File {
	lines := untrackedAdditionLines(content)
	return File{
		NewPath:     path,
		DisplayPath: strings.ReplaceAll(escapeDisplayText(path), "\n", `\n`),
		NewSource:   content,
		Metadata:    []string{"untracked file"},
		Hunks: []Hunk{{
			Header: fmt.Sprintf("@@ -0,0 +1,%d @@", len(lines)),
			Lines:  lines,
		}},
	}
}

func untrackedAdditionLines(content string) []Line {
	content = strings.TrimSuffix(content, "\n")
	if content == "" {
		return nil
	}
	sourceLines := strings.Split(content, "\n")
	lines := make([]Line, 0, len(sourceLines))
	for index, line := range sourceLines {
		lines = append(lines, Line{
			Kind: Addition, Text: line,
			NewNumber: LineNumber(index + 1),
		})
	}
	return lines
}

func readPatchFileSource(newPath string, root string,

) string {
	if !validPatchSourcePath(newPath) {
		return ""
	}
	full := filepath.Join(root, filepath.FromSlash(newPath))
	out, ok := readRegularPatchSource(full)
	if !ok {
		return ""
	}
	return escapeSourceText(string(out))
}

func validPatchSourcePath(path string) bool {
	return path != "" && path != "/dev/null"
}

func readRegularPatchSource(path string) ([]byte, bool) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() ||
		info.Size() > maxFileBytes {
		return nil, false
	}
	out, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(out, 0) >= 0 {
		return nil, false
	}
	return out, true
}

func escapeDisplayText(path string,

) string {
	var result strings.Builder
	for _, r := range path {
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
