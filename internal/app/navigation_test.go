package app

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey(selectionKey))
	for range 10 {
		m = updateModel(t, m, textKey(keyJ))
	}
	_, line, _ := m.diffView.Current()
	requireEqual(
		t,
		line.Text != "more()",
		true,
		valueFormat,
		"diffSelection crossed hunk",
	)
}

func TestVimSequencesAndLayoutToggle(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		false,
		func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	m = updateModel(t, m, textKey(fixtureG))
	requireEqual(t, lineText(m), "more()", "G line=%q", lineText(m))
	m = updateModel(t, m, textKey(keyG))
	m = updateModel(t, m, textKey(keyG))
	requireEqual(
		t,
		lineText(m), fixturePackageMain,
		"gg line=%q",
		lineText(m),
	)
	m = updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		m.diffOptions.SideBySide && strings.Contains(m.render(), "│"),
		true,
		valueFormat,
		"split view not enabled",
	)
	m = updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		!m.diffOptions.SideBySide && slices.Equal(saved, []bool{true, false}),
		true,
		"saved=%v",
		saved,
	)
}

func TestSavedSideBySideCanBeDisabledInNarrowTerminal(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	var saved []bool
	m.configureSideBySide(
		true,
		func(enabled bool) error {
			saved = append(saved, enabled)
			return nil
		},
	)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: defaultWidth, Height: 20})
	m = updateModel(t, m, textKey(fixtureT))
	requireEqual(
		t,
		!m.diffOptions.SideBySide && slices.Equal(saved, []bool{false}),
		true,
		"sideBySide=%v saved=%v",
		m.diffOptions.SideBySide,
		saved,
	)
}

func TestResizeAcrossSideBySideThresholdPreservesFocusedLine(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m.configureSideBySide(true, nil)
	focusLine(t, &m, fixtureKeep)
	_, before, _ := m.diffView.Current()
	m = updateModel(t, m, tea.WindowSizeMsg{Width: defaultWidth, Height: 20})
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 120, Height: 20})
	_, got, ok := m.diffView.Current()
	requireEqual(
		t,
		ok && got == before, true,
		"focused line=%#v, want %#v",
		got,
		before,
	)
}

func TestZSequencesPositionCurrentLineInViewport(t *testing.T) {
	for _, test := range []struct {
		key       string
		alignment diffscreen.Alignment
	}{
		{"z", diffscreen.Center},
		{fixtureT, diffscreen.Top},
		{"b", diffscreen.Bottom},
	} {
		m := testModel(longModelPatch(), nil, nil)
		m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 9})
		want := diffscreen.New(
			longModelPatch(),
			diffscreen.Options{Dark: true},
		)
		want.Resize(100, 9)
		for range 10 {
			m.diffView.Move(diffscreen.NextLine)
			want.Move(diffscreen.NextLine)
		}
		want.Align(test.alignment)
		m = updateModel(t, m, textKey("z"))
		m = updateModel(t, m, textKey(test.key))
		requireEqual(
			t,
			m.diffView.Render(nil), want.Render(nil),
			"z%s did not align view",
			test.key,
		)
	}
}

func TestPendingKeyIsConsumedByNextKey(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey(fixtureG))
	last := lineText(m)
	m = updateModel(t, m, textKey(keyG))
	m = updateModel(t, m, textKey(fixtureH))
	m = updateModel(t, m, textKey(keyG))
	requireEqual(
		t, lineText(m), last, valueFormat, "pending prefix moved focus",
	)
	m = updateModel(t, m, textKey(keyG))
	requireEqual(
		t,
		lineText(m), fixturePackageMain,
		"gg line=%q",
		lineText(m),
	)
}

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	status := ansi.Strip(strings.Split(m.render(), newline)[m.height-2])
	requireEqual(
		t,
		strings.HasPrefix(status, "j/k/h/l move") &&
			strings.HasSuffix(status, "local changes"),
		true,
		"status=%q",
		status,
	)
	help := ansi.Strip(m.renderHelp())
	for _, binding := range []string{
		"Ctrl-w h/l/w",
		"zz/zt/zb",
		"n/N",
		"]f/[f",
		"Tab",
		fixtureT,
	} {
		requireEqual(
			t,
			strings.Contains(help, binding), true,
			"help missing %q",
			binding,
		)
	}
}

func TestStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: defaultWidth, Height: 9},
	)
	requireEqual(
		t,
		!strings.Contains(m.render(), "%"),
		true,
		valueFormat,
		"initial view shows progress",
	)
	m = updateModel(t, m, textKey("l"))
	requireEqual(
		t,
		!strings.Contains(m.render(), "%"),
		true,
		valueFormat,
		"horizontal scroll shows progress",
	)
	for range 10 {
		m = updateModel(t, m, textKey(keyJ))
	}
	requireEqual(
		t,
		strings.Contains(m.render(), progressSuffix),
		true,
		valueFormat,
		"scrolled view hides progress",
	)
	m = updateModel(t, m, textKey(fixtureG))
	requireEqual(
		t,
		strings.Contains(m.render(), "local changes (100%)"),
		true,
		valueFormat,
		"final view hides progress",
	)
}

func TestStatusHidesProgressWhenDiffFitsViewport(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 100})
	requireEqual(
		t,
		!strings.Contains(m.render(), progressSuffix),
		true,
		valueFormat,
		"fitting diff shows progress",
	)
}

func TestSideBySidePaneSwitchingUsesCtrlWSequences(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: 120, Height: defaultHeight},
	)
	m.setSideBySide(true)
	focusLine(t, &m, fixtureNew)
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, textKey(fixtureH))
	requireEqual(t, lineText(m), fixtureOld, "old pane line=%q", lineText(m))
	m = updateModel(t, m, controlKey('w'))
	m = updateModel(t, m, controlKey('w'))
	requireEqual(t, lineText(m), fixtureNew, "new pane line=%q", lineText(m))
}

func TestHorizontalScrollKeysMoveByStepAndReset(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(
		t,
		m,
		tea.WindowSizeMsg{Width: narrowTestWidth, Height: defaultHeight},
	)
	initial := m.diffView.Render(nil)
	want := diffscreen.New(longModelPatch(), diffscreen.Options{Dark: true})
	want.Resize(narrowTestWidth, defaultHeight)
	want.ScrollHorizontal(2 * horizontalScrollStep)
	m = updateModel(t, m, textKey("l"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	requireEqual(
		t,
		m.diffView.Render(nil), want.Render(nil),
		valueFormat,
		"right keys did not scroll by two steps",
	)
	m = updateModel(t, m, textKey(fixtureH))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	requireEqual(
		t,
		m.diffView.Render(nil), initial,
		valueFormat,
		"left keys did not restore initial offset",
	)
	m = updateModel(t, m, textKey("$"))
	requireEqual(
		t,
		m.diffView.Render(nil) != initial,
		true,
		valueFormat,
		"$ did not scroll",
	)
	m = updateModel(t, m, textKey("0"))
	requireEqual(
		t,
		m.diffView.Render(nil), initial,
		valueFormat,
		"0 did not restore initial offset",
	)
}
