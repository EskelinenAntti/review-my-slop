package highlight

import (
	"strings"
	"testing"
)

func TestLicenseHighlightFixture(t *testing.T) {
	lines := render("LICENSE", `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`)
	if len(lines) != 5 {
		t.Fatalf("highlighted lines = %d, want 5", len(lines))
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, `Copyright (c) 2026`) ||
		!strings.Contains(plain, `"AS IS"`) {
		t.Fatalf("highlighted text was corrupted: %q", plain)
	}
}

func stripANSI(value string) string {
	var result strings.Builder
	for len(value) > 0 {
		start := strings.Index(value, "\x1b[")
		if start < 0 {
			result.WriteString(value)
			break
		}
		result.WriteString(value[:start])
		end := strings.IndexByte(value[start:], 'm')
		if end < 0 {
			result.WriteString(value[start:])
			break
		}
		value = value[start+end+1:]
	}
	return result.String()
}
