// Package frame provides the shared screen composition used by UI screens.
package frame

import (
	"strings"
)

// BodyHeight returns the number of rows available for screen content.
func BodyHeight(height int) int { return max(one, height-3) }

// BodyRow maps a terminal cell to a body row, excluding header and footer cells
// and any part of the frame that falls outside the terminal dimensions.
func BodyRow(x, y, width, height int) (int, bool) {
	row := y - one
	return row, y < height-two &&
		(x >= zero && x < width && row >= zero && row < BodyHeight(height))
}

// Render composes the header, body and footer for the terminal.
func Render(header string, body []string, footer string, height int) string {
	bodyHeight := BodyHeight(height)
	lines := make([]string, zero, bodyHeight+3)
	lines = append(lines, header)
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+one {
		lines = append(lines, "")
	}
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}
