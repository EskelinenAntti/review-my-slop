package keys

import (
	"strings"
	"testing"
)

func TestReadNamedKeys(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ctrl-c", "\x03", Q},
		{"tab", "\t", Tab},
		{"ctrl-d", "\x04", CtrlD},
		{"ctrl-u", "\x15", CtrlU},
		{"enter", "\r", Enter},
		{"escape", "\x1b", Esc},
		{"up", "\x1b[A", Up},
		{"down", "\x1b[B", Down},
		{"right", "\x1b[C", Right},
		{"left", "\x1b[D", Left},
		{"page-up", "\x1b[5~", PageUp},
		{"page-down", "\x1b[6~", PageDown},
		{"plain", "j", "j"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Read(strings.NewReader(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Read(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}
