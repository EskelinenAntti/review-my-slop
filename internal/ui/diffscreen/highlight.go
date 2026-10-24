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

func newHighlightedSources(
	file patch.File,
	options Options,
) highlightedSources {
	theme := lightTheme
	if options.Dark {
		theme = darkTheme
	}
	return highlightedSources{
		old: highlightSource(file.OldPath, file.OldSource, theme),
		new: highlightSource(file.NewPath, file.NewSource, theme),
	}
}

func (sources highlightedSources) text(line patch.Line) string {
	if line.Kind == patch.Deletion {
		return highlightedLine(sources.old, line.OldNumber, line.Text)
	}
	return highlightedLine(sources.new, line.NewNumber, line.Text)
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
