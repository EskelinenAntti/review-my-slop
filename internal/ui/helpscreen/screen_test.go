package helpscreen

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderAlignsBindingsAndKeepsHintAtBottom(t *testing.T) {
	lines := strings.Split(ansi.Strip(Render([]Binding{{"x", "short"}, {"long", "wide"}}, 80, 8)), "\n")
	testutil.Requiref(t, strings.Index(lines[2], "short") == strings.Index(lines[3], "wide"), "unaligned rows: %q", lines)
	testutil.Requiref(t, len(lines) == 8 && strings.Contains(lines[6], "? or Esc closes help"), "misplaced hint: %q", lines)
}

func TestRenderEmptyAndTruncatedBindings(t *testing.T) {
	for _, height := range []int{1, 4, 8} {
		result := Render(nil, 80, height)
		testutil.Requiref(t, strings.Contains(result, "? or Esc closes help"), "missing hint: %q", result)
	}
	result := Render([]Binding{{"one", "shown"}, {"two", "hidden"}}, 80, 5)
	testutil.Requiref(t, strings.Contains(result, "shown") && !strings.Contains(result, "hidden"), "body not truncated: %q", result)
}

func TestRenderClipsWideBindingText(t *testing.T) {
	rendered := Render([]Binding{{"界", strings.Repeat("description", 10)}}, 24, 8)
	for _, line := range strings.Split(rendered, "\n") {
		testutil.Requiref(t, ansi.StringWidth(line) <= 24, "line exceeds width: %q", line)
	}
}
