package patch

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestParseHunkBodyEdgeCases(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []Line
	}{
		{name: "empty", want: []Line{}},
		{
			name: "empty source lines",
			body: " \n-\n+\n",
			want: []Line{
				{Kind: Context, OldNumber: 10, NewNumber: 20},
				{Kind: Deletion, OldNumber: 11},
				{Kind: Addition, NewNumber: 21},
			},
		},
		{
			name: "newline markers do not advance line numbers",
			body: "-old\n\\ No newline at end of file\n" +
				"+new\n\\ No newline at end of file\n same",
			want: []Line{
				{Kind: Deletion, Text: "old", OldNumber: 10},
				{Kind: Addition, Text: "new", NewNumber: 20},
				{Kind: Context, Text: "same", OldNumber: 11, NewNumber: 21},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHunkBody(10, 20, []byte(tt.body))
			testutil.NoError(t, err)
			testutil.RequireEqual(t, reflect.DeepEqual(got, tt.want), true,
				"lines = %#v, want %#v", got, tt.want)
		})
	}
}

func TestParseHunkBodyRejectsMalformedLines(t *testing.T) {
	for _, body := range []string{"\n", "+valid\n\n", "?invalid\n"} {
		t.Run(body, func(t *testing.T) {
			lines, err := parseHunkBody(1, 1, []byte(body))
			if err == nil || lines != nil {
				t.Fatalf("parseHunkBody = %#v, %v; want nil lines and an error",
					lines, err)
			}
		})
	}
}
