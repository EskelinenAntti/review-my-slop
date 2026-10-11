package diffscreen

import (
	"bytes"
	"io"
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
	theme := map[bool]string{
		false: "catppuccin-latte",
		true:  "catppuccin-mocha",
	}[darkBackground]
	var buf bytes.Buffer
	highlightErr := quick.Highlight(
		&buf, source, filename, "terminal16m", theme,
	)
	if highlightErr != nil {
		return strings.Split(strings.TrimSuffix(source, newline), newline)
	}
	rendered, err := io.ReadAll(&buf)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, newline), newline)
	}
	return strings.Split(
		strings.TrimSuffix(string(rendered), newline),
		newline,
	)
}
