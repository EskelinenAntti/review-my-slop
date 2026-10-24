package patch

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestParseHunkBodyEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name             string
		oldLine, newLine int32
		body             string
		want             []Line
	}{
		{
			name: "empty body",
			want: []Line{},
		},
		{
			name: "blank source lines", oldLine: 10, newLine: 20,
			body: " \n-\n+\n \n",
			want: []Line{
				{Kind: Context, OldNumber: 10, NewNumber: 20},
				{Kind: Deletion, OldNumber: 11},
				{Kind: Addition, NewNumber: 21},
				{Kind: Context, OldNumber: 12, NewNumber: 22},
			},
		},
		{
			name: "addition-only hunk", newLine: 1,
			body: "+first\n+second",
			want: []Line{
				{Kind: Addition, Text: "first", NewNumber: 1},
				{Kind: Addition, Text: "second", NewNumber: 2},
			},
		},
		{
			name: "deletion-only hunk", oldLine: 1,
			body: "-first\n-second\n",
			want: []Line{
				{Kind: Deletion, Text: "first", OldNumber: 1},
				{Kind: Deletion, Text: "second", OldNumber: 2},
			},
		},
		{
			name:    "newline markers do not advance counters",
			oldLine: 10, newLine: 20,
			body: "-old\n\\ No newline at end of file\n" +
				"+new\n\\ No newline at end of file\n same\n",
			want: []Line{
				{Kind: Deletion, Text: "old", OldNumber: 10},
				{Kind: Addition, Text: "new", NewNumber: 20},
				{Kind: Context, Text: "same", OldNumber: 11, NewNumber: 21},
			},
		},
		{
			name: "marker-only body",
			body: "\\ No newline at end of file\n",
			want: []Line{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := parseHunkBody(tc.oldLine, tc.newLine, []byte(tc.body))
			testutil.NoError(t, err)
			testutil.RequireEqual(t, reflect.DeepEqual(lines, tc.want), true,
				"lines = %#v, want %#v", lines, tc.want,
			)
		})
	}
}

func TestParseHunkBodyRejectsMalformedLines(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantError string
	}{
		{"missing prefix", "\n", "malformed empty diff line"},
		{
			"empty line after valid line", "+valid\n\n",
			"malformed empty diff line",
		},
		{
			"invalid prefix after valid line", "+valid\n?invalid\n",
			"unexpected diff prefix '?'",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines, err := parseHunkBody(1, 1, []byte(tc.body))
			testutil.RequireEqual(t, err != nil, true,
				"lines = %#v, error = %v", lines, err,
			)
			testutil.RequireEqual(t, err.Error(), tc.wantError,
				"error = %v", err,
			)
			testutil.RequireEqual(t, lines == nil, true,
				"malformed input returned partial lines: %#v", lines,
			)
		})
	}
}
