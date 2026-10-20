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
	highlighted := view.highlight(file)
	old := strings.Join(highlighted.old, "\n")
	updated := strings.Join(highlighted.new, "\n")
	if old == updated || stripANSI(old) != strings.TrimSuffix(
		source,
		"\n",
	) || stripANSI(updated) != strings.TrimSuffix(source, "\n") {
		t.Fatalf("highlighted old=%q new=%q", old, updated)
	}
}

func TestLicenseHighlightFixture(t *testing.T) {
	lines := highlightSource("LICENSE", `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`, darkTheme)
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
	file := patch.File{NewPath: "example.go", NewSource: source}
	darkView, lightView := &diffView{dark: true}, &diffView{dark: false}
	dark := strings.Join(darkView.highlight(file).new, "\n")
	light := strings.Join(lightView.highlight(file).new, "\n")
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

func TestHighlightedSourcesText(t *testing.T) {
	sources := highlightedSources{
		old: []string{"old first", "old second"},
		new: []string{"new first", "new second"},
	}
	for _, test := range []struct {
		name string
		line patch.Line
		want string
	}{
		{
			"deletion uses old number",
			patch.Line{Kind: patch.Deletion, OldNumber: 2, NewNumber: 1},
			"old second",
		},
		{
			"addition uses new number",
			patch.Line{Kind: patch.Addition, OldNumber: 2, NewNumber: 1},
			"new first",
		},
		{
			"context uses new source",
			patch.Line{Kind: patch.Context, OldNumber: 1, NewNumber: 2},
			"new second",
		},
		{
			"missing number",
			patch.Line{Kind: patch.Addition, Text: "fallback"},
			"fallback",
		},
		{
			"negative number",
			patch.Line{Kind: patch.Deletion, OldNumber: -1, Text: "fallback"},
			"fallback",
		},
		{
			"old number out of range",
			patch.Line{Kind: patch.Deletion, OldNumber: 3, Text: "fallback"},
			"fallback",
		},
		{
			"new number out of range",
			patch.Line{Kind: patch.Context, NewNumber: 3, Text: "fallback"},
			"fallback",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := sources.text(test.line); got != test.want {
				t.Fatalf("text(%+v) = %q, want %q", test.line, got, test.want)
			}
		})
	}
}

func TestHighlightedSourcesWithoutSource(t *testing.T) {
	view := &diffView{dark: true}
	sources := view.highlight(patch.File{})
	for _, kind := range []patch.LineKind{
		patch.Context, patch.Addition, patch.Deletion,
	} {
		line := patch.Line{
			Kind: kind, OldNumber: 1, NewNumber: 1, Text: "fallback",
		}
		if got := sources.text(line); got != line.Text {
			t.Errorf("text(%+v) = %q, want %q", line, got, line.Text)
		}
	}
}

func stripANSI(value string) string {
	var result strings.Builder
	for len(value) > 0 {
		start := strings.Index(value, "\x1b[")
		if start < 0 {
			_, _ = result.WriteString(value)
			break
		}
		_, _ = result.WriteString(value[:start])
		end := strings.IndexByte(value[start:], 'm')
		if end < 0 {
			_, _ = result.WriteString(value[start:])
			break
		}
		value = value[start+end+1:]
	}
	return result.String()
}
