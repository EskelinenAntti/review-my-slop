package diffscreen

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type highlightedSources struct {
	Old []string
	New []string
}

func highlightSource(filename, source string, darkBackground bool) []string {
	if source == "" {
		return nil
	}
	theme := "catppuccin-latte"
	if darkBackground {
		theme = "catppuccin-mocha"
	}
	var buf bytes.Buffer
	if err := quick.Highlight(&buf, source, filename, "terminal16m", theme); err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

func (highlighted highlightedSources) textFor(line patch.Line) string {
	if line.Kind == patch.Deletion {
		return highlightedLine(highlighted.Old, line.OldNumber, line.Text)
	}
	return highlightedLine(highlighted.New, line.NewNumber, line.Text)
}

func highlightFile(file *patch.File, dark bool) highlightedSources {
	return highlightedSources{
		Old: highlightSource(file.OldPath, file.OldSource, dark),
		New: highlightSource(file.NewPath, file.NewSource, dark),
	}
}

func highlightedLine(lines []string, number patch.LineNumber, fallback string) string {
	if number <= 0 || int(number) > len(lines) {
		return fallback
	}
	return lines[int(number)-1]
}
