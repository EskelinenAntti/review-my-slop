package ui

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

type Pair struct {
	Old []string
	New []string
}

func Sources(filename, oldSource, newSource string, darkBackground bool) Pair {
	return Pair{render(filename, oldSource, darkBackground), render(filename, newSource, darkBackground)}
}

func render(filename, source string, darkBackground bool) []string {
	if source == "" {
		return nil
	}
	fallback := strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	theme := "catppuccin-latte"
	if darkBackground {
		theme = "catppuccin-mocha"
	}
	var buf bytes.Buffer
	if err := quick.Highlight(&buf, source, filename, "terminal16m", theme); err != nil {
		return fallback
	}
	rendered, err := io.ReadAll(&buf)
	if err != nil {
		return fallback
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}
