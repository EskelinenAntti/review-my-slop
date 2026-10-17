package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestHelpscreenRenderAlignsBindingsAndKeepsHintAtBottom(t *testing.T) {
	lines := strings.Split(ansi.Strip(RenderHelp(
		[]Binding{{"x", "short"}, {"long", "wide"}}, 80, 8,
	)), "\n")
	if strings.Index(lines[2], "short") != strings.Index(lines[3], "wide") {
		t.Fatalf("unaligned rows: %q", lines)
	}
	if len(lines) != 8 || !strings.Contains(lines[6], "? or Esc closes help") {
		t.Fatalf("misplaced hint: %q", lines)
	}
}

func TestHelpscreenRenderEmptyAndTruncatedBindings(t *testing.T) {
	for _, height := range []int{1, 4, 8} {
		result := RenderHelp(nil, 80, height)
		if !strings.Contains(result, "? or Esc closes help") {
			t.Fatalf("missing hint: %q", result)
		}
	}
	result := RenderHelp(
		[]Binding{{"one", "shown"}, {"two", "hidden"}}, 80, 5,
	)
	if !strings.Contains(
		result,
		"shown",
	) || strings.Contains(
		result,
		"hidden",
	) {
		t.Fatalf("body not truncated: %q", result)
	}
}

func TestHelpscreenRenderClipsWideBindingText(t *testing.T) {
	rendered := RenderHelp(
		[]Binding{{"界", strings.Repeat("description", 10)}}, 24, 8,
	)
	for _, line := range strings.Split(rendered, "\n") {
		if ansi.StringWidth(line) > 24 {
			t.Fatalf("line exceeds width: %q", line)
		}
	}
}
