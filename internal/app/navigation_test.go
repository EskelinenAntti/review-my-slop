package app

import (
	tea "charm.land/bubbletea/v2"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func TestNavigationMatcherDerivesMultiKeyBindings(t *testing.T) {
	for binding := range navigationMotions {
		assertNavigationBinding(t, binding)
	}
	for binding := range navigationAlignments {
		assertNavigationBinding(t, binding)
	}
}

func assertNavigationBinding(t *testing.T, binding string) {
	t.Helper()
	keys := strings.Fields(binding)
	if len(keys) != 2 {
		return
	}

	matcher := navigationMatcher()
	if got := matcher.Feed(keys[0]); got != "" {
		t.Fatalf(
			"first key %q for %q: got %q, want pending",
			keys[0], binding, got,
		)
	}
	if got := matcher.Feed(keys[1]); got != binding {
		t.Fatalf("binding %q: got %q", binding, got)
	}
}

func TestNavigationMatcherKeepsPrefixFallbackBehavior(t *testing.T) {
	matcher := navigationMatcher()
	if got := matcher.Feed("g"); got != "" {
		t.Fatalf("g prefix: got %q", got)
	}
	if got := matcher.Feed("j"); got != "j" {
		t.Fatalf("unmatched g continuation: got %q, want j", got)
	}

	if got := matcher.Feed("z"); got != "" {
		t.Fatalf("z prefix: got %q", got)
	}
	if got := matcher.Feed("q"); got != "" {
		t.Fatalf("unmatched z continuation: got %q, want consumed", got)
	}
	if got := matcher.Feed("j"); got != "j" {
		t.Fatalf("key after consumed prefix: got %q, want j", got)
	}
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
			"z%s did not align view", test.key,
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
	testutil.RequireEqual(t, lineText(m), last, "%s",
		"pending prefix moved focus",
	)

	m = updateModel(t, m, textKey("g"))
	testutil.RequireEqual(t, lineText(m), "package main", "gg line=%q",
		lineText(m),
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
	testutil.RequireEqual(t, m.diffView.Render(nil), want.Render(nil), "%s",
		"right keys did not scroll by two steps",
	)

	m = updateModel(t, m, textKey("h"))
	m = updateModel(t, m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}))
	testutil.RequireEqual(t, m.diffView.Render(nil), initial, "%s",
		"left keys did not restore initial offset",
	)

	m = updateModel(t, m, textKey("$"))
	testutil.RequireEqual(t, m.diffView.Render(nil) != initial, true, "%s",
		"$ did not scroll",
	)

	m = updateModel(t, m, textKey("0"))
	testutil.RequireEqual(t, m.diffView.Render(nil), initial, "%s",
		"0 did not restore initial offset",
	)
}

func TestSelectionCannotCrossHunk(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, textKey("v"))
	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	_, line, _ := m.diffView.Current()
	testutil.RequireEqual(t, line.Text != "more()", true, "%s",
		"diffSelection crossed hunk",
	)
}
