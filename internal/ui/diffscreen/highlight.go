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
	if quick.Highlight(&buf, source, filename, "terminal16m", theme) == nil {
		source = buf.String()
	}
	return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
}
