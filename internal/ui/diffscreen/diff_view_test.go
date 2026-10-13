package diffscreen

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestUnifiedNavigationSearchAndFileJumps(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, ok := v.first()
	testutil.RequireEqual(t, ok, true, "First returned no cursor")

	line, _ := v.line(first)
	testutil.RequireEqual(t, line.Text, "before",
		fmt.Sprintf("first line = %q", line.Text),
	)

	next, ok := v.move(first, forward)
	testutil.RequireEqual(t, ok, true, "Move returned no cursor")

	line, _ = v.line(next)
	testutil.RequireEqual(t, line.Kind, patch.Deletion,
		fmt.Sprintf("next kind = %v", line.Kind),
	)

	match, ok := v.search("added", first, forward)
	testutil.RequireEqual(t, ok, true, "Search returned no cursor")

	line, _ = v.line(match)
	testutil.RequireEqual(t, line.Text, "added one",
		fmt.Sprintf("match = %q", line.Text),
	)

	jumped, ok := v.jumpFile(first, forward)
	testutil.RequireEqual(t, ok, true, "JumpFile returned no cursor")

	file, _ := v.file(jumped)
	testutil.RequireEqual(t, file.DisplayPath, "second.go",
		fmt.Sprintf("jumped file = %q", file.DisplayPath),
	)

	if _, ok := v.jumpFile(jumped, forward); ok {
		t.Fatal("JumpFile wrapped unexpectedly")
	}
}

func TestSplitPairsChangeBlocksAndSupportsEmptyPanes(t *testing.T) {
	v := newSideBySideView(testPatch(), true)
	first, _ := v.first()
	removed, _ := v.move(first, forward)
	testutil.RequireEqual(t, removed.pane, right,
		fmt.Sprintf("initial pane = %v", removed.pane),
	)

	removed, ok := v.switchPane(removed, left)
	testutil.RequireEqual(t, ok, true, "could not switch to deletion pane")

	line, _ := v.line(removed)
	testutil.RequireEqual(t, line.Text, "removed one",
		fmt.Sprintf("left line = %q", line.Text),
	)

	added, ok := v.switchPane(removed, right)
	testutil.RequireEqual(t, ok, true, "paired addition missing")

	line, _ = v.line(added)
	testutil.RequireEqual(t, line.Text, "added one",
		fmt.Sprintf("right line = %q", line.Text),
	)

	secondRemoved, _ := v.move(removed, forward)
	if _, ok := v.switchPane(secondRemoved, right); !ok {
		t.Fatal("pane switch should find a nearby right line")
	}

	viewport := v.newViewport(100, 20)
	rendered := v.render(viewport, added, nil)
	testutil.RequireEqual(t, strings.Contains(rendered, "removed one"), true,
		fmt.Sprintf("paired render missing lines: %q", rendered),
	)
	testutil.RequireEqual(t, strings.Contains(rendered, "added one"), true,
		fmt.Sprintf("paired render missing lines: %q", rendered),
	)

	for _, row := range strings.Split(rendered, "\n") {
		testutil.RequireEqual(t,
			!strings.Contains(row, " │ ") || lipgloss.Width(row) == 100, true,
			fmt.Sprintf("rendered width = %d", lipgloss.Width(row)),
		)
	}
}

func TestSelectionLines(t *testing.T) {
	v := newUnifiedView(testPatch(), true)
	first, _ := v.first()
	last, _ := v.move(first, forward)
	last, _ = v.move(last, forward)
	selection := beginSelection(first)
	selection, ok := v.extendSelection(selection, last)
	testutil.RequireEqual(t, ok, true, "selection extension failed")

	lines := v.lines(selection)
	testutil.RequireEqual(t, len(lines), 3,
		fmt.Sprintf("selected lines = %d", len(lines)),
	)

	nextFile, _ := v.jumpFile(first, forward)
	if _, ok := v.extendSelection(selection, nextFile); ok {
		t.Fatal("selection crossed a hunk")
	}
}
