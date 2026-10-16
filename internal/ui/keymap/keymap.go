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
	sequences map[string]Sequence
	pending   string
}

// NewKeyMatcher builds a matcher from the supplied prefix rules.
func NewKeyMatcher(sequences ...Sequence) Matcher {
	m := Matcher{sequences: make(map[string]Sequence, len(sequences))}
	for _, sequence := range sequences {
		sequence.Keys = append([]string(nil), sequence.Keys...)
		m.sequences[sequence.Prefix] = sequence
	}
	return m
}

// MatchKeySequence completes a binding with space-separated keys or returns an
// unprefixed key. An empty result means the key is pending or consumed.
func (m *Matcher) MatchKeySequence(key string) string {
	if m.pending != "" {
		prefix := m.pending
		m.pending = ""
		sequence := m.sequences[prefix]
		if slices.Contains(sequence.Keys, key) {
			return prefix + " " + key
		}
		if !sequence.RetryUnmatched {
			return ""
		}
	}
	if _, ok := m.sequences[key]; ok {
		m.pending = key
		return ""
	}
	return key
}
