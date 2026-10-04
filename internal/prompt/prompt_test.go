package prompt

import (
	"bytes"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestWritePreservesPromptFormat(t *testing.T) {
	var out bytes.Buffer
	item := comments.Comment{Anchor: comments.Anchor{
		FilePath: "main.go", OldStart: 10, OldEnd: 11, NewStart: 12, NewEnd: 13,
		QuotedLines: []string{"-old()", "+new()"},
	}, Body: "Handle the nil case."}
	if err := Write(&out, []comments.Comment{item}); err != nil {
		t.Fatal(err)
	}
	want := "New comments since last run:\n\n### 1. `main.go` (old lines 10-11, new lines 12-13)\n\n```diff\n-old()\n+new()\n```\nHandle the nil case.\n"
	if out.String() != want {
		t.Fatalf("prompt = %q, want %q", out.String(), want)
	}
}

func TestWriteEmptyQueue(t *testing.T) {
	var out bytes.Buffer
	if err := Write(&out, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "No pending review comments.\n"; got != want {
		t.Fatalf("prompt = %q, want %q", got, want)
	}
}
