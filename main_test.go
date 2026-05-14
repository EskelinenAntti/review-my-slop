package main

import "testing"

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
