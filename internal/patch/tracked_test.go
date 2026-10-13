package patch

import (
	"fmt"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := parseHunkBody(
		10,
		20,
		[]byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"),
	)
	testutil.NoError(t, err)

	want := []Line{
		{Kind: Context, Text: "context", OldNumber: 10, NewNumber: 20},
		{Kind: Deletion, Text: "old", OldNumber: 11},
		{Kind: Addition, Text: "new", NewNumber: 21},
		{Kind: Context, Text: "same", OldNumber: 12, NewNumber: 22},
	}
	testutil.RequireEqual(t, len(lines), len(want),
		fmt.Sprintf("lines = %#v", lines),
	)

	for i := range want {
		testutil.RequireEqual(t, lines[i], want[i],
			fmt.Sprintf("line %d = %#v, want %#v", i, lines[i], want[i]),
		)
	}
}

func TestParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := parseHunkBody(1, 1, []byte("+hello\x1b[2J\n"))
	testutil.NoError(t, err)
	testutil.RequireEqual(t, lines[0].Text, `hello\x1b[2J`,
		fmt.Sprintf("text = %q", lines[0].Text),
	)
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(_ *testing.T, body []byte) {
		_, _ = parseHunkBody(1, 1, body)
	})
}
