package highlight

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

func Sources(filename, oldSource, newSource string) Pair {
	return Pair{
		Old: render(filename, oldSource),
		New: render(filename, newSource),
	}
}

func render(filename, source string) []string {
	if source == "" {
		return nil
	}
	var buf bytes.Buffer
	if err := quick.Highlight(&buf, source, filename, "terminal16m", "gruvbox"); err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	rendered, err := io.ReadAll(&buf)
	if err != nil {
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
}
