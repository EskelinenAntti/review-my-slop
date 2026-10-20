package diffscreen

import (
	"regexp"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type foregroundPolicy uint8

const (
	preserveForeground foregroundPolicy = iota
	discardForeground
)

func renderStyledRow(
	style lipgloss.Style,
	value string,
	width int,
	colors foregroundPolicy,
) string {
	value = filterANSIColors(value, colors)
	fitted := fitANSIWindow(value, 0, width)
	prefix := stylePrefix(style)
	if prefix != "" {
		fitted = strings.ReplaceAll(fitted, "\x1b[0m", "\x1b[0m"+prefix)
		fitted = strings.ReplaceAll(fitted, "\x1b[m", "\x1b[m"+prefix)
	}
	return style.Render(fitted)
}

func filterANSIColors(value string, colors foregroundPolicy) string {
	return ansiSGRPattern.ReplaceAllStringFunc(
		value, func(sequence string) string {
			parameters := sequence[2 : len(sequence)-1]
			if parameters == "" {
				return sequence
			}
			parts := strings.Split(parameters, ";")
			filtered := filterSGRParts(parts, colors)
			if len(filtered) == 0 {
				return ""
			}
			return "\x1b[" + strings.Join(filtered, ";") + "m"
		},
	)
}

func filterSGRParts(parts []string, colors foregroundPolicy) []string {
	filtered := parts[:0]
	for index := 0; index < len(parts); index++ {
		code, err := strconv.Atoi(parts[index])
		if err != nil || !colorRemoved(code, colors) {
			filtered = append(filtered, parts[index])
			continue
		}
		if code == 38 || code == 48 {
			index = colorParameterEnd(parts, index)
		}
	}
	return filtered
}

func colorRemoved(code int, colors foregroundPolicy) bool {
	switch {
	case code >= 40 && code <= 49, code >= 100 && code <= 107:
		return true
	case code >= 30 && code <= 39, code >= 90 && code <= 97:
		return colors == discardForeground
	default:
		return false
	}
}

func colorParameterEnd(parts []string, index int) int {
	if index+1 >= len(parts) {
		return index
	}
	switch parts[index+1] {
	case "2":
		return min(index+4, len(parts)-1)
	case "5":
		return min(index+2, len(parts)-1)
	default:
		return index
	}
}

func fitANSIWindow(value string, offset, width int) string {
	if width <= 0 {
		return ""
	}
	value = expandTabs(value)
	if offset > 0 {
		value = ansi.TruncateLeft(value, offset, "")
	}
	value = ansi.Truncate(value, width, "")
	if padding := width - lipgloss.Width(value); padding > 0 {
		value += strings.Repeat(" ", padding)
	}
	return value
}

func expandTabs(value string) string {
	return strings.ReplaceAll(value, "\t", "    ")
}

func stylePrefix(style lipgloss.Style) string {
	const marker = "\x00"
	rendered := style.Render(marker)
	index := strings.Index(rendered, marker)
	if index < 0 {
		return ""
	}
	return rendered[:index]
}

var ansiSGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)
