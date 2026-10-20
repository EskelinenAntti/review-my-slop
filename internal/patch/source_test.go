package patch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestVisibleContent(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		source string
		text   string
	}{
		{name: "empty"},
		{
			name:   "unicode",
			input:  "hello 世界 🌍",
			source: "hello 世界 🌍",
			text:   "hello 世界 🌍",
		},
		{
			name:   "newlines and tabs",
			input:  "\nfirst\tline\nsecond\n",
			source: "\nfirst\tline\nsecond\n",
			text:   `\nfirst` + "\t" + `line\nsecond\n`,
		},
		{
			name:   "controls",
			input:  "\x00\x01\x1b\r\x1f\x7f",
			source: `\x00\x01\x1b\r\x1f\x7f`,
			text:   `\x00\x01\x1b\r\x1f\x7f`,
		},
		{
			name:   "literal escapes",
			input:  `\n\r\x1b`,
			source: `\n\r\x1b`,
			text:   `\n\r\x1b`,
		},
		{
			name:   "invalid UTF-8",
			input:  "\xff\n",
			source: "�\n",
			text:   `�\n`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, text := visibleSource(tt.input), visibleText(tt.input)
			testutil.CheckEqual(t, source, tt.source,
				"visibleSource() = %q, want %q", source, tt.source)
			testutil.CheckEqual(t, text, tt.text,
				"visibleText() = %q, want %q", text, tt.text)
		})
	}
}

func BenchmarkVisibleText(b *testing.B) {
	value := strings.Repeat("source\t世界\n", 100)
	b.ReportAllocs()
	for b.Loop() {
		visibleText(value)
	}
}

func TestReadLimitedFileEnforcesLimitAfterStat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("small"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", maxFileBytes+1)
	if err := os.WriteFile(path, []byte(large), 0o600); err != nil {
		t.Fatal(err)
	}

	content, oversized, err := readLimitedFile(path, info)
	if err != nil {
		t.Fatal(err)
	}
	if !oversized {
		t.Fatal("oversized file was not reported")
	}
	if len(content) != 0 {
		t.Fatalf("oversized content length = %d, want 0", len(content))
	}
}
