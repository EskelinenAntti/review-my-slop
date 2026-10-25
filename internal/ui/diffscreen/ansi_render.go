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

const (
	sgrExtendedForeground = 38
	sgrExtendedBackground = 48
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
			return filterSGR(sequence, colors)
		},
	)
}

func filterSGR(sequence string, colors foregroundPolicy) string {
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
}

func filterSGRParts(parts []string, colors foregroundPolicy) []string {
	filtered := make([]string, 0, len(parts))
	for len(parts) > 0 {
		parameter := parts[0]
		parts = parts[1:]
		code, err := strconv.Atoi(parameter)
		if err != nil {
			filtered = append(filtered, parameter)
			continue
		}
		switch {
		case code == sgrExtendedBackground ||
			(colors == discardForeground && code == sgrExtendedForeground):
			parts = skipColorPayload(parts)
		case isBackgroundSGR(code):
			// Background colors are supplied by the row style.
		case colors == discardForeground && isForegroundSGR(code):
			// Cursor and selection styles also replace syntax foregrounds.
		default:
			filtered = append(filtered, parameter)
		}
	}
	return filtered
}

func isBackgroundSGR(code int) bool {
	return (code >= 40 && code <= 49) || (code >= 100 && code <= 107)
}

func isForegroundSGR(code int) bool {
	return (code >= 30 && code <= 39) || (code >= 90 && code <= 97)
}

func skipColorPayload(parts []string) []string {
	if len(parts) == 0 {
		return parts
	}
	switch parts[0] {
	case "2":
		return parts[min(4, len(parts)):]
	case "5":
		return parts[min(2, len(parts)):]
	default:
		return parts
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
