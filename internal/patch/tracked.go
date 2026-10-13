package patch

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/sourcegraph/go-diff/diff"
)

func (r gitRepository) parseTracked(
	ctx context.Context,
	raw []byte,
	readOld sourceReader,
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
		file, err := r.parseTrackedFile(ctx, fd, readOld)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func (r gitRepository) parseTrackedFile(
	ctx context.Context,
	fd *diff.FileDiff,
	readOld sourceReader,
) (File, error) {
	oldPath := cleanDiffPath(fd.OrigName)
	newPath := cleanDiffPath(fd.NewName)
	display := newPath
	if display == "" || display == "/dev/null" {
		display = oldPath
	}
	file := File{
		OldPath:     oldPath,
		NewPath:     newPath,
		DisplayPath: visibleText(display),
		Metadata:    visibleStrings(fd.Extended),
	}
	file.OldSource = readOld(ctx, oldPath)
	file.NewSource = readWorkingTree(r.root, newPath)
	for _, h := range fd.Hunks {
		lines, err := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", display, err)
		}
		file.Hunks = append(
			file.Hunks,
			Hunk{Header: formatHunkHeader(h), Lines: lines},
		)
	}
	return file, nil
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]Line, error) {
	rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
	if len(body) == 0 {
		return []Line{}, nil
	}
	lines := make([]Line, 0, len(rawLines))
	for _, raw := range rawLines {
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := visibleText(string(raw[1:]))
		switch raw[0] {
		case ' ':
			lines = append(
				lines,
				Line{
					Kind:      Context,
					Text:      text,
					OldNumber: LineNumber(oldLine),
					NewNumber: LineNumber(newLine),
				},
			)
			oldLine++
			newLine++
		case '+':
			lines = append(
				lines,
				Line{
					Kind:      Addition,
					Text:      text,
					NewNumber: LineNumber(newLine),
				},
			)
			newLine++
		case '-':
			lines = append(
				lines,
				Line{
					Kind:      Deletion,
					Text:      text,
					OldNumber: LineNumber(oldLine),
				},
			)
			oldLine++
		case '\\':
			// "\ No newline at end of file" belongs to the preceding line.
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
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
		header += " " + h.Section
	}
	return header
}
