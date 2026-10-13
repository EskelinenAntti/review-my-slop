package patch

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/sourcegraph/go-diff/diff"
)

func parseUnifiedHunkBody(oldLine, newLine int32, body []byte) ([]Line, error) {
	rawLines := bytes.Split(bytes.TrimSuffix(body, []byte("\n")), []byte("\n"))
	if len(body) == 0 {
		return []Line{}, nil
	}
	lines := make([]Line, 0, len(rawLines))
	for _, raw := range rawLines {
		if len(raw) == 0 {
			return nil, errors.New("malformed empty diff line")
		}
		text := escapeDisplayText(string(raw[1:]))
		switch raw[0] {
		case ' ':
			lines = append(lines, Line{
				Kind: Context, Text: text,
				OldNumber: LineNumber(oldLine), NewNumber: LineNumber(newLine),
			})
			oldLine++
			newLine++
		case '+':
			lines = append(lines, Line{
				Kind: Addition, Text: text, NewNumber: LineNumber(newLine),
			})
			newLine++
		case '-':
			lines = append(lines, Line{
				Kind: Deletion, Text: text, OldNumber: LineNumber(oldLine),
			})
			oldLine++
		case '\\':
			// "\\ No newline at end of file" belongs to the preceding line.
		default:
			return nil, fmt.Errorf("unexpected diff prefix %q", raw[0])
		}
	}
	return lines, nil
}

func formatUnifiedHunkHeader(hunk *diff.Hunk) string {
	header := fmt.Sprintf(
		"@@ -%d,%d +%d,%d @@",
		hunk.OrigStartLine,
		hunk.OrigLines,
		hunk.NewStartLine,
		hunk.NewLines,
	)
	if hunk.Section != "" {
		header += " " + hunk.Section
	}
	return header
}
