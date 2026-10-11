// Package frame provides the shared screen composition used by UI screens.
package frame

import (
	"strings"
)

// BodyHeight returns the number of rows available for screen content.
func BodyHeight(height int) int {
	return max(1, height-headerRows-footerRows)
}

// BodyRow maps a terminal cell to a body row, excluding header and footer cells
// and any part of the frame that falls outside the terminal dimensions.
func BodyRow(x, y, width, height int) (int, bool) {
	row := y - headerRows
	return row, y < height-footerRows &&
		(x >= 0 && x < width && row >= 0 && row < BodyHeight(height))
}

// Render composes the header, body and footer for the terminal.
func Render(header string, body []string, footer string, height int) string {
	bodyHeight := BodyHeight(height)
	lines := make([]string, 0, bodyHeight+headerRows+footerRows)
	lines = append(lines, header)
	lines = append(lines, body[:min(len(body), bodyHeight)]...)
	for len(lines) < bodyHeight+headerRows {
		lines = append(lines, "")
	}
	lines = append(lines, footer, "")
	return strings.Join(lines, "\n")
}
