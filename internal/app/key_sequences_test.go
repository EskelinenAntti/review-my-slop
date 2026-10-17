package app

import (
	"testing"
)

func TestKeymapFeed(t *testing.T) {
	for _, test := range []struct {
		name       string
		keys, want []string
	}{
		{"single keys", []string{"j", "q"}, []string{"j", "q"}},
		{"completed binding", []string{"g", "g"}, []string{"", "g g"}},
		{"retry unmatched", []string{"g", "j"}, []string{"", "j"}},
		{
			"retry starts another binding",
			[]string{"g", "z", "t"},
			[]string{"", "", "z t"},
		},
		{"consume unmatched", []string{"z", "q", "j"}, []string{"", "", "j"}},
		{
			"consume another prefix",
			[]string{"z", "g", "g", "g"},
			[]string{"", "", "", "g g"},
		},
		{
			"pane binding",
			[]string{"ctrl+w", "ctrl+w"},
			[]string{"", "ctrl+w ctrl+w"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertTestKeymapFeed(t, test.keys, test.want)
		})
	}
}

func assertTestKeymapFeed(t *testing.T, keys, want []string) {
	t.Helper()
	matcher := buildTestKeymapMatcher(
		keymapSequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true},
		keymapSequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
		keymapSequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}},
	)
	for index, key := range keys {
		if got := matcher.Feed(key); got != want[index] {
			t.Fatalf(
				"key %d (%q): got %q, want %q",
				index,
				key,
				got,
				want[index],
			)
		}
	}
}

func TestKeymapIndependentCopies(t *testing.T) {
	m := buildTestKeymapMatcher(keymapSequence{
		Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true,
	})
	m.Feed("g")
	cloned := m
	if got := m.Feed("j"); got != "j" {
		t.Fatalf("unmatched key: %q", got)
	}
	if got := cloned.Feed("g"); got != "g g" {
		t.Fatalf("copy: %q", got)
	}
	if got := m.Feed("g"); got != "" {
		t.Fatalf("original retained consumed prefix: %q", got)
	}
	if got := m.Feed("g"); got != "g g" {
		t.Fatalf("original: %q", got)
	}
}

func TestKeymapZeroValuePassesKeysThrough(t *testing.T) {
	var m keymapMatcher
	if got := m.Feed("j"); got != "j" {
		t.Fatalf("got %q", got)
	}
}
