package diffscreen

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

type highlightedSources struct {
	old []string
	new []string
}

const (
	lightTheme = "catppuccin-latte"
	darkTheme  = "catppuccin-mocha"
)

func render(filename, source, theme string) []string {
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
