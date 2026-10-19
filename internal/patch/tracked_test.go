package patch

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestParseHunkBodyBoundaries(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []Line
	}{
		{name: "empty body", want: []Line{}},
		{
			name: "marker only",
			body: "\\ No newline at end of file\n",
			want: []Line{},
		},
		{
			name: "no trailing newline",
			body: "+added",
			want: []Line{{Kind: Addition, Text: "added", NewNumber: 20}},
		},
		{
			name: "empty code lines",
			body: " \n-\n+\n",
			want: []Line{
				{Kind: Context, OldNumber: 10, NewNumber: 20},
				{Kind: Deletion, OldNumber: 11},
				{Kind: Addition, NewNumber: 21},
			},
		},
		{
			name: "markers do not advance line numbers",
			body: "-old\n\\ No newline at end of file\n" +
				"+new\n\\ No newline at end of file\n same\n",
			want: []Line{
				{Kind: Deletion, Text: "old", OldNumber: 10},
				{Kind: Addition, Text: "new", NewNumber: 20},
				{Kind: Context, Text: "same", OldNumber: 11, NewNumber: 21},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHunkBody(10, 20, []byte(tt.body))
			testutil.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("lines = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseHunkBodyRejectsMalformedLines(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "empty line", body: "\n", want: "malformed empty diff line"},
		{
			name: "unknown prefix",
			body: "?code\n",
			want: "unexpected diff prefix '?'",
		},
		{
			name: "malformed line after valid line",
			body: "+valid\n\n",
			want: "malformed empty diff line",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHunkBody(10, 20, []byte(tt.body))
			testutil.RequireEqual(t, err != nil, true,
				"error = nil, want %q", tt.want)
			testutil.RequireEqual(t, err.Error(), tt.want,
				"error = %v, want %q", err, tt.want)
			testutil.RequireEqual(t, got == nil, true,
				"lines = %#v, want nil on error", got)
		})
	}
}
