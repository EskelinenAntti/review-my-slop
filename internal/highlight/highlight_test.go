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

func TestGruvboxUsesDistinctGoTokenColors(t *testing.T) {
	lines := render("example.go", `package main

// comment
func answer(value int) string {
	if value >= 42 {
		return "ready"
	}
	return ""
}
`)
	rendered := strings.Join(lines, "\n")
	expected := map[string]string{
		"keyword":  "\x1b[38;2;254;128;25m",
		"function": "\x1b[38;2;250;189;47m",
		"string":   "\x1b[38;2;184;187;38m",
		"number":   "\x1b[38;2;211;134;155m",
		"comment":  "\x1b[38;2;146;131;116m",
	}
	for token, sequence := range expected {
		if !strings.Contains(rendered, sequence) {
			t.Errorf("%s color %q not found in %q", token, sequence, rendered)
		}
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
