package diffscreen

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestRenamedFileHighlightsEachSideUsingItsOwnPath(t *testing.T) {
	source := "package main\n\nfunc example() {}\n"
	file := patch.File{OldPath: "old.go", NewPath: "new.txt", OldSource: source, NewSource: source}
	var hunk patch.Hunk
	for _, kind := range []patch.LineKind{patch.Deletion, patch.Addition} {
		for index, text := range strings.Split(strings.TrimSuffix(source, "\n"), "\n") {
			hunk.Lines = append(hunk.Lines, patch.Line{Kind: kind, Text: text, OldNumber: patch.LineNumber(index + 1), NewNumber: patch.LineNumber(index + 1)})
		}
	}
	file.Hunks = []patch.Hunk{hunk}
	view := newDiffView(patch.Patch{Files: []patch.File{file}}, true, true)
	var oldLines, newLines []string
	for _, row := range view.rows {
		if row.kind == lineRow {
			oldLines = append(oldLines, row.left)
			newLines = append(newLines, row.right)
		}
	}
	old, new := strings.Join(oldLines, "\n"), strings.Join(newLines, "\n")
	if old == new || stripANSI(old) != strings.TrimSuffix(source, "\n") || stripANSI(new) != strings.TrimSuffix(source, "\n") {
		t.Fatalf("highlighted old=%q new=%q", old, new)
	}
}

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
