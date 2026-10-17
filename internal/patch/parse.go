package patch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

func diffHunkHeader(h *diff.Hunk) string {
	header := fmt.Sprintf(
		"@@ -%d,%d +%d,%d @@",
		h.OrigStartLine,
		h.OrigLines,
		h.NewStartLine,
		h.NewLines,
	)
	if h.Section != "" {
		header += " " + h.Section
	}
	return header
}

func parseDiffHunkLines(h *diff.Hunk) ([]Line, error) {
	oldLine, newLine := h.OrigStartLine, h.NewStartLine
	if len(h.Body) == 0 {
		return []Line{}, nil
	}
	rawLines := bytes.Split(
		bytes.TrimSuffix(h.Body, []byte("\n")),
		[]byte("\n"),
	)
	lines := make([]Line, 0, len(rawLines))
	for _, raw := range rawLines {
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := escapeDiffLine(string(raw[1:]))
		switch raw[0] {
		case ' ':
			lines = append(lines, Line{
				Kind: Context, Text: text,
				OldNumber: LineNumber(oldLine),
				NewNumber: LineNumber(newLine),
			})
			oldLine++
			newLine++
		case '+':
			lines = append(lines, Line{
				Kind: Addition, Text: text, NewNumber: LineNumber(
					newLine,
				),
			})
			newLine++
		case '-':
			lines = append(lines, Line{
				Kind: Deletion, Text: text, OldNumber: LineNumber(
					oldLine,
				),
			})
			oldLine++
		case '\\':
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func escapeDiffLine(text string) string {
	var escaped strings.Builder
	for _, r := range text {
		switch {
		case r == '\n' || r == '\t':
			_, _ = escaped.WriteRune(r)
		case r == '\r':
			_, _ = escaped.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&escaped, `\x%02x`, r)
		default:
			_, _ = escaped.WriteRune(r)
		}
	}
	return strings.ReplaceAll(escaped.String(), "\n", `\n`)
}

func parsePatchDiff(
	ctx context.Context,
	raw []byte,
	readOld sourceReader,
	root string,

) ([]File, error) {
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
		file, err := parseOneDiffFile(ctx, fd, readOld, root)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func normalizeDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func escapeDiffMetadata(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = escapeDiffDisplayValue(value)
	}
	return result
}

func escapeDiffDisplayValue(value string) string {
	return strings.ReplaceAll(escapeDisplayText(value), "\n", `\n`)
}

func parseOneDiffFile(
	ctx context.Context,
	fd *diff.FileDiff,
	readOld sourceReader,
	root string,

) (File, error) {
	oldPath := normalizeDiffPath(fd.OrigName)
	newPath := normalizeDiffPath(fd.NewName)
	display := newPath
	if display == "" {
		display = oldPath
	}
	file := File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: escapeDiffDisplayValue(display),
		Metadata:    escapeDiffMetadata(fd.Extended),
	}
	file.OldSource = readOld(ctx, execRunner{}, root, oldPath)
	file.NewSource = readPatchFileSource(newPath, root)
	for _, h := range fd.Hunks {
		lines, err := parseOneDiffHunk(h)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(
			file.Hunks,
			Hunk{Header: diffHunkHeader(h), Lines: lines},
		)
	}
	return file, nil
}

func parseOneDiffHunk(h *diff.Hunk,

) ([]Line, error) {
	if len(h.Body) == 0 {
		return []Line{}, nil
	}
	rawBody := bytes.TrimSuffix(h.Body, []byte("\n"))
	rawLines := bytes.Split(rawBody, []byte("\n"))
	lines := make([]Line, 0, len(rawLines))
	oldLine := LineNumber(h.OrigStartLine)
	newLine := LineNumber(h.NewStartLine)
	for _, raw := range rawLines {
		line, include, err := parseDiffLine(raw, oldLine, newLine)
		if err != nil {
			return nil, err
		}
		if !include {
			continue
		}
		lines = append(lines, line)
		oldLine, newLine = advancePatchLineNumbers(line.Kind, oldLine, newLine)
	}
	return lines, nil
}

func advancePatchLineNumbers(
	kind LineKind,
	oldLine, newLine LineNumber,
) (nextOldLine, nextNewLine LineNumber) {
	switch kind {
	case Context:
		return oldLine + 1, newLine + 1
	case Addition:
		return oldLine, newLine + 1
	case Deletion:
		return oldLine + 1, newLine
	default:
		return oldLine, newLine
	}
}

func parseDiffLine(
	raw []byte,
	oldLine, newLine LineNumber,
) (Line, bool, error) {
	if len(raw) == 0 {
		return Line{}, false, errors.New("malformed empty diff line")
	}
	line := Line{Text: escapeDiffDisplayValue(string(raw[1:]))}
	switch raw[0] {
	case ' ':
		line.Kind = Context
		line.OldNumber, line.NewNumber = oldLine, newLine
	case '+':
		line.Kind, line.NewNumber = Addition, newLine
	case '-':
		line.Kind, line.OldNumber = Deletion, oldLine
	case '\\':
		return Line{}, false, nil
	default:
		return Line{}, false, fmt.Errorf(
			"unexpected diff prefix %q",
			raw[0],
		)
	}
	return line, true, nil
}
