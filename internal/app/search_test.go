package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	origin := renderedScreenBody(m.patchView.Render(nil))
	m = applyTestMessage(t, m, textKeyMessage("/"))
	m = applyTestMessage(t, m, textKeyMessage("keep"))
	first := renderedScreenBody(m.patchView.Render(nil))
	testutil.RequireEqual(t, m.mode, modeSearch,
		fmt.Sprintf("mode=%v line=%q", m.mode, focusedLineText(m)),
	)
	testutil.RequireEqual(t, focusedLineText(m), "keep()",
		fmt.Sprintf("mode=%v line=%q", m.mode, focusedLineText(m)),
	)

	m = applyTestMessage(t, m, specialKeyMessage(tea.KeyEnter))
	m = applyTestMessage(t, m, textKeyMessage("n"))
	testutil.RequireEqual(
		t, renderedScreenBody(m.patchView.Render(nil)) != first, true,
		fmt.Sprintf("next=%#v", renderedScreenBody(m.patchView.Render(nil))),
	)
	testutil.RequireEqual(t, focusedLineText(m), "keep()",
		fmt.Sprintf("next=%#v", renderedScreenBody(m.patchView.Render(nil))),
	)

	m = applyTestMessage(t, m, textKeyMessage("N"))
	testutil.RequireEqual(
		t, renderedScreenBody(m.patchView.Render(nil)), first,
		fmt.Sprintf(
			"previous=%#v", renderedScreenBody(m.patchView.Render(nil)),
		),
	)

	m = applyTestMessage(t, m, textKeyMessage("/"))
	m = applyTestMessage(t, m, textKeyMessage("missing"))
	testutil.RequireEqual(t, strings.Contains(m.render(), "no matches"), true,
		"missing search did not miss",
	)

	m = applyTestMessage(t, m, specialKeyMessage(tea.KeyEsc))
	testutil.RequireEqual(t, renderedScreenBody(m.patchView.Render(nil)), first,
		fmt.Sprintf("cancel=%#v origin=%#v",
			renderedScreenBody(m.patchView.Render(nil)), origin,
		),
	)
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	origin := renderedScreenBody(m.patchView.Render(nil))
	m = applyTestMessage(t, m, textKeyMessage("/"))
	m = applyTestMessage(t, m, textKeyMessage("main.go"))
	file, _, _ := m.patchView.Current()
	testutil.RequireEqual(t, file.DisplayPath, "main.go",
		fmt.Sprintf("file=%q", file.DisplayPath),
	)

	for range len("main.go") {
		m = applyTestMessage(t, m, specialKeyMessage(tea.KeyBackspace))
	}
	testutil.RequireEqual(
		t, renderedScreenBody(m.patchView.Render(nil)), origin,
		fmt.Sprintf(
			"diffCursor=%#v", renderedScreenBody(m.patchView.Render(nil)),
		),
	)
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	origin := renderedScreenBody(m.patchView.Render(nil))
	m = applyTestMessage(t, m, textKeyMessage("/"))
	m = applyTestMessage(t, m, textKeyMessage("old()"))
	testutil.RequireEqual(t, focusedLineText(m), "old()",
		fmt.Sprintf("diffCursor=%#v line=%q",
			renderedScreenBody(m.patchView.Render(nil)), focusedLineText(m),
		),
	)

	m = applyTestMessage(t, m, specialKeyMessage(tea.KeyEsc))
	testutil.RequireEqual(
		t, renderedScreenBody(m.patchView.Render(nil)), origin,
		fmt.Sprintf(
			"cancel=%#v want=%#v",
			renderedScreenBody(m.patchView.Render(nil)),
			origin,
		),
	)
}
