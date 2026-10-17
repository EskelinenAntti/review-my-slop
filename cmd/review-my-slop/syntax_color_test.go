package main

import (
	"strings"
	"testing"
)

func TestDiffscreenRenamedFileHighlightsEachSideUsingItsOwnPath(t *testing.T) {
	source := "package main\n\nfunc example() {}\n"
	file := patchFile{
		OldPath:   "old.go",
		NewPath:   "new.txt",
		OldSource: source,
		NewSource: source,
	}
	view := &diffscreenDiffView{dark: true}
	theme := diffscreenLightTheme
	if view.dark {
		theme = diffscreenDarkTheme
	}
	old := strings.Join(patchHighlightSource(
		file.OldSource,
		file.OldPath,
		theme,
	), "\n")
	updated := strings.Join(patchHighlightSource(
		file.NewSource,
		file.NewPath,
		theme,
	), "\n")
	plainSource := strings.TrimSuffix(source, "\n")
	if old == updated || patchStripANSI(old) != plainSource ||
		patchStripANSI(updated) != plainSource {
		t.Fatalf("highlighted old=%q new=%q", old, updated)
	}
}

func TestDiffscreenLicenseHighlightFixture(t *testing.T) {
	source := `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`
	lines := patchHighlightSource(source, "LICENSE", diffscreenDarkTheme)
	if len(lines) != 5 {
		t.Fatalf("highlighted lines = %d, want 5", len(lines))
	}
	plain := patchStripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, `Copyright (c) 2026`) ||
		!strings.Contains(plain, `"AS IS"`) {
		t.Fatalf("highlighted text was corrupted: %q", plain)
	}
}

func TestDiffscreenHighlightAdaptsToTerminalBackground(t *testing.T) {
	source := `package main

// comment
func answer(value int) string {
	if value >= 42 {
		return "ready"
	}
	return ""
}
`
	dark := strings.Join(patchHighlightSource(
		source,
		"example.go",
		diffscreenDarkTheme,
	), "\n")
	light := strings.Join(patchHighlightSource(
		source,
		"example.go",
		diffscreenLightTheme,
	), "\n")
	if dark == light {
		t.Fatal(
			"light and dark terminal backgrounds use identical highlighting",
		)
	}
	for name, rendered := range map[string]string{
		"dark":  dark,
		"light": light,
	} {
		if !strings.Contains(rendered, "[38;2;") {
			t.Errorf(
				"%s theme does not use truecolor syntax highlighting: %q",
				name,
				rendered,
			)
		}
		if strings.Contains(rendered, "[48;2;") {
			t.Errorf(
				"%s theme overrides terminal background: %q",
				name,
				rendered,
			)
		}
	}
}
