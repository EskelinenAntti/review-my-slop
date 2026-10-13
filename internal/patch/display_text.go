package patch

import (
	"fmt"
	"strings"
)

func escapeDisplayTexts(values []string) []string {
	result := make([]string, len(values))
	for index, value := range values {
		result[index] = escapeDisplayText(value)
	}
	return result
}

func escapeDisplayText(value string) string {
	return strings.ReplaceAll(escapeSourceText(value), "\n", `\n`)
}

func escapeSourceText(value string) string {
	var result strings.Builder
	for _, r := range value {
		switch {
		case r == '\n' || r == '\t':
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
