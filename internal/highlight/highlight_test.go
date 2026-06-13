package highlight

import (
	"strings"
	"testing"
)

func TestLicenseHighlightFixture(t *testing.T) {
	lines := render("LICENSE", `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`, true)
	if len(lines) != 5 {
		t.Fatalf("highlighted lines = %d, want 5", len(lines))
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, `Copyright (c) 2026`) ||
		!strings.Contains(plain, `"AS IS"`) {
		t.Fatalf("highlighted text was corrupted: %q", plain)
	}
}

func TestHighlightAdaptsToTerminalBackground(t *testing.T) {
	source := `package main

// comment
func answer(value int) string {
	if value >= 42 {
		return "ready"
	}
	return ""
}
`
	dark := strings.Join(render("example.go", source, true), "\n")
	light := strings.Join(render("example.go", source, false), "\n")
	if dark == light {
		t.Fatal("light and dark terminal backgrounds use identical highlighting")
	}
	for name, rendered := range map[string]string{"dark": dark, "light": light} {
		if !strings.Contains(rendered, "[38;2;") {
			t.Errorf("%s theme does not use truecolor syntax highlighting: %q", name, rendered)
		}
		if strings.Contains(rendered, "[48;2;") {
			t.Errorf("%s theme overrides terminal background: %q", name, rendered)
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
