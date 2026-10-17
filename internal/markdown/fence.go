// Package markdown provides shared Markdown formatting for review text.
package markdown

import "strings"

// CodeFence returns a backtick fence that cannot occur within the given lines.
func CodeFence(lines []string) string {
	return strings.Repeat("`", max(3, longestBacktickRun(lines)+1))
}

// CodeSpan renders inline text with safe backticks and edge spacing.
func CodeSpan(text string) string {
	delimiter := strings.Repeat("`", longestBacktickRun([]string{text})+1)
	backtickAtEdge := strings.HasPrefix(text, "`") ||
		strings.HasSuffix(text, "`")
	spacesAtEdges := strings.HasPrefix(text, " ") &&
		strings.HasSuffix(text, " ")
	if backtickAtEdge || spacesAtEdges && strings.Trim(text, " ") != "" {
		text = " " + text + " "
	}
	return delimiter + text + delimiter
}

func longestBacktickRun(lines []string) int {
	longest := 0
	for _, line := range lines {
		run := 0
		for _, char := range line {
			if char == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
	}
	return longest
}
