package app

import (
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"github.com/eskelinenantti/review-my-slop/internal/ui/diffscreen"
)

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
