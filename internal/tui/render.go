package tui

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

func Render(w io.Writer, lines []string, v Viewport, cols int) {
	RenderWithOptions(w, lines, v, cols, RenderOptions{})
}

type RenderOptions struct {
	Sticky StickyLine
}

type StickyLine struct {
	Text   string
	Active bool
}

func RenderWithOptions(w io.Writer, lines []string, v Viewport, cols int, options RenderOptions) {
	cols = max(1, cols)
	fmt.Fprint(w, "\x1b[H\x1b[2J")
	for row := 0; row < v.Rows; row++ {
		if row == 0 && options.Sticky.Active {
			fmt.Fprintf(w, "%s\x1b[K", truncateVisible(options.Sticky.Text, cols))
			if row != v.Rows-1 {
				fmt.Fprint(w, "\r\n")
			}
			continue
		}
		index := v.Top + row
		if options.Sticky.Active {
			index--
		}
		if index >= len(lines) {
			fmt.Fprint(w, "\x1b[K")
		} else if index == v.Cursor {
			fmt.Fprintf(w, "%s\x1b[K", highlightLine(lines[index], cols))
		} else {
			fmt.Fprintf(w, "%s\x1b[K", truncateVisible(lines[index], cols))
		}
		if row != v.Rows-1 {
			fmt.Fprint(w, "\r\n")
		}
	}
}

func highlightLine(s string, cols int) string {
	plain := truncatePlain(StripANSI(s), cols)
	if length := len([]rune(plain)); length < cols {
		plain += strings.Repeat(" ", cols-length)
	}
	return "\x1b[7m" + plain + "\x1b[0m"
}

func StripANSI(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			if end := ansiEnd(s, i); end > i {
				i = end
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		out.WriteRune(r)
		i += size
	}
	return out.String()
}

func truncateVisible(s string, cols int) string {
	var out strings.Builder
	visible := 0
	for i := 0; i < len(s) && visible < cols; {
		if s[i] == '\x1b' {
			if end := ansiEnd(s, i); end > i {
				out.WriteString(s[i:end])
				i = end
				continue
			}
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			i++
			continue
		}
		out.WriteRune(r)
		visible++
		i += size
	}
	return out.String()
}

func truncatePlain(s string, cols int) string {
	var out strings.Builder
	visible := 0
	for _, r := range s {
		if visible >= cols {
			break
		}
		out.WriteRune(r)
		visible++
	}
	return out.String()
}

func ansiEnd(s string, start int) int {
	if start+1 >= len(s) || s[start] != '\x1b' || s[start+1] != '[' {
		return -1
	}
	for i := start + 2; i < len(s); i++ {
		if s[i] >= '@' && s[i] <= '~' {
			return i + 1
		}
	}
	return -1
}
