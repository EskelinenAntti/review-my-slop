package keymap

import "testing"

func TestMatchKeySequence(t *testing.T) {
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
			assertKeySequence(t, test.keys, test.want)
		})
	}
}

func assertKeySequence(t *testing.T, keys, want []string) {
	t.Helper()
	m := NewKeyMatcher(
		Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true},
		Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
		Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}},
	)
	for i, key := range keys {
		if got := m.MatchKeySequence(key); got != want[i] {
			t.Fatalf("key %d (%q): got %q, want %q", i, key, got, want[i])
		}
	}
}

func TestIndependentCopies(t *testing.T) {
	m := NewKeyMatcher(
		Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true},
	)
	m.MatchKeySequence("g")
	cloned := m
	if got := m.MatchKeySequence("j"); got != "j" {
		t.Fatalf("unmatched key: %q", got)
	}
	if got := cloned.MatchKeySequence("g"); got != "g g" {
		t.Fatalf("copy: %q", got)
	}
	if got := m.MatchKeySequence("g"); got != "" {
		t.Fatalf("original retained consumed prefix: %q", got)
	}
	if got := m.MatchKeySequence("g"); got != "g g" {
		t.Fatalf("original: %q", got)
	}
}

func TestZeroValuePassesKeysThrough(t *testing.T) {
	var m Matcher
	if got := m.MatchKeySequence("j"); got != "j" {
		t.Fatalf("got %q", got)
	}
}
