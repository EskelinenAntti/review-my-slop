package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestFilterANSIColors(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		preserve string
		discard  string
	}{
		{"plain text", "code", "code", "code"},
		{"resets", "\x1b[m\x1b[0m", "\x1b[m\x1b[0m", "\x1b[m\x1b[0m"},
		{
			"non-color parameters",
			"\x1b[29;50;89;98;99;108mcode",
			"\x1b[29;50;89;98;99;108mcode",
			"\x1b[29;50;89;98;99;108mcode",
		},
		{"empty parameter", "\x1b[;31mcode", "\x1b[;31mcode", "\x1b[mcode"},
		{
			"standard colors",
			"\x1b[1;31;44;39;49mcode",
			"\x1b[1;31;39mcode",
			"\x1b[1mcode",
		},
		{
			"bright colors",
			"\x1b[90;97;100;107mcode",
			"\x1b[90;97mcode",
			"code",
		},
		{
			"indexed colors",
			"\x1b[48;5;31;38;5;123;4mcode",
			"\x1b[38;5;123;4mcode",
			"\x1b[4mcode",
		},
		{
			"true colors",
			"\x1b[48;2;31;32;33;38;2;1;2;3;1mcode",
			"\x1b[38;2;1;2;3;1mcode",
			"\x1b[1mcode",
		},
		{"truncated background", "\x1b[48;2;31m", "", ""},
		{"truncated foreground", "\x1b[38;5m", "\x1b[38;5m", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.RequireEqual(t,
				filterANSIColors(tc.input, preserveForeground), tc.preserve,
				"preserved syntax foreground",
			)
			testutil.RequireEqual(t,
				filterANSIColors(tc.input, discardForeground), tc.discard,
				"discarded syntax foreground",
			)
		})
	}
}
