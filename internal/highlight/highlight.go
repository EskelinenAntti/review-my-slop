package highlight

import (
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

type FileSources struct {
	Old []string
	New []string
}

func Highlight(filename, oldSource, newSource string, darkBackground bool) FileSources {
	return FileSources{
		Old: highlightSource(filename, oldSource, darkBackground),
		New: highlightSource(filename, newSource, darkBackground),
	}
}

func highlightSource(filename, source string, darkBackground bool) []string {
	if source == "" {
		return nil
	}
	theme := "catppuccin-latte"
	if darkBackground {
		theme = "catppuccin-mocha"
	}
	var rendered strings.Builder
	if err := quick.Highlight(&rendered, source, filename, "terminal16m", theme); err != nil {
		return sourceLines(source)
	}
	return sourceLines(rendered.String())
}

func sourceLines(source string) []string {
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}
