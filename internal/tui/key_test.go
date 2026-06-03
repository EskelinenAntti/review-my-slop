package tui

import (
	"strings"
	"testing"
)

func TestReadKeyDecodesNavigation(t *testing.T) {
	tests := map[string]Key{
		"j":       "j",
		"\x1b[A":  KeyUp,
		"\x1b[B":  KeyDown,
		"\x1b[C":  KeyRight,
		"\x1b[D":  KeyLeft,
		"\x04":    KeyCtrlD,
		"\x15":    KeyCtrlU,
		"\x1b[5~": KeyPageUp,
		"\x1b[6~": KeyPageDown,
	}

	for input, want := range tests {
		got, err := ReadKey(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ReadKey(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("ReadKey(%q) = %q, want %q", input, got, want)
		}
	}
}
