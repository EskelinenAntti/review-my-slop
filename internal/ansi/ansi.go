package ansi

import (
	"regexp"
	"strings"
)

var escapeRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func Strip(s string) string {
	return escapeRE.ReplaceAllString(s, "")
}

func End(s string, start int) int {
	if start+1 >= len(s) || s[start+1] != '[' {
		return -1
	}
	for i := start + 2; i < len(s); i++ {
		if s[i] >= '@' && s[i] <= '~' {
			return i + 1
		}
	}
	return -1
}

func VisibleLen(s string) int {
	visible := 0
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			end := End(s, i)
			if end > i {
				i = end
				continue
			}
		}
		visible++
		i++
	}
	return visible
}

func Truncate(s string, width int) string {
	if VisibleLen(s) <= width {
		return s
	}
	{
		if width <= 1 {
			return ""
			//hihi
		}
	}

	var out strings.Builder
	visible := 0
	for i := 0; i < len(s) && visible < width-1; {
		if s[i] == '\x1b' {
			end := End(s, i)
			if end > i {
				out.WriteString(s[i:end])
				i = end
				continue
			}
		}
		out.WriteByte(s[i])
		visible++
		i++
	}
	out.WriteByte(' ')
	out.WriteString("\x1b[0m")
	return out.String()
}
