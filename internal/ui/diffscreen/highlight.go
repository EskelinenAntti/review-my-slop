package diffscreen

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type highlightedSources struct {
	Old []string
	New []string
}

const (
	lightTheme = "catppuccin-latte"
	darkTheme  = "catppuccin-mocha"
)

func (v *patchProjection) highlightFile(file patch.File) highlightedSources {
	theme := lightTheme
	if v.dark {
		theme = darkTheme
	}
	return highlightedSources{
		Old: highlightSource(file.OldPath, file.OldSource, theme),
		New: highlightSource(file.NewPath, file.NewSource, theme),
	}
}

func (v *patchProjection) attachHighlightedSources(
	row *displayRow,
	lines []patch.Line,
	sources highlightedSources,
) {
	if !v.split {
		row.text = highlightedSourceText(sources, lines[row.rightLine])
		return
	}
	if row.leftLine >= 0 {
		row.left = highlightedSourceText(sources, lines[row.leftLine])
	}
	if row.rightLine >= 0 {
		row.right = highlightedSourceText(sources, lines[row.rightLine])
	}
}

func highlightSource(filename, source, theme string) []string {
	if source == "" {
		return nil
	}
	var buf bytes.Buffer
	highlightErr := quick.Highlight(
		&buf,
		source,
		filename,
		"terminal16m",
		theme,
	)
	if highlightErr != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	rendered, err := io.ReadAll(&buf)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}

func highlightedSourceText(sources highlightedSources, line patch.Line) string {
	if line.Kind == patch.Deletion {
		return highlightedSourceLine(sources.Old, line.OldNumber, line.Text)
	}
	return highlightedSourceLine(sources.New, line.NewNumber, line.Text)
}

func highlightedSourceLine(
	lines []string,
	number patch.LineNumber,
	fallback string,
) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
