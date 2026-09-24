package ui

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

type Pair struct {
	Old, New []string
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
	if err := quick.Highlight(&buf, source, filename, "terminal16m", theme); err == nil {
		if rendered, err := io.ReadAll(&buf); err == nil {
			return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
		}
	}
	return fallback
}
