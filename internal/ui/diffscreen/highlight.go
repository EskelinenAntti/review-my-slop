package diffscreen

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type highlightedSources struct {
	old []string
	new []string
}

const (
	lightTheme = "catppuccin-latte"
	darkTheme  = "catppuccin-mocha"
)

func (v *diffView) highlight(file *patch.File) highlightedSources {
	theme := lightTheme
	if v.dark {
		theme = darkTheme
	}
	return highlightedSources{
		old: highlightSource(file.OldPath, file.OldSource, theme),
		new: highlightSource(file.NewPath, file.NewSource, theme),
	}
}

func sourceText(highlighted highlightedSources, line patch.Line) string {
	if line.Kind == patch.Deletion {
		return highlightedLine(highlighted.old, line.OldNumber, line.Text)
	}
	return highlightedLine(highlighted.new, line.NewNumber, line.Text)
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
		return splitLines(source)
	}
	return splitLines(buf.String())
}

func splitLines(source string) []string {
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}

func highlightedLine(
	lines []string,
	number patch.LineNumber,
	fallback string,
) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
