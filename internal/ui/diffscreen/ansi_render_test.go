package diffscreen

import (
	"slices"
	"testing"
)

func TestFilterANSIColors(t *testing.T) {
	for _, test := range []struct {
		name, input, preserved, discarded string
	}{
		{"plain text", "plain", "plain", "plain"},
		{"empty reset", "\x1b[m", "\x1b[m", "\x1b[m"},
		{"reset and attributes", "\x1b[0;1;4m", "\x1b[0;1;4m", "\x1b[0;1;4m"},
		{"standard colors", "\x1b[31;44;1m", "\x1b[31;1m", "\x1b[1m"},
		{"bright colors", "\x1b[97;107m", "\x1b[97m", ""},
		{"default colors", "\x1b[39;49m", "\x1b[39m", ""},
		{"range boundaries", "\x1b[29;30;40;50;89;90;98;100;108m",
			"\x1b[29;30;50;89;90;98;108m", "\x1b[29;50;89;98;108m"},
		{"indexed colors", "\x1b[48;5;123;38;5;123;1m",
			"\x1b[38;5;123;1m", "\x1b[1m"},
		{"truecolor", "\x1b[48;2;255;0;0;38;2;1;2;3;4m",
			"\x1b[38;2;1;2;3;4m", "\x1b[4m"},
		// Foreground payloads currently undergo parameter-wise filtering too.
		// Preserve that behavior rather than changing it in a refactor.
		{"indexed payload matching background code", "\x1b[38;5;45;1m",
			"\x1b[38;5;1m", "\x1b[1m"},
		{"truecolor payload matching color codes", "\x1b[38;2;40;90;100;4m",
			"\x1b[38;2;90;4m", "\x1b[4m"},
		{"missing color mode", "\x1b[48m", "", ""},
		{"truncated truecolor", "\x1b[48;2;255m", "", ""},
		{"truecolor mode without channels", "\x1b[48;2m", "", ""},
		{"indexed mode without index", "\x1b[48;5m", "", ""},
		{"adjacent background payloads", "\x1b[48;5;48;48;2;38;48;49;4m",
			"\x1b[4m", "\x1b[4m"},
		{"truncated indexed foreground", "\x1b[38;5m", "\x1b[38;5m", ""},
		{"unknown color mode", "\x1b[48;3;1m", "\x1b[3;1m", "\x1b[3;1m"},
		{"empty parameter", "\x1b[;44;1m", "\x1b[;1m", "\x1b[;1m"},
		{"non-SGR escape", "\x1b[2Ktext", "\x1b[2Ktext", "\x1b[2Ktext"},
		{"multiple sequences", "\x1b[41mone\x1b[32mtwo\x1b[0m",
			"one\x1b[32mtwo\x1b[0m", "onetwo\x1b[0m"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, policy := range []struct {
				name   string
				colors foregroundPolicy
				want   string
			}{
				{"preserve foreground", preserveForeground, test.preserved},
				{"discard foreground", discardForeground, test.discarded},
			} {
				t.Run(policy.name, func(t *testing.T) {
					got := filterANSIColors(test.input, policy.colors)
					if got != policy.want {
						t.Errorf("filterANSIColors(%q) = %q, want %q",
							test.input, got, policy.want)
					}
				})
			}
		})
	}
}

func TestFilterSGRPartsLeavesInputUnchanged(t *testing.T) {
	parts := []string{"44", "1", "48", "5", "123", "4"}
	original := slices.Clone(parts)
	got := filterSGRParts(parts, preserveForeground)
	if !slices.Equal(got, []string{"1", "4"}) {
		t.Fatalf("filterSGRParts() = %v, want [1 4]", got)
	}
	if !slices.Equal(parts, original) {
		t.Errorf("filterSGRParts mutated input: %v", parts)
	}
}
