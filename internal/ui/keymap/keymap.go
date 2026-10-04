// Package keymap recognizes two-key bindings without executing UI actions.
package keymap

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

// New builds a matcher from the supplied prefix rules.
func New(sequences ...Sequence) Matcher {
	m := Matcher{sequences: make(map[string]Sequence, len(sequences))}
	for _, sequence := range sequences {
		sequence.Keys = append([]string(nil), sequence.Keys...)
		m.sequences[sequence.Prefix] = sequence
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
		for _, continuation := range sequence.Keys {
			if continuation == key {
				return prefix + " " + key
			}
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
