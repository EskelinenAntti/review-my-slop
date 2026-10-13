package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		false,
		func(enabled bool) error { saved = append(saved, enabled); return nil },
	)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateModel(t, m, textKey("G"))
	testutil.RequireEqual(t, lineText(m), "more()",
		fmt.Sprintf("G line=%q", lineText(m)),
	)

	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("g"))
	testutil.RequireEqual(t, lineText(m), "package main",
		fmt.Sprintf("gg line=%q", lineText(m)),
	)

	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, m.diffOptions.SideBySide, true,
		"split view not enabled",
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		"split view not enabled",
	)

	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, !m.diffOptions.SideBySide, true,
		fmt.Sprintf("saved=%v", saved),
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		fmt.Sprintf("saved=%v", saved),
	)
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		true,
		func(enabled bool) error { saved = append(saved, enabled); return nil },
	)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, textKey("t"))
	testutil.RequireEqual(t, !m.diffOptions.SideBySide, true,
		fmt.Sprintf("sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved),
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{false}), true,
		fmt.Sprintf("sideBySide=%v saved=%v", m.diffOptions.SideBySide, saved),
	)
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.configureSideBySide(true, nil)
	focusLine(t, &m, "keep()")
	_, before, _ := m.diffView.Current()
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.diffView.Current()
	testutil.RequireEqual(t, ok, true,
		fmt.Sprintf("focused line=%#v, want %#v", got, before),
	)
	testutil.RequireEqual(t, got, before,
		fmt.Sprintf("focused line=%#v, want %#v", got, before),
	)
}

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreen.Alignment
	}{
		{
			"z",
			diffscreen.Center,
		}, {"t", diffscreen.Top}, {"b", diffscreen.Bottom},
	} {
		m := testModel(longModelPatch(), nil, nil)
		m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 9})
		want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
		want.Resize(100, 9)
		for range 10 {
			m.diffView.Move(diffscreen.NextLine)
			want.Move(diffscreen.NextLine)
		}
		want.Align(test.alignment)
		m = updateModel(t, m, textKey("z"))
		m = updateModel(t, m, textKey(test.key))
		testutil.RequireEqual(t, m.diffView.Render(nil), want.Render(nil),
			fmt.Sprintf("z%s did not align view", test.key),
		)
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("G"))
	last := lineText(m)
	m = updateModel(t, m, textKey("g"))
	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, textKey("g"))
	testutil.RequireEqual(t, lineText(m), last, "pending prefix moved focus")

	m = updateModel(t, m, textKey("g"))
	testutil.RequireEqual(t, lineText(m), "package main",
		fmt.Sprintf("gg line=%q", lineText(m)),
	)
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	focusLine(t, &m, "new()")
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, textKey("h"))
	testutil.RequireEqual(t, lineText(m), "old()",
		fmt.Sprintf("old pane line=%q", lineText(m)),
	)

	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, controlKey('w'))
	testutil.RequireEqual(t, lineText(m), "new()",
		fmt.Sprintf("new pane line=%q", lineText(m)),
	)
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 37, Height: 30})
	initial := m.diffView.Render(nil)
	want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
	want.Resize(37, 30)
	want.ScrollHorizontal(2 * horizontalScrollStep)
	m = updateModel(t, m, textKey("l"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	testutil.RequireEqual(t, m.diffView.Render(nil), want.Render(nil),
		"right keys did not scroll by two steps",
	)

	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	testutil.RequireEqual(t, m.diffView.Render(nil), initial,
		"left keys did not restore initial offset",
	)

	m = updateModel(t, m, textKey("$"))
	testutil.RequireEqual(t, m.diffView.Render(nil) != initial, true,
		"$ did not scroll",
	)

	m = updateModel(t, m, textKey("0"))
	testutil.RequireEqual(t, m.diffView.Render(nil), initial,
		"0 did not restore initial offset",
	)
}
