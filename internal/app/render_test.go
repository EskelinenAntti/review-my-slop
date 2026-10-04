package app

import (
	"charm.land/lipgloss/v2"
	"fmt"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"strings"
	"testing"
)

func TestWorkflowErrorKeepsDiffLabelWithinFooter(t *testing.T) {
	for _, width := range []int{20, 80} {
		for _, kind := range []patch.Kind{patch.Unstaged, patch.Branch} {
			p := modelPatch()
			p.Kind, p.Branch = kind, "main"
			m := newModel(p, nil, nil, initialLayout{size: size{Width: width, Height: 8}})
			m.err = fmt.Errorf("a very long storage failure that exceeds the available footer width")
			lines := strings.Split(m.render(), "\n")
			footer := lines[len(lines)-2]
			label := "local changes"
			if kind == patch.Branch {
				label = "branch changes from main"
			}
			if !strings.HasSuffix(ansi.Strip(footer), label) {
				t.Fatalf("footer lost label: %q", footer)
			}
			// Labels longer than the terminal are retained, matching existing behavior.
			if width >= len(label)+1 && lipgloss.Width(footer) != width {
				t.Fatalf("footer width=%d want=%d", lipgloss.Width(footer), width)
			}
		}
	}
}
