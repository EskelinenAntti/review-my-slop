// Package frame provides the shared screen composition used by UI screens.
package frame

import "strings"

func BodyHeight(height int) int { return max(1, height-3) }

func Render(header string, body []string, footer string, height int) string {
	bodyHeight := BodyHeight(height)
	lines := make([]string, 0, bodyHeight+3)
	lines = append(lines, header)
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+1 {
		lines = append(lines, "")
	}
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}
