package keymap

import (
	"testing"
)

func TestFeed(t *testing.T) {
	for _, test := range []struct {
		name       string
		keys, want []string
	}{
		{"single keys", []string{keyJ, keyQ}, []string{keyJ, keyQ}},

		{"completed binding", []string{keyG, keyG}, []string{"", keyGG}},

		{"retry unmatched", []string{keyG, keyJ}, []string{"", keyJ}},

		{
			"retry starts another binding",
			[]string{keyG, keyZ, "t"},
			[]string{"", "", "z t"},
		},

		{
			"consume unmatched",
			[]string{keyZ, keyQ, keyJ},
			[]string{"", "", keyJ},
		},

		{
			"consume another prefix",
			[]string{keyZ, keyG, keyG, keyG},
			[]string{"", "", "", keyGG},
		},

		{
			"pane binding",
			[]string{"ctrl+w", "ctrl+w"},
			[]string{"", "ctrl+w ctrl+w"},
		},
	} {
		t.Run(
			test.name,
			func(t *testing.T) {
				m := New(
					Sequence{
						Prefix:         keyG,
						Keys:           []string{keyG},
						RetryUnmatched: true,
					},
					Sequence{Prefix: keyZ, Keys: []string{keyZ, "t", "b"}},
					Sequence{
						Prefix: "ctrl+w",
						Keys:   []string{"h", "l", "ctrl+w"},
					},
				)
				for i, key := range test.keys {
					{
						got := m.Feed(key)
						requireEqual(
							t,
							got, test.want[i],
							"key %d (%q): got %q, want %q",
							i,
							key,
							got,
							test.want[i],
						)
					}
				}
			},
		)
	}
}

func TestIndependentCopies(t *testing.T) {
	m := New(
		Sequence{Prefix: keyG, Keys: []string{keyG}, RetryUnmatched: true},
	)
	m.Feed(keyG)
	clone := m
	{
		got := m.Feed(keyJ)
		requireEqual(t, got, keyJ, "unmatched key: %q", got)
	}
	{
		got := clone.Feed(keyG)
		requireEqual(t, got, keyGG, "copy: %q", got)
	}
	{
		got := m.Feed(keyG)
		requireEqual(t, got, "", "original retained consumed prefix: %q", got)
	}
	{
		got := m.Feed(keyG)
		requireEqual(t, got, keyGG, "original: %q", got)
	}
}

func TestZeroValuePassesKeysThrough(t *testing.T) {
	var m Matcher
	{
		got := m.Feed(keyJ)
		requireEqual(t, got, keyJ, "got %q", got)
	}
}
