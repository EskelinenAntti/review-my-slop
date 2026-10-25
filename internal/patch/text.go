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

// visibleSource makes terminal control bytes visible while retaining source
// layout. In particular, an ESC byte must never reach the renderer unchanged.
func visibleSource(value string) string {
	var result strings.Builder
	result.Grow(len(value))
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

// visibleText also escapes newlines so paths and metadata stay on one line.
func visibleText(value string) string {
	return strings.ReplaceAll(visibleSource(value), "\n", `\n`)
}
