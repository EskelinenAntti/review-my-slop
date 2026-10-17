package app

import (
	"errors"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	patches "github.com/eskelinenantti/review-my-slop/internal/patch"
)

func TestAppWorkflowErrorKeepsDiffLabelWithinFooter(t *testing.T) {
	for _, width := range []int{20, 80} {
		for _, kind := range []patches.Kind{patches.Unstaged, patches.Branch} {
			assertWorkflowErrorFooter(t, width, kind)
		}
	}
}

func assertWorkflowErrorFooter(t *testing.T, width int, kind patches.Kind) {
	t.Helper()
	patch := newTestBasicPatch()
	patch.Kind, patch.Branch = kind, "main"
	model := newTestBasicAppModel()
	model.currentPatch = patch
	model.width, model.height = width, 8
	model.diffView = newTestDiffscreenView(patch, model.diffOptions)
	model.diffView.Resize(model.width, model.height)
	model.err = errors.New(
		"a very long storage failure\n" +
			"that exceeds the available footer width",
	)
	lines := strings.Split(renderTestAppMode(model), "\n")
	requireEqual(t, len(lines), 8,
		"error changed screen height: %d", len(lines))
	footer := lines[len(lines)-2]
	label := workflowDiffLabel(kind)
	requireEqual(t,
		strings.HasSuffix(ansi.Strip(footer), label), true,
		"footer lost label: %q", footer,
	)
	// Labels longer than the terminal retain the existing rendering behavior.
	requireEqual(t,
		width < len(label)+1 || lipgloss.Width(footer) == width, true,
		"footer width=%d want=%d", lipgloss.Width(footer), width,
	)
}

func workflowDiffLabel(kind patches.Kind) string {
	if kind == patches.Branch {
		return "branch changes from main"
	}
	return "local changes"
}
