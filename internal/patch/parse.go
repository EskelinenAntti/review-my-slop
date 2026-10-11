package patch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

func parseTracked(
	ctx context.Context,
	run runner,
	root string,
	raw []byte,
	readOld sourceReader,
) (
	[]File,
	error,
) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := diff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, 0, len(parsed))
	for _, fd := range parsed {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file, err := trackedFile(ctx, run, root, fd, readOld)
		if err != nil {
			return nil, err
		}

		files = append(files, file)
	}
	return files, nil
}

func parseHunkBody(initialOldLine, initialNewLine int32, body []byte) (
	[]Line,
	error,
) {
	oldLine, newLine := initialOldLine, initialNewLine
	rawLines := bytes.Split(body, []byte(newline))
	lines := make([]Line, 0, len(rawLines))
	for i, raw := range rawLines {
		if isTrailingEmptyLine(i, raw, len(rawLines)) {
			continue
		}
		line, include, err := parseDiffLine(raw, &oldLine, &newLine)
		if err != nil {
			return nil, err
		}
		if include {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func parseDiffLine(raw []byte, oldLine, newLine *int32) (Line, bool, error) {
	if len(raw) == 0 {
		return Line{}, false, errors.New("malformed empty diff line")
	}
	line := Line{Text: visibleText(string(raw[1:]))}
	switch raw[0] {
	case ' ':
		line.Kind, line.OldNumber, line.NewNumber = Context, LineNumber(
			*oldLine,
		), LineNumber(*newLine)
		*oldLine++
		*newLine++
	case '+':
		line.Kind, line.NewNumber = Addition, LineNumber(*newLine)
		*newLine++
	case '-':
		line.Kind, line.OldNumber = Deletion, LineNumber(*oldLine)
		*oldLine++
	case '\\':
		// The no-newline marker belongs to the preceding line.
		return Line{}, false, nil
	default:
		return Line{}, false, fmt.Errorf(
			"unexpected diff prefix %q",
			raw[0],
		)
	}
	return line, true, nil
}

func cleanDiffPath(initialPath string) string {
	path := initialPath

	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == nullPath {
		return ""
	}
	return path
}

func formatHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf(
		"@@ -%d,%d +%d,%d @@",
		h.OrigStartLine,
		h.OrigLines,
		h.NewStartLine,
		h.NewLines,
	)
	if h.Section != "" {
		header += space + h.Section
	}
	return header
}

func trackedFile(
	ctx context.Context,
	run runner,
	root string,
	fd *diff.FileDiff,
	readOld sourceReader,
) (
	File,
	error,
) {
	oldPath := cleanDiffPath(fd.OrigName)
	newPath := cleanDiffPath(fd.NewName)
	display := newPath
	if display == "" || display == nullPath {
		display = oldPath
	}
	file := File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: visibleText(display),
		Metadata:    visibleStrings(fd.Extended),
	}
	file.OldSource = readOld(ctx, run, root, oldPath)
	file.NewSource = readWorkingTree(root, newPath)
	for _, h := range fd.Hunks {
		lines, parseErr := parseHunkBody(
			h.OrigStartLine,
			h.NewStartLine,
			h.Body,
		)
		if parseErr != nil {
			return File{}, fmt.Errorf("%s: %w", display, parseErr)
		}
		file.Hunks = append(file.Hunks, Hunk{
			Header: formatHunkHeader(h),
			Lines:  lines,
		})
	}
	return file, nil
}

func isTrailingEmptyLine(index int, raw []byte, count int) bool {
	return index == count-1 && len(raw) == 0
}
