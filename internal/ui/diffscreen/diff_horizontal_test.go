package diffscreen

import (
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRenderedLineWidth(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  entry
		want int
	}{
		{name: "empty"},
		{
			name: "styled unified text",
			row:  entry{text: "\x1b[31mhello\x1b[0m"},
			want: 5,
		},
		{
			name: "styled split text with tabs",
			row: entry{
				left:  "\x1b[31ma\t\x1b[0m",
				right: "\x1b[38;2;1;2;3mb\x1b[0m",
			},
			want: 6,
		},
		{
			name: "wide and combining characters",
			row:  entry{text: "\x1b[32m界e\u0301\x1b[0m"},
			want: 3,
		},
		{
			name: "hyperlink",
			row: entry{
				text: "\x1b]8;;https://example.com\x1b\\link\x1b]8;;\x1b\\",
			},
			want: 4,
		},
		{
			name: "multiline metadata",
			row:  entry{text: "\x1b[31mlong\x1b[0m\nx"},
			want: 4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.RequireEqual(t, renderedLineWidth(tc.row), tc.want,
				"rendered line width")
		})
	}
}
