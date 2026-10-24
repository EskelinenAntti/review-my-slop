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
	oldRevision string,
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
		file, err := parseTrackedFile(ctx, run, root, fd, oldRevision)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

func parseTrackedFile(
	ctx context.Context,
	run runner,
	root string,
	fd *diff.FileDiff,
	oldRevision string,
) (File, error) {
	oldPath := cleanDiffPath(fd.OrigName)
	newPath := cleanDiffPath(fd.NewName)
	file := File{
		OldPath:  oldPath,
		NewPath:  newPath,
		Metadata: visibleStrings(fd.Extended),
	}
	file.DisplayPath = visibleText(file.PreferredPath())
	file.OldSource = readGitSource(ctx, run, root, oldRevision, oldPath)
	file.NewSource = readWorkingTree(root, newPath)
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
		case '+':
			line.Kind = Addition
		case '-':
			line.Kind = Deletion
		case '\\':
			// "\ No newline at end of file" belongs to the preceding line.
			continue
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
		if line.Kind != Addition {
			line.OldNumber = LineNumber(oldLine)
			oldLine++
		}
		if line.Kind != Deletion {
			line.NewNumber = LineNumber(newLine)
			newLine++
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
