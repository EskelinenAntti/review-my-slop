package diffscreen

import (
	"bytes"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

type sourceHighlights struct {
	oldLines []string
	newLines []string
}

const (
	lightTheme = "catppuccin-latte"
	darkTheme  = "catppuccin-mocha"
)

func highlightSourceLines(filename, source, themeName string) []string {
	if source == "" {
		return nil
	}
	var buf bytes.Buffer
	highlightErr := quick.Highlight(
		&buf,
		source,
		filename,
		"terminal16m",
		themeName,
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
