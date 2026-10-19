package patch

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestParseHunkBodyEdgeCases(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []Line
		err  string
	}{
		{name: "empty body", want: []Line{}},
		{
			name: "no trailing newline",
			body: "-old\n+new",
			want: []Line{
				{Kind: Deletion, Text: "old", OldNumber: 10},
				{Kind: Addition, Text: "new", NewNumber: 20},
			},
		},
		{
			name: "newline markers do not advance line numbers",
			body: "-old\n\\ No newline at end of file\n" +
				"+new\n\\ No newline at end of file\n same\n",
			want: []Line{
				{Kind: Deletion, Text: "old", OldNumber: 10},
				{Kind: Addition, Text: "new", NewNumber: 20},
				{Kind: Context, Text: "same", OldNumber: 11, NewNumber: 21},
			},
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
			name: "missing prefix",
			body: " context\n\n",
			err:  "malformed empty diff line",
		},
		{
			name: "unknown prefix",
			body: " context\n?bad\n",
			err:  "unexpected diff prefix '?'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHunkBody(10, 20, []byte(tt.body))
			var errorText string
			if err != nil {
				errorText = err.Error()
			}
			testutil.RequireEqual(t, errorText, tt.err,
				"error = %v, want %q", err, tt.err)
			testutil.RequireEqual(t, reflect.DeepEqual(got, tt.want), true,
				"lines = %#v, want %#v", got, tt.want)
		})
	}
}
