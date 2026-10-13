package app

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestWorkflowErrorKeepsDiffLabelWithinFooter(t *testing.T) {
	for _, width := range []int{20, 80} {
		for _, kind := range []patch.Kind{patch.Unstaged, patch.Branch} {
			p := singleHunkPatch()
			p.Kind, p.Branch = kind, "main"
			m := newModel(
				p,
				nil,
				nil,
				initialLayout{size: size{Width: width, Height: 8}},
			)
			m.err = errors.New(
				"a very long storage failure\n" +
					"that exceeds the available footer width",
			)
			lines := strings.Split(m.render(), "\n")
			testutil.RequireEqual(t, len(lines), 8,
				fmt.Sprintf("error changed screen height: %d", len(lines)),
			)

			footer := lines[len(lines)-2]
			label := "local changes"
			if kind == patch.Branch {
				label = "branch changes from main"
			}
			testutil.RequireEqual(t,
				strings.HasSuffix(ansi.Strip(footer), label), true,
				fmt.Sprintf("footer lost label: %q", footer),
			)
			testutil.RequireEqual( // Labels longer than the terminal retain
				// the existing rendering behavior.
				t, width < len(label)+1 || lipgloss.Width(footer) == width,
				true,
				fmt.Sprintf("footer width=%d want=%d", lipgloss.Width(footer),
					width,
				),
			)
		}
	}
}
