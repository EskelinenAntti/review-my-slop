package patch

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
)

func parseTrackedFile(fd *diff.FileDiff) (File, error) {
	file := File{
		OldPath:  cleanDiffPath(fd.OrigName),
		NewPath:  cleanDiffPath(fd.NewName),
		Metadata: visibleStrings(fd.Extended),
	}
	file.DisplayPath = visibleText(file.PreferredPath())
	for _, h := range fd.Hunks {
		lines, err := parseHunkBody(h.OrigStartLine, h.NewStartLine, h.Body)
		if err != nil {
			return File{}, fmt.Errorf("%s: %w", file.DisplayPath, err)
		}
		file.Hunks = append(
			file.Hunks,
			Hunk{Header: formatHunkHeader(h), Lines: lines},
		)
	}
	return file, nil
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]Line, error) {
	if len(body) == 0 {
		return []Line{}, nil
	}
	lines := make([]Line, 0)
	for raw := range bytes.SplitSeq(
		bytes.TrimSuffix(body, []byte("\n")),
		[]byte("\n"),
	) {
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		line := Line{Text: visibleText(string(raw[1:]))}
		switch raw[0] {
		case ' ':
			line.Kind = Context
			line.OldNumber = LineNumber(oldLine)
			line.NewNumber = LineNumber(newLine)
			oldLine++
			newLine++
		case '+':
			line.Kind = Addition
			line.NewNumber = LineNumber(newLine)
			newLine++
		case '-':
			line.Kind = Deletion
			line.OldNumber = LineNumber(oldLine)
			oldLine++
		case '\\':
			// "\ No newline at end of file" belongs to the preceding line.
			continue
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func cleanDiffPath(path string) string {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "a/") {
		path = path[2:]
	} else {
		path = strings.TrimPrefix(path, "b/")
	}
	if path == "/dev/null" {
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
		header += " " + h.Section
	}
	return header
}
