package patch

import (
	"testing"
)

func TestParseHunkBodyLineNumbers(t *testing.T) {
	lines, err := parseHunkBody(
		10,
		minimumWidth,
		[]byte(" context\n-old\n+new\n same\n\\ No newline at end of file\n"),
	)
	requireEqual(t, err == nil, true, fixtureV, err)
	want := []Line{
		{
			Kind:      Context,
			Text:      "context",
			OldNumber: 10,
			NewNumber: minimumWidth,
		},

		{Kind: Deletion, Text: "old", OldNumber: deletedSourceLine},

		{Kind: Addition, Text: "new", NewNumber: addedSourceLine},

		{
			Kind:      Context,
			Text:      "same",
			OldNumber: trailingOldSourceLine,
			NewNumber: trailingNewSourceLine,
		},
	}
	requireEqual(t, len(lines), len(want), "lines = %#v", lines)
	for i := range want {
		requireEqual(
			t,
			lines[i], want[i],
			"line %d = %#v, want %#v",
			i,
			lines[i],
			want[i],
		)
	}
}

func TestParseHunkBodyEscapesTerminalControls(t *testing.T) {
	lines, err := parseHunkBody(1, 1, []byte("+hello\x1b[2J\n"))
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		lines[0].Text, `hello\x1b[2J`,
		"text = %q",
		lines[0].Text,
	)
}

func FuzzParseHunkBody(f *testing.F) {
	f.Add([]byte("+hello\n-world\n"))
	f.Add([]byte(" context\n"))
	f.Fuzz(func(_ *testing.T, body []byte) {
		_, _ = parseHunkBody(1, 1, body)
	})
}
