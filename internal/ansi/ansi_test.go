package ansi

import (
	"strings"
	"testing"
)

func TestTruncatePreservesEscapeSequences(t *testing.T) {
	got := Truncate("\x1b[31mabcdef\x1b[0m", 4)
	if !strings.Contains(got, "\x1b[31m") {
		t.Fatalf("expected ansi escapes in %q", got)
	}
	if VisibleLen(got) != 4 {
		t.Fatalf("visible length = %d, want 4", VisibleLen(got))
	}
}

func TestStripRemovesEscapeSequences(t *testing.T) {
	got := Strip("\x1b[31mred\x1b[0m plain")
	if got != "red plain" {
		t.Fatalf("Strip result = %q, want plain text", got)
	}
}

func TestEndRejectsNonEscape(t *testing.T) {
	if got := End("abc", 0); got != -1 {
		t.Fatalf("End = %d, want -1", got)
	}
}
