package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestSearchMovesIncrementallyRepeatsAndRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("keep"))
	first := screenBody(m.diffView.Render(nil))
	testutil.RequireEqual(t, m.mode, modeSearch, "mode=%v line=%q", m.mode,
		lineText(m),
	)
	testutil.RequireEqual(t, lineText(m), "keep()", "mode=%v line=%q", m.mode,
		lineText(m),
	)

	m = updateModel(t, m, specialKey(tea.KeyEnter))
	m = updateModel(t, m, textKey("n"))
	testutil.RequireEqual(t, screenBody(m.diffView.Render(nil)) != first, true,
		"next=%#v", screenBody(m.diffView.Render(nil)),
	)
	testutil.RequireEqual(t, lineText(m), "keep()", "next=%#v",
		screenBody(m.diffView.Render(nil)),
	)

	m = updateModel(t, m, textKey("N"))
	testutil.RequireEqual(t, screenBody(m.diffView.Render(nil)), first,
		"previous=%#v", screenBody(m.diffView.Render(nil)),
	)

	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("missing"))
	testutil.RequireEqual(t, strings.Contains(m.render(), "no matches"), true,
		"%s", "missing search did not miss",
	)

	m = updateModel(t, m, specialKey(tea.KeyEsc))
	testutil.RequireEqual(t, screenBody(m.diffView.Render(nil)), first,
		"cancel=%#v origin=%#v", screenBody(m.diffView.Render(nil)), origin,
	)
}

func TestSearchMatchesFileNamesAndBackspaceRestoresOrigin(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("main.go"))
	file, _, _ := m.diffView.Current()
	testutil.RequireEqual(t, file.DisplayPath, "main.go", "file=%q",
		file.DisplayPath,
	)

	for range len("main.go") {
		m = updateModel(t, m, specialKey(tea.KeyBackspace))
	}
	testutil.RequireEqual(t, screenBody(m.diffView.Render(nil)), origin,
		"diffCursor=%#v", screenBody(m.diffView.Render(nil)),
	)
}

func TestSideBySideSearchActivatesPaneAndCancelRestoresIt(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	origin := screenBody(m.diffView.Render(nil))
	m = updateModel(t, m, textKey("/"))
	m = updateModel(t, m, textKey("old()"))
	testutil.RequireEqual(t, lineText(m), "old()", "diffCursor=%#v line=%q",
		screenBody(m.diffView.Render(nil)), lineText(m),
	)

	m = updateModel(t, m, specialKey(tea.KeyEsc))
	testutil.RequireEqual(t, screenBody(m.diffView.Render(nil)), origin,
		"cancel=%#v want=%#v", screenBody(m.diffView.Render(nil)), origin,
	)
}
