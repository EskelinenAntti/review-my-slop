package diffscreen

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
)

type pair struct {
	Old []string
	New []string
}

func render(filename, source string, darkBackground bool) []string {
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
