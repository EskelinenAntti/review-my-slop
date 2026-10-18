package app

import (
	"strings"
	"testing"
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
