package diffscreen

import (
	"slices"
	"testing"
)

func TestSGRParameters(t *testing.T) {
	type parameter struct {
		code int
		text string
	}
	tests := []struct {
		name, input string
		want        []parameter
	}{
		{
			name:  "attributes and empty parameters",
			input: ";1;;4",
			want:  []parameter{{-1, ""}, {1, "1"}, {-1, ""}, {4, "4"}},
		},
		{
			name:  "color components remain grouped",
			input: "48;2;1;31;4;38;5;49;7",
			want:  []parameter{{48, "48;2;1;31;4"}, {38, "38;5;49"}, {7, "7"}},
		},
		{
			name:  "incomplete truecolor",
			input: "1;38;2;42",
			want:  []parameter{{1, "1"}, {38, "38;2;42"}},
		},
		{
			name:  "incomplete indexed color",
			input: "48;5",
			want:  []parameter{{48, "48;5"}},
		},
		{
			name:  "missing color mode",
			input: "38",
			want:  []parameter{{38, "38"}},
		},
		{
			name:  "unknown color mode is a separate attribute",
			input: "48;1;38;3;4",
			want: []parameter{
				{48, "48"}, {1, "1"}, {38, "38"}, {3, "3"}, {4, "4"},
			},
		},
		{
			name:  "original spelling is preserved",
			input: "038;5;049;invalid",
			want:  []parameter{{38, "038;5;049"}, {-1, "invalid"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []parameter
			for code, text := range sgrParameters(test.input) {
				got = append(got, parameter{code, text})
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("sgrParameters(%q) = %v, want %v",
					test.input, got, test.want,
				)
			}
		})
	}
}

func TestSGRParametersStopsWhenYieldReturnsFalse(t *testing.T) {
	count := 0
	sgrParameters("1;38;2;40;49;107;4")(func(int, string) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("yield called %d times, want 1", count)
	}
}
