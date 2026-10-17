package diffscreen

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/quick"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

type highlightedFile struct {
	Old []string
	New []string
}

const (
	lightTheme = "catppuccin-latte"
	darkTheme  = "catppuccin-mocha"
)

func (v *diffView) highlightFile(file *patch.File) highlightedFile {
	theme := lightTheme
	if v.dark {
		theme = darkTheme
	}
	return highlightedFile{
		Old: highlightSource(file.OldPath, file.OldSource, theme),
		New: highlightSource(file.NewPath, file.NewSource, theme),
	}
}

func highlightSource(filename, source, theme string) []string {
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
		return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

func (file highlightedFile) lineText(line patch.Line) string {
	lines, number := file.New, line.NewNumber
	if line.Kind == patch.Deletion {
		lines, number = file.Old, line.OldNumber
	}
	if number <= 0 || int(number) > len(lines) {
		return line.Text
	}
	return lines[int(number)-1]
}
