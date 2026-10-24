package diffscreen

import (
	"iter"
	"strconv"
	"strings"
)

// sgrParameters yields SGR commands with their arguments intact. Color
// components are data, even when they resemble SGR commands. Empty or invalid
// command codes are yielded as -1 so callers can preserve their original text.
func sgrParameters(parameters string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		parts := strings.Split(parameters, ";")
		for index := 0; index < len(parts); {
			code, err := strconv.Atoi(parts[index])
			if err != nil {
				code = -1
			}
			end := index + 1
			if code == 38 || code == 48 {
				end = colorParameterEnd(parts, index)
			}
			if !yield(code, strings.Join(parts[index:end], ";")) {
				return
			}
			index = end
		}
	}
}

func colorParameterEnd(parts []string, index int) int {
	if index+1 >= len(parts) {
		return index + 1
	}
	switch parts[index+1] {
	case "2":
		return min(index+5, len(parts))
	case "5":
		return min(index+3, len(parts))
	default:
		return index + 1
	}
}
