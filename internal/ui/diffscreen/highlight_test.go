package diffscreen

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenamedFileHighlightsEachSideUsingItsOwnPath(t *testing.T) {
	source := "package main\n\nfunc example() {}\n"
	file := patch.File{OldPath: "old.go", NewPath: "new.txt", OldSource: source, NewSource: source}
	view := &diffView{dark: true}
	pair := view.highlight(&file)
	old, new := strings.Join(pair.Old, "\n"), strings.Join(pair.New, "\n")
	testutil.Requiref(t, old != new && stripANSI(old) == strings.TrimSuffix(source, "\n") && stripANSI(new) == strings.TrimSuffix(source, "\n"), "highlighted old=%q new=%q", old, new)
}

func TestLicenseHighlightFixture(t *testing.T) {
	lines := render("LICENSE", `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`, true)
	testutil.Requiref(t, len(lines) == 5, "highlighted lines = %d, want 5", len(lines))
	plain := stripANSI(strings.Join(lines, "\n"))
	testutil.Requiref(t, strings.Contains(plain, `Copyright (c) 2026`) &&
		strings.Contains(plain, `"AS IS"`), "highlighted text was corrupted: %q", plain)
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
	testutil.Require(t, dark != light, "light and dark terminal backgrounds use identical highlighting")
	for name, rendered := range map[string]string{"dark": dark, "light": light} {
		testutil.Assertf(t, strings.Contains(rendered, "[38;2;"), "%s theme does not use truecolor syntax highlighting: %q", name, rendered)
		testutil.Assertf(t, !strings.Contains(rendered, "[48;2;"), "%s theme overrides terminal background: %q", name, rendered)
	}
}

func stripANSI(value string) string { return ansi.Strip(value) }
