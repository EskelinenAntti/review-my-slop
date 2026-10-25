package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

func TestBrowseActionsRespectPendingPrefixes(t *testing.T) {
	actions := []struct {
		key        string
		mode       mode
		quitting   bool
		sideBySide bool
	}{
		{key: "?", mode: modeHelp},
		{key: "/", mode: modeSearch},
		{key: "C", mode: modeComments},
		{key: "q", quitting: true},
		{key: "t", sideBySide: true},
	}
	prefixes := []struct {
		name  string
		key   tea.KeyPressMsg
		retry bool
	}{
		{name: "none", retry: true},
		{name: "g", key: textKey("g"), retry: true},
		{name: "z", key: textKey("z")},
		{name: "[", key: textKey("[")},
		{name: "]", key: textKey("]")},
		{name: "ctrl+w", key: controlKey('w')},
	}
	for _, prefix := range prefixes {
		for _, action := range actions {
			t.Run(prefix.name+"/"+action.key, func(t *testing.T) {
				m := testModel(coveragePatch(), nil, nil)
				if prefix.name != "none" {
					m = updateModel(t, m, prefix.key)
				}
				m = updateModel(t, m, textKey(action.key))
				if !prefix.retry {
					testutil.RequireEqual(t, m.mode, modeBrowse,
						"unmatched action must be consumed")
					testutil.RequireEqual(t, m.quitting, false,
						"consumed action must not quit")
					testutil.RequireEqual(t, m.diffOptions.SideBySide, false,
						"consumed action must not change layout")
					m = updateModel(t, m, textKey(action.key))
				}
				testutil.RequireEqual(t, m.mode, action.mode,
					"browse action mode")
				testutil.RequireEqual(t, m.quitting, action.quitting,
					"quit action")
				testutil.RequireEqual(t,
					m.diffOptions.SideBySide, action.sideBySide,
					"layout action")
			})
		}
	}
}

func TestNavigationMatcherRecognizesActions(t *testing.T) {
	check := func(binding string) {
		t.Helper()
		matcher := navigationMatcher()
		keys := strings.Split(binding, " ")
		for index, key := range keys {
			want := ""
			if index == len(keys)-1 {
				want = binding
			}
			testutil.RequireEqual(t, matcher.Feed(key), want,
				"binding %q, key %q", binding, key,
			)
		}
	}
	for binding := range navigationMotions {
		check(binding)
	}
	for binding := range navigationAlignments {
		check(binding)
	}
}

func TestNavigationMatcherDerivesNewSequences(t *testing.T) {
	navigationMotions["x y"] = diffscreen.NextLine
	navigationAlignments["x z"] = diffscreen.Top
	t.Cleanup(func() {
		delete(navigationMotions, "x y")
		delete(navigationAlignments, "x z")
	})
	for _, key := range []string{"y", "z"} {
		matcher := navigationMatcher()
		testutil.RequireEqual(t, matcher.Feed("x"), "", "prefix must wait")
		testutil.RequireEqual(t, matcher.Feed(key), "x "+key,
			"new binding must be recognized",
		)
	}
}

func TestNavigationMatcherUnmatchedContinuations(t *testing.T) {
	for _, prefix := range []string{"g", "z", "[", "]", "ctrl+w"} {
		t.Run(prefix, func(t *testing.T) {
			matcher := navigationMatcher()
			testutil.RequireEqual(t, matcher.Feed(prefix), "",
				"prefix must wait",
			)
			want := ""
			if prefix == "g" {
				want = "j"
			}
			testutil.RequireEqual(t, matcher.Feed("j"), want,
				"unmatched continuation",
			)
			testutil.RequireEqual(t, matcher.Feed("k"), "k",
				"pending prefix must be cleared",
			)
		})
	}
}
