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
	m := newTestModel(multiHunkPatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		false,
		func(enabled bool) error { saved = append(saved, enabled); return nil },
	)
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = applyTestMessage(t, m, textKeyMessage("G"))
	testutil.RequireEqual(t, focusedLineText(m), "more()",
		fmt.Sprintf("G line=%q", focusedLineText(m)),
	)

	m = applyTestMessage(t, m, textKeyMessage("g"))
	m = applyTestMessage(t, m, textKeyMessage("g"))
	testutil.RequireEqual(t, focusedLineText(m), "package main",
		fmt.Sprintf("gg line=%q", focusedLineText(m)),
	)

	m = applyTestMessage(t, m, textKeyMessage("t"))
	testutil.RequireEqual(t, m.patchViewOptions.SideBySide, true,
		"split view not enabled",
	)
	testutil.RequireEqual(t, strings.Contains(m.render(), "│"), true,
		"split view not enabled",
	)

	m = applyTestMessage(t, m, textKeyMessage("t"))
	testutil.RequireEqual(t, !m.patchViewOptions.SideBySide, true,
		fmt.Sprintf("saved=%v", saved),
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{true, false}), true,
		fmt.Sprintf("saved=%v", saved),
	)
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		true,
		func(enabled bool) error { saved = append(saved, enabled); return nil },
	)
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = applyTestMessage(t, m, textKeyMessage("t"))
	testutil.RequireEqual(t, !m.patchViewOptions.SideBySide, true,
		fmt.Sprintf(
			"sideBySide=%v saved=%v", m.patchViewOptions.SideBySide, saved,
		),
	)
	testutil.RequireEqual(t, slices.Equal(saved, []bool{false}), true,
		fmt.Sprintf(
			"sideBySide=%v saved=%v", m.patchViewOptions.SideBySide, saved,
		),
	)
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	m.configureSideBySide(true, nil)
	focusPatchLine(t, &m, "keep()")
	_, before, _ := m.patchView.Current()
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 80, Height: 20})
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.patchView.Current()
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
		m := newTestModel(longContextPatch(), nil, nil)
		m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 100, Height: 9})
		want := diffscreen.New(
			longContextPatch(), diffscreen.Options{Dark: true},
		)
		want.Resize(100, 9)
		for range 10 {
			m.patchView.Move(diffscreen.NextLine)
			want.Move(diffscreen.NextLine)
		}
		want.Align(test.alignment)
		m = applyTestMessage(t, m, textKeyMessage("z"))
		m = applyTestMessage(t, m, textKeyMessage(test.key))
		testutil.RequireEqual(t, m.patchView.Render(nil), want.Render(nil),
			fmt.Sprintf("z%s did not align view", test.key),
		)
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	m = applyTestMessage(t, m, textKeyMessage("G"))
	last := focusedLineText(m)
	m = applyTestMessage(t, m, textKeyMessage("g"))
	m = applyTestMessage(t, m, textKeyMessage("h"))
	m = applyTestMessage(t, m, textKeyMessage("g"))
	testutil.RequireEqual(
		t, focusedLineText(m), last, "pending prefix moved focus",
	)

	m = applyTestMessage(t, m, textKeyMessage("g"))
	testutil.RequireEqual(t, focusedLineText(m), "package main",
		fmt.Sprintf("gg line=%q", focusedLineText(m)),
	)
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := newTestModel(multiHunkPatch(), nil, nil)
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m.setSideBySide(true)
	focusPatchLine(t, &m, "new()")
	m = applyTestMessage(t, m, controlKeyMessage('w'))
	m = applyTestMessage(t, m, textKeyMessage("h"))
	testutil.RequireEqual(t, focusedLineText(m), "old()",
		fmt.Sprintf("old pane line=%q", focusedLineText(m)),
	)

	m = applyTestMessage(t, m, controlKeyMessage('w'))
	m = applyTestMessage(t, m, controlKeyMessage('w'))
	testutil.RequireEqual(t, focusedLineText(m), "new()",
		fmt.Sprintf("new pane line=%q", focusedLineText(m)),
	)
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := newTestModel(longContextPatch(), nil, nil)
	m = applyTestMessage(t, m, tea.WindowSizeMsg{Width: 37, Height: 30})
	initial := m.patchView.Render(nil)
	want := diffscreen.New(longContextPatch(), diffscreen.Options{Dark: true})
	want.Resize(37, 30)
	want.ScrollHorizontal(2 * horizontalScrollStep)
	m = applyTestMessage(t, m, textKeyMessage("l"))
	m = applyTestMessage(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	testutil.RequireEqual(t, m.patchView.Render(nil), want.Render(nil),
		"right keys did not scroll by two steps",
	)

	m = applyTestMessage(t, m, textKeyMessage("h"))
	m = applyTestMessage(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	testutil.RequireEqual(t, m.patchView.Render(nil), initial,
		"left keys did not restore initial offset",
	)

	m = applyTestMessage(t, m, textKeyMessage("$"))
	testutil.RequireEqual(t, m.patchView.Render(nil) != initial, true,
		"$ did not scroll",
	)

	m = applyTestMessage(t, m, textKeyMessage("0"))
	testutil.RequireEqual(t, m.patchView.Render(nil), initial,
		"0 did not restore initial offset",
	)
}
