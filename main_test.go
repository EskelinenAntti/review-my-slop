package main

import (
	"strings"
	"testing"
)

func TestParseDiff(t *testing.T) {
	diff := []byte(`diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -2 +2,2 @@
-old line
+new line
+another line
`)

	refs := parseDiff(diff)
	if len(refs) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(refs))
	}

	tests := []struct {
		index   int
		file    string
		line    int
		side    string
		content string
	}{
		{1, "a.go", 2, "old", "old line"},
		{2, "a.go", 2, "new", "new line"},
		{3, "a.go", 3, "new", "another line"},
	}

	for i, want := range tests {
		got := refs[i]
		if got.Index != want.index || got.File != want.file || got.Line != want.line || got.Side != want.side || got.Content != want.content {
			t.Fatalf("ref %d = %#v, want %#v", i, got, want)
		}
	}
}

func TestSelectRefs(t *testing.T) {
	refs := []lineRef{
		{Index: 1, File: "a", Line: 1},
		{Index: 2, File: "a", Line: 2},
		{Index: 3, File: "a", Line: 3},
	}

	selected, err := selectRefs(refs, "3,1-2,2")
	if err != nil {
		t.Fatal(err)
	}
	if len(selected) != 3 {
		t.Fatalf("expected 3 selections, got %d", len(selected))
	}
	for i, ref := range selected {
		if ref.Index != i+1 {
			t.Fatalf("selection %d index = %d", i, ref.Index)
		}
	}
}

func TestTruncateANSIPreservesEscapeSequences(t *testing.T) {
	got := truncateANSI("\x1b[31mabcdef\x1b[0m", 4)
	if !ansiRE.MatchString(got) {
		t.Fatalf("expected ansi escapes in %q", got)
	}
	if visibleLen(got) != 4 {
		t.Fatalf("visible length = %d, want 4", visibleLen(got))
	}
}

func TestBuildSelectionsUsesDisplayedRows(t *testing.T) {
	lines := []string{
		"\x1b[1mmain.go\x1b[0m --- Go",
		"\x1b[2m 10 \x1b[0m old text        \x1b[2m 10 \x1b[0m new text",
		"\x1b[2m .. \x1b[0m wrapped text     \x1b[2m .. \x1b[0m wrapped text",
		"\x1b[2m .. \x1b[0m                  \x1b[92;1m 12 \x1b[0m added only",
		"\x1b[91;1m 11 \x1b[0m removed       \x1b[92;1m 11 \x1b[0m added",
	}

	selections := buildSelections(lines, nil)
	if len(selections) != 2 {
		t.Fatalf("expected 2 selectable rows, got %d", len(selections))
	}
	if selections[0].LineIndex != 3 || selections[0].Ref.Line != 12 || selections[0].Ref.Side != "new" {
		t.Fatalf("first selection = %#v", selections[0])
	}
	if selections[1].LineIndex != 4 || selections[1].Ref.Line != 11 || selections[1].Ref.Side != "new" {
		t.Fatalf("second selection = %#v", selections[1])
	}
}

func TestHighlightANSIReappliesReverseAfterReset(t *testing.T) {
	got := highlightANSI("\x1b[31mred\x1b[0m plain", 12)
	if !strings.Contains(got, "\x1b[0m\x1b[7m") {
		t.Fatalf("expected reverse mode to resume after reset in %q", got)
	}
	if visibleLen(got) != 12 {
		t.Fatalf("visible length = %d, want 12", visibleLen(got))
	}
}
