// Package keymap recognizes two-key bindings without executing UI actions.
package keymap

import "slices"

// Sequence defines the accepted keys following a prefix. RetryUnmatched causes
// an invalid continuation to be processed as a fresh key instead of consumed.
type Sequence struct {
	Prefix         string
	Keys           []string
	RetryUnmatched bool
}

// Matcher retains pending input. Copies have independent pending state.
type Matcher struct {
	sequences map[string]continuation
	pending   string
}

type continuation struct {
	keys           []string
	retryUnmatched bool
}

// New builds a matcher from the supplied prefix rules.
func New(sequences ...Sequence) Matcher {
	m := Matcher{sequences: make(map[string]continuation, len(sequences))}
	for _, binding := range sequences {
		m.sequences[binding.Prefix] = continuation{
			keys:           slices.Clone(binding.Keys),
			retryUnmatched: binding.RetryUnmatched,
		}
	}
	return m
}

// Feed returns a completed binding with keys separated by a space, or an
// unprefixed key. An empty result means pending or consumed input.
func (m *Matcher) Feed(key string) string {
	if m.pending != "" {
		prefix := m.pending
		m.pending = ""
		sequence := m.sequences[prefix]
		if slices.Contains(sequence.keys, key) {
			return prefix + " " + key
		}
		if !sequence.retryUnmatched {
			return ""
		}
	}
	if _, ok := m.sequences[key]; ok {
		m.pending = key
		return ""
	}
	return key
}
