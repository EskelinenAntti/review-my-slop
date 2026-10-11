package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestRenamedFileHighlightsEachSideUsingItsOwnPath(t *testing.T) {
	source := "package main\n\nfunc example() {}\n"
	file := patch.File{
		OldPath:   "old.go",
		NewPath:   "new.txt",
		OldSource: source,
		NewSource: source,
	}
	view := &diffView{dark: true}
	pair := view.highlight(&file)
	old, newSource := strings.Join(pair.Old, newline), strings.Join(
		pair.New,
		newline,
	)
	requireEqual(
		t,
		old != newSource &&
			stripANSI(old) == strings.TrimSuffix(source, newline) &&
			stripANSI(newSource) == strings.TrimSuffix(source, newline),
		true,
		"highlighted old=%q new=%q",
		old,
		newSource,
	)
}

func TestLicenseHighlightFixture(t *testing.T) {
	lines := render("LICENSE", `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`, true)
	requireEqual(
		t,
		len(lines), five,
		"highlighted lines = %d, want 5",
		len(lines),
	)
	plain := stripANSI(strings.Join(lines, newline))
	requireEqual(
		t,
		strings.Contains(plain, `Copyright (c) 2026`) &&
			strings.Contains(
				plain,
				`"AS IS"`,
			),
		true,
		"highlighted text was corrupted: %q",
		plain,
	)
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
	dark := strings.Join(render("example.go", source, true), newline)
	light := strings.Join(render("example.go", source, false), newline)
	requireEqual(
		t,
		dark != light,
		true,
		"%v",
		"light and dark terminal backgrounds use identical highlighting",
	)
	for name, rendered := range map[string]string{
		"dark":  dark,
		"light": light,
	} {
		checkEqual(
			t,
			strings.Contains(rendered, "[38;2;"),
			true,
			"%s theme does not use truecolor syntax highlighting: %q",
			name,
			rendered,
		)
		checkEqual(
			t,
			!strings.Contains(rendered, "[48;2;"),
			true,
			"%s theme overrides terminal background: %q",
			name,
			rendered,
		)
	}
}

func stripANSI(initialValue string) string {
	value := initialValue

	var result strings.Builder
	for len(value) > zero {
		start := strings.Index(value, "\x1b[")
		if start < zero {
			_, _ = result.WriteString(value)
			break
		}
		_, _ = result.WriteString(value[:start])
		end := strings.IndexByte(value[start:], 'm')
		if end < zero {
			_, _ = result.WriteString(value[start:])
			break
		}
		value = value[start+end+one:]
	}
	return result.String()
}
