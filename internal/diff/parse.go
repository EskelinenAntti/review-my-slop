package diff

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	gitdiff "github.com/sourcegraph/go-diff/diff"
)

func parseTracked(ctx context.Context, runner Runner, root string, raw []byte, readOld sourceReader) ([]File, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, nil
	}
	parsed, err := gitdiff.ParseMultiFileDiff(raw)
	if err != nil {
		return nil, fmt.Errorf("parse git diff: %w", err)
	}
	files := make([]File, 0, len(parsed))
	for _, fileDiff := range parsed {
		oldPath := cleanDiffPath(fileDiff.OrigName)
		newPath := cleanDiffPath(fileDiff.NewName)
		displayPath := newPath
		if displayPath == "" || displayPath == "/dev/null" {
			displayPath = oldPath
		}
		file := File{
			OldPath:     oldPath,
			NewPath:     newPath,
			DisplayPath: visibleText(displayPath),
			Metadata:    visibleStrings(fileDiff.Extended),
		}
		file.OldSource = readOld(ctx, runner, root, oldPath)
		file.NewSource = readWorkingTree(root, newPath)
		for _, hunk := range fileDiff.Hunks {
			lines, parseErr := parseHunkBody(hunk.OrigStartLine, hunk.NewStartLine, hunk.Body)
			if parseErr != nil {
				return nil, fmt.Errorf("%s: %w", displayPath, parseErr)
			}
			file.Hunks = append(file.Hunks, Hunk{
				Header: formatHunkHeader(hunk),
				Lines:  lines,
			})
		}
		files = append(files, file)
	}
	return files, nil
}

func parseHunkBody(oldLine, newLine int32, body []byte) ([]Line, error) {
	rawLines := bytes.Split(body, []byte("\n"))
	lines := make([]Line, 0, len(rawLines))
	for index, raw := range rawLines {
		if index == len(rawLines)-1 && len(raw) == 0 {
			continue
		}
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := visibleText(string(raw[1:]))
		switch raw[0] {
		case ' ':
			lines = append(lines, Line{Kind: Context, Text: text, OldNumber: LineNumber(oldLine), NewNumber: LineNumber(newLine)})
			oldLine++
			newLine++
		case '+':
			lines = append(lines, Line{Kind: Addition, Text: text, NewNumber: LineNumber(newLine)})
			newLine++
		case '-':
			lines = append(lines, Line{Kind: Deletion, Text: text, OldNumber: LineNumber(oldLine)})
			oldLine++
		case '\\':
			// "\\ No newline at end of file" belongs to the preceding line.
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func cleanDiffPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "a/")
	path = strings.TrimPrefix(path, "b/")
	if path == "/dev/null" {
		return ""
	}
	return path
}

func formatHunkHeader(hunk *gitdiff.Hunk) string {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", hunk.OrigStartLine, hunk.OrigLines, hunk.NewStartLine, hunk.NewLines)
	if hunk.Section != "" {
		header += " " + hunk.Section
	}
	return header
}
