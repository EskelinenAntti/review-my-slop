package app

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestWorkflowErrorKeepsDiffLabelWithinFooter(t *testing.T) {
	for _, width := range []int{minimumWidth, defaultWidth} {
		for _, kind := range []patch.Kind{patch.Unstaged, patch.Branch} {
			p := modelPatch()
			p.Kind, p.Branch = kind, "main"
			m := newModel(
				p,
				nil,
				nil,
				initialLayout{size: size{Width: width, Height: 8}},
			)
			m.err = errors.New(
				"a very long storage failure\nthat exceeds" +
					" the available footer width",
			)
			lines := strings.Split(m.render(), "\n")
			requireEqual(
				t,
				len(lines), 8,
				"error changed screen height: %d",
				len(lines),
			)
			footer := lines[len(lines)-2]
			label := "local changes"
			if kind == patch.Branch {
				label = "branch changes from main"
			}
			requireEqual(
				t,
				strings.HasSuffix(ansi.Strip(footer), label), true,
				"footer lost label: %q",
				footer,
			)
			// Labels longer than the terminal are retained, matching existing
			// behavior.
			requireEqual(
				t,
				width < len(label)+1 || lipgloss.Width(footer) == width,
				true,
				"footer width=%d want=%d",
				lipgloss.Width(footer),
				width,
			)
		}
	}
}
