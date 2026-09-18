package ui

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

func highlightSources(filename, oldSource, newSource string, dark bool) highlightedSource {
	return highlightedSource{
		old: highlightSource(filename, oldSource, dark),
		new: highlightSource(filename, newSource, dark),
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
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	rendered, err := io.ReadAll(&buffer)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}
