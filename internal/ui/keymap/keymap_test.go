package keymap

import "testing"

func TestFeed(t *testing.T) {
	for _, test := range []struct {
		name       string
		keys, want []string
	}{
		{"single keys", []string{"j", "q"}, []string{"j", "q"}},
		{"completed binding", []string{"g", "g"}, []string{"", "g g"}},
		{"retry unmatched", []string{"g", "j"}, []string{"", "j"}},
		{"retry starts another binding", []string{"g", "z", "t"}, []string{"", "", "z t"}},
		{"consume unmatched", []string{"z", "q", "j"}, []string{"", "", "j"}},
		{"consume another prefix", []string{"z", "g", "g", "g"}, []string{"", "", "", "g g"}},
		{"pane binding", []string{"ctrl+w", "ctrl+w"}, []string{"", "ctrl+w ctrl+w"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := New(
				Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true},
				Sequence{Prefix: "z", Keys: []string{"z", "t", "b"}},
				Sequence{Prefix: "ctrl+w", Keys: []string{"h", "l", "ctrl+w"}},
			)
			for i, key := range test.keys {
				if got := m.Feed(key); got != test.want[i] {
					t.Fatalf("key %d (%q): got %q, want %q", i, key, got, test.want[i])
				}
			}
		})
	}
}

func TestIndependentCopies(t *testing.T) {
	m := New(Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true})
	m.Feed("g")
	copy := m
	if got := m.Feed("j"); got != "j" {
		t.Fatalf("unmatched key: %q", got)
	}
	if got := copy.Feed("g"); got != "g g" {
		t.Fatalf("copy: %q", got)
	}
	if got := m.Feed("g"); got != "" {
		t.Fatalf("original retained consumed prefix: %q", got)
	}
	if got := m.Feed("g"); got != "g g" {
		t.Fatalf("original: %q", got)
	}
}

func TestZeroValuePassesKeysThrough(t *testing.T) {
	var m Matcher
	if got := m.Feed("j"); got != "j" {
		t.Fatalf("got %q", got)
	}
}
