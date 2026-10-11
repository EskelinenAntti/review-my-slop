package keymap

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

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
				{
					got := m.Feed(key)
					testutil.Requiref(t, got == test.want[i], "key %d (%q): got %q, want %q", i, key, got, test.want[i])
				}
			}
		})
	}
}

func TestIndependentCopies(t *testing.T) {
	m := New(Sequence{Prefix: "g", Keys: []string{"g"}, RetryUnmatched: true})
	m.Feed("g")
	copy := m
	{
		got := m.Feed("j")
		testutil.Requiref(t, got == "j", "unmatched key: %q", got)
	}
	{
		got := copy.Feed("g")
		testutil.Requiref(t, got == "g g", "copy: %q", got)
	}
	{
		got := m.Feed("g")
		testutil.Requiref(t, got == "", "original retained consumed prefix: %q", got)
	}
	{
		got := m.Feed("g")
		testutil.Requiref(t, got == "g g", "original: %q", got)
	}
}

func TestZeroValuePassesKeysThrough(t *testing.T) {
	var m Matcher
	{
		got := m.Feed("j")
		testutil.Requiref(t, got == "j", "got %q", got)
	}
}
