package diffscreen

import "testing"

func TestFilterANSIColors(t *testing.T) {
	tests := []struct {
		name, input, preserved, discarded string
	}{
		{
			name:      "plain text and resets",
			input:     "plain\x1b[m\x1b[0m",
			preserved: "plain\x1b[m\x1b[0m",
			discarded: "plain\x1b[m\x1b[0m",
		},
		{
			name:      "standard and bright colors",
			input:     "\x1b[1;31;45;92;105;39;49;4mtext",
			preserved: "\x1b[1;31;92;39;4mtext",
			discarded: "\x1b[1;4mtext",
		},
		{
			name:      "indexed colors",
			input:     "\x1b[48;5;123;38;5;45;4mtext",
			preserved: "\x1b[38;5;45;4mtext",
			discarded: "\x1b[4mtext",
		},
		{
			name:      "truecolor components that resemble SGR codes",
			input:     "\x1b[48;2;1;31;4;38;2;40;49;107;7mtext",
			preserved: "\x1b[38;2;40;49;107;7mtext",
			discarded: "\x1b[7mtext",
		},
		{
			name:      "color only sequences disappear",
			input:     "\x1b[41mbackground\x1b[32mforeground",
			preserved: "background\x1b[32mforeground",
			discarded: "backgroundforeground",
		},
		{
			name:      "empty parameters and unrelated attributes",
			input:     "\x1b[;3;48;5;1;;9mtext",
			preserved: "\x1b[;3;;9mtext",
			discarded: "\x1b[;3;;9mtext",
		},
		{
			name:      "incomplete truecolor",
			input:     "\x1b[1;48;2;255mtext\x1b[38;2;42m",
			preserved: "\x1b[1mtext\x1b[38;2;42m",
			discarded: "\x1b[1mtext",
		},
		{
			name:      "incomplete indexed color",
			input:     "\x1b[48;5mtext\x1b[38;5m",
			preserved: "text\x1b[38;5m",
			discarded: "text",
		},
		{
			name:      "missing color mode",
			input:     "\x1b[48mtext\x1b[38m",
			preserved: "text\x1b[38m",
			discarded: "text",
		},
		{
			name:      "unknown color mode retains following attributes",
			input:     "\x1b[48;1;38;3;4mtext",
			preserved: "\x1b[1;38;3;4mtext",
			discarded: "\x1b[1;3;4mtext",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertANSIColorPolicies(
				t, test.input, test.preserved, test.discarded,
			)
		})
	}
}

func assertANSIColorPolicies(t *testing.T, input, preserved, discarded string) {
	t.Helper()
	for _, policy := range []struct {
		name   string
		colors foregroundPolicy
		want   string
	}{
		{"preserve foreground", preserveForeground, preserved},
		{"discard foreground", discardForeground, discarded},
	} {
		t.Run(policy.name, func(t *testing.T) {
			got := filterANSIColors(input, policy.colors)
			if got != policy.want {
				t.Errorf("filterANSIColors(%q) = %q, want %q",
					input, got, policy.want,
				)
			}
		})
	}
}
