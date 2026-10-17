package main

import (
	bytes "bytes"
	io "io"
	strings "strings"
	testing "testing"

	quick "github.com/alecthomas/chroma/v2/quick"
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
	inlineLocalPair1 := func() diffscreenPair {
		var file *patchFile = &file
		theme := diffscreenLightTheme
		if view.dark {
			theme = diffscreenDarkTheme
		}
		return diffscreenPair{Old: func() []string {
			var filename, source, theme string = file.OldPath, file.OldSource, theme
			if source == "" {
				return nil
			}
			var buf bytes.Buffer
			highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
			if highlightErr != nil {
				return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
			}
			rendered, err := io.ReadAll(&buf)
			if err != nil {
				return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
			}
			return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
		}(), New: func() []string {
			var filename, source, theme string = file.NewPath, file.NewSource, theme
			if source == "" {
				return nil
			}
			var buf bytes.Buffer
			highlightErr := quick.Highlight(&buf, source, filename, "terminal16m", theme)
			if highlightErr != nil {
				return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
			}
			rendered, err := io.ReadAll(&buf)
			if err != nil {
				return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
			}
			return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
		}()}
	}()
	old, updated := strings.Join(inlineLocalPair1.Old, "\n"), strings.Join(inlineLocalPair1.New, "\n")
	if old == updated || (func(value string) string {
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
	})(old) != strings.TrimSuffix(
		source,
		"\n",
	) || (func(value string) string {
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
	})(updated) != strings.TrimSuffix(source, "\n") {
		t.Fatalf("highlighted old=%q new=%q", old, updated)
	}
}

func TestDiffscreenLicenseHighlightFixture(t *testing.T) {
	lines := func() []string {
		var source string = `MIT License

Copyright (c) 2026 Antti Eskelinen

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND.
`
		if source == "" {
			return nil
		}
		var buf bytes.Buffer
		highlightErr := quick.Highlight(&buf, source, "LICENSE", "terminal16m", diffscreenDarkTheme)
		if highlightErr != nil {
			return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
		}
		rendered, err := io.ReadAll(&buf)
		if err != nil {
			return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
		}
		return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
	}()
	if len(lines) != 5 {
		t.Fatalf("highlighted lines = %d, want 5", len(lines))
	}
	plain := (func(value string) string {
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
	})(strings.Join(lines, "\n"))
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
	dark := strings.Join(func() []string {
		if source == "" {
			return nil
		}
		var buf bytes.Buffer
		highlightErr := quick.Highlight(&buf, source, "example.go", "terminal16m", diffscreenDarkTheme)
		if highlightErr != nil {
			return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
		}
		rendered, err := io.ReadAll(&buf)
		if err != nil {
			return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
		}
		return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
	}(), "\n")
	light := strings.Join(func() []string {
		if source == "" {
			return nil
		}
		var buf bytes.Buffer
		highlightErr := quick.Highlight(&buf, source, "example.go", "terminal16m", diffscreenLightTheme)
		if highlightErr != nil {
			return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
		}
		rendered, err := io.ReadAll(&buf)
		if err != nil {
			return strings.Split(strings.TrimSuffix(source, "\n"), "\n")
		}
		return strings.Split(strings.TrimSuffix(string(rendered), "\n"), "\n")
	}(), "\n")
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
