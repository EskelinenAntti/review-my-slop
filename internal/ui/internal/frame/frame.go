// Package frame provides the shared screen composition used by UI screens.
package frame

import "strings"

// ScreenWidth returns the minimum width used when rendering screen content.
func ScreenWidth(width int) int { return max(20, width) }

// SingleLine replaces embedded line breaks in screen text with spaces.
func SingleLine(value string) string {
	return strings.ReplaceAll(value, "\n", " ")
}

// BodyHeight returns the available body rows after reserving the header and
// footer.
func BodyHeight(height int) int { return max(1, height-3) }

// EmptyBody creates body rows with a muted message near the top.
func EmptyBody(height int, message string) []string {
	rows := make([]string, BodyHeight(height))
	rows[min(1, len(rows)-1)] = Muted(message)
	return rows
}

// BodyRow maps a terminal cell to a body row, excluding header and footer cells
// and any part of the frame that falls outside the terminal dimensions.
func BodyRow(x, y, width, height int) (int, bool) {
	row := y - 1
	return row, x >= 0 && x < width && row >= 0 &&
		row < BodyHeight(height)
}

// Render composes a header, padded body, footer, and final blank line.
func Render(header string, body []string, footer string, height int) string {
	bodyHeight := BodyHeight(height)
	lines := make([]string, bodyHeight+3)
	lines[0] = header
	copy(lines[1:bodyHeight+1], body)
	lines[bodyHeight+1] = footer
	return strings.Join(lines, "\n")
}
