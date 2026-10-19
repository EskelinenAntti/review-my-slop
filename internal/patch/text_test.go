package patch

import (
	"reflect"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestVisibleTextPreservesLayoutOnlyForSource(t *testing.T) {
	tests := []struct {
		name, input, source, text string
	}{
		{name: "empty"},
		{
			name: "unicode", input: "café 日本語",
			source: "café 日本語", text: "café 日本語",
		},
		{
			name: "layout", input: "a\t b\n\n",
			source: "a\t b\n\n", text: "a\t b\\n\\n",
		},
		{
			name: "controls", input: "\x00\x01\x1b[31m\r\x7f",
			source: `\x00\x01\x1b[31m\r\x7f`, text: `\x00\x01\x1b[31m\r\x7f`,
		},
		{name: "invalid UTF-8", input: "\xff", source: "�", text: "�"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleSource(tt.input)
			testutil.CheckEqual(t, got, tt.source,
				"visibleSource() = %q, want %q", got, tt.source)
			got = visibleText(tt.input)
			testutil.CheckEqual(t, got, tt.text,
				"visibleText() = %q, want %q", got, tt.text)
		})
	}
}

func TestVisibleStringsDoesNotModifyInput(t *testing.T) {
	input := []string{"a\nb", "\x1b[31m"}
	want := []string{`a\nb`, `\x1b[31m`}
	if got := visibleStrings(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("visibleStrings() = %q, want %q", got, want)
	}
	if input[0] != "a\nb" || input[1] != "\x1b[31m" {
		t.Fatalf("input modified: %q", input)
	}
}

func TestSplitSourceLines(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{input: ""},
		{input: "\n"},
		{input: "one", want: []string{"one"}},
		{input: "one\n", want: []string{"one"}},
		{input: "one\n\n", want: []string{"one", ""}},
		{input: "\none\ntwo", want: []string{"", "one", "two"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitSourceLines(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitSourceLines() = %q, want %q",
					got, tt.want)
			}
		})
	}
}
