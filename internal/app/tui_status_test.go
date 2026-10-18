package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestStatusShowsBasicBindingsAndHelpShowsCompleteKeyMap(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	status := ansi.Strip(strings.Split(m.render(), "\n")[m.dimensions.height-2])
	testutil.RequireEqual(t, strings.HasPrefix(status, "j/k/h/l move"), true,
		"status=%q", status,
	)
	testutil.RequireEqual(t, strings.HasSuffix(status, "local changes"), true,
		"status=%q", status,
	)

	m.mode = modeHelp
	help := ansi.Strip(m.render())
	for _, binding := range []string{
		"Ctrl-w h/l/w",
		"zz/zt/zb",
		"n/N",
		"]f/[f",
		"Tab",
		"t",
	} {
		testutil.RequireEqual(t, strings.Contains(help, binding), true,
			"help missing %q", binding,
		)
	}
}

func TestStatusShowsProgressOnlyAfterViewportMoves(t *testing.T) {
	m := testModel(longModelPatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 80, Height: 9})
	testutil.RequireEqual(t, !strings.Contains(m.render(), "%"), true, "%s",
		"initial view shows progress",
	)

	m = updateModel(t, m, textKey("l"))
	testutil.RequireEqual(t, !strings.Contains(m.render(), "%"), true, "%s",
		"horizontal scroll shows progress",
	)

	for range 10 {
		m = updateModel(t, m, textKey("j"))
	}
	testutil.RequireEqual(t, strings.Contains(m.render(), "%)"), true, "%s",
		"scrolled view hides progress",
	)

	m = updateModel(t, m, textKey("G"))
	testutil.RequireEqual(t,
		strings.Contains(m.render(), "local changes (100%)"), true, "%s",
		"final view hides progress",
	)
}

func TestStatusHidesProgressWhenPatchFitsViewport(t *testing.T) {
	m := testModel(coveragePatch(), nil, nil)
	m = updateModel(t, m, tea.WindowSizeMsg{Width: 100, Height: 100})
	testutil.RequireEqual(t, !strings.Contains(m.render(), "%)"), true, "%s",
		"fitting patch shows progress",
	)
}

func TestHeaderShowsAddedAndRemovedLineCounts(t *testing.T) {
	header := strings.SplitN(
		ansi.Strip(testModel(coveragePatch(), nil, nil).render()),
		"\n",
		2,
	)[0]
	testutil.RequireEqual(t, header, "review-my-slop  +2-1", "header = %q",
		header,
	)
}
