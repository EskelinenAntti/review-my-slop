package app

import (
	"slices"
)

type keymapSequence struct {
	Prefix         string
	Keys           []string
	RetryUnmatched bool
}

type keymapMatcher struct {
	sequences map[string]keymapSequence
	pending   string
}

func (m *keymapMatcher) Feed(key string) string {
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

func newKeymapMatcher(sequences ...keymapSequence) keymapMatcher {
	m := keymapMatcher{sequences: make(
		map[string]keymapSequence,
		len(sequences),
	)}
	for _, sequence := range sequences {
		sequence.Keys = append([]string(nil), sequence.Keys...)
		m.sequences[sequence.Prefix] = sequence
	}
	return m
}
