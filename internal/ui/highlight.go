package ui

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

type highlightedSources struct {
	Old []string
	New []string
}

func highlightSources(filename, oldSource, newSource string, dark bool) highlightedSources {
	return highlightedSources{
		Old: highlightSource(filename, oldSource, dark),
		New: highlightSource(filename, newSource, dark),
	}
}

func highlightSource(filename, source string, dark bool) []string {
	if source == "" {
		return nil
	}
	theme := "catppuccin-latte"
	if dark {
		theme = "catppuccin-mocha"
	}
	var buffer bytes.Buffer
	if err := quick.Highlight(&buffer, source, filename, "terminal16m", theme); err != nil {
		return sourceLines(source)
	}
	rendered, err := io.ReadAll(&buffer)
	if err != nil {
		return sourceLines(source)
	}
	return sourceLines(string(rendered))
}

func sourceLines(source string) []string {
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}
