package helpscreen

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestRenderAlignsBindingsAndKeepsHintAtBottom(t *testing.T) {
	lines := strings.Split(
		ansi.Strip(
			Render(
				[]Binding{{"x", "short"}, {"long", "wide"}},
				defaultWidth,
				8,
			),
		),
		"\n",
	)
	requireEqual(
		t,
		strings.Index(lines[2], "short"), strings.Index(lines[3], "wide"),
		"unaligned rows: %q",
		lines,
	)
	requireEqual(
		t,
		len(lines) == 8 &&
			strings.Contains(lines[6], "? or Esc closes help"),
		true,
		"misplaced hint: %q",
		lines,
	)
}

func TestRenderEmptyAndTruncatedBindings(t *testing.T) {
	for _, height := range []int{1, 4, 8} {
		result := Render(nil, defaultWidth, height)
		requireEqual(
			t,
			strings.Contains(result, "? or Esc closes help"), true,
			"missing hint: %q",
			result,
		)
	}
	result := Render(
		[]Binding{{"one", "shown"}, {"two", "hidden"}},
		defaultWidth,
		5,
	)
	requireEqual(
		t,
		strings.Contains(result, "shown") &&
			!strings.Contains(result, "hidden"),
		true,
		"body not truncated: %q",
		result,
	)
}

func TestRenderClipsWideBindingText(t *testing.T) {
	rendered := Render(
		[]Binding{{"界", strings.Repeat("description", 10)}},
		narrowTestWidth,
		8,
	)
	for _, line := range strings.Split(rendered, "\n") {
		requireEqual(
			t,
			ansi.StringWidth(line) <= narrowTestWidth, true,
			"line exceeds width: %q",
			line,
		)
	}
}
