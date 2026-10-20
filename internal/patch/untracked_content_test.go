package patch

import (
	"reflect"
	"testing"
)

func TestParseUntrackedContent(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		source  string
		header  string
		lines   []Line
	}{
		{
			name:   "empty",
			header: "@@ -0,0 +1,0 @@",
			lines:  []Line{},
		},
		{
			name:    "text with trailing newline",
			content: "first\nsecond\n",
			source:  "first\nsecond\n",
			header:  "@@ -0,0 +1,2 @@",
			lines: []Line{
				{Kind: Addition, Text: "first", NewNumber: 1},
				{Kind: Addition, Text: "second", NewNumber: 2},
			},
		},
		{
			name:    "controls without trailing newline",
			content: "\ttext\r\x1b",
			source:  "\ttext\\r\\x1b",
			header:  "@@ -0,0 +1,1 @@",
			lines: []Line{
				{Kind: Addition, Text: "\ttext\\r\\x1b", NewNumber: 1},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := parseUntrackedContent("new\nfile.txt", []byte(test.content))
			want := File{
				NewPath:     "new\nfile.txt",
				DisplayPath: `new\nfile.txt`,
				Metadata:    []string{"untracked file"},
				NewSource:   test.source,
				Hunks:       []Hunk{{Header: test.header, Lines: test.lines}},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("file = %#v, want %#v", got, want)
			}
		})
	}
}

func TestParseUntrackedBinaryContent(t *testing.T) {
	got := parseUntrackedContent("binary.dat", []byte("text\x00data"))
	want := File{
		NewPath:     "binary.dat",
		DisplayPath: "binary.dat",
		Metadata:    []string{"untracked binary file"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("file = %#v, want %#v", got, want)
	}
}
