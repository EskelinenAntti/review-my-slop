package patch

import (
	"fmt"
	"strings"
)

func visibleStrings(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = visibleText(value)
	}
	return result
}

func visibleSource(value string) string {
	return escapeControls(value, "\n")
}

func visibleText(value string) string {
	return escapeControls(value, `\n`)
}

func escapeControls(value, newline string) string {
	var result strings.Builder
	size := len(value)
	if len(newline) > 1 {
		size += strings.Count(value, "\n") * (len(newline) - 1)
	}
	result.Grow(size)
	for _, r := range value {
		switch {
		case r == '\n':
			_, _ = result.WriteString(newline)
		case r == '\t':
			_, _ = result.WriteRune(r)
		case r == '\r':
			_, _ = result.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&result, `\x%02x`, r)
		default:
			_, _ = result.WriteRune(r)
		}
	}
	return result.String()
}
