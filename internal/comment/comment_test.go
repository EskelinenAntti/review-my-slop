package comment_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comment"
)

func TestWritePromptDescribesAnchorsAndQuotesLines(t *testing.T) {
	var output bytes.Buffer
	err := comment.WritePrompt(&output, []comment.Comment{{
		Anchor: comment.Anchor{
			FilePath:    "main.go",
			OldStart:    10,
			OldEnd:      11,
			NewStart:    12,
			NewEnd:      12,
			QuotedLines: []string{"-old", "+new"},
		},
		Body: "  Fix this.  ",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "New comments since last run:\n\n### 1. `main.go` (old lines 10-11, new line 12)\n\n```diff\n-old\n+new\n```\nFix this.\n"
	if output.String() != want {
		t.Fatalf("prompt = %q, want %q", output.String(), want)
	}
}

func TestWritePromptHandlesEmptyInbox(t *testing.T) {
	var output bytes.Buffer
	if err := comment.WritePrompt(&output, nil); err != nil {
		t.Fatal(err)
	}
	if output.String() != "No pending review comments.\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestDraftAndStripSuggestionRoundTrip(t *testing.T) {
	anchor := comment.Anchor{QuotedLines: []string{" unchanged", "-old", "+new"}}
	draft := comment.Draft("Keep reviewing this.", anchor)
	want := "Keep reviewing this.\n\n```suggestion\nunchanged\nnew\n```\n"
	if draft != want {
		t.Fatalf("draft = %q, want %q", draft, want)
	}
	if got := comment.StripSuggestion(draft, anchor.QuotedLines); got != "Keep reviewing this." {
		t.Fatalf("stripped draft = %q", got)
	}
}

func TestDraftUsesFenceLongerThanQuotedBackticks(t *testing.T) {
	anchor := comment.Anchor{QuotedLines: []string{"+````go", "+fmt.Println(\"hello\")", "+````"}}
	draft := comment.Draft("Explain this.", anchor)
	if !strings.Contains(draft, "`````suggestion") {
		t.Fatalf("draft did not lengthen fence: %q", draft)
	}
	if comment.StripSuggestion(draft, anchor.QuotedLines) != "Explain this." {
		t.Fatal("unchanged suggestion was not stripped")
	}
}

func TestStripSuggestionKeepsEditedSuggestion(t *testing.T) {
	body := "Comment.\n\n```suggestion\nbetter()\n```\n"
	if got := comment.StripSuggestion(body, []string{"-old()", "+new()"}); got != body {
		t.Fatalf("edited suggestion changed: %q", got)
	}
}

func TestCreateDraftUsesPrivateFileAndReadRemovesIt(t *testing.T) {
	state := t.TempDir()
	anchor := comment.Anchor{QuotedLines: []string{"+new()"}}
	path, err := comment.CreateDraft(state, "Existing", anchor)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("draft mode = %o", info.Mode().Perm())
	}
	body, err := comment.ReadDraft(path, anchor, nil)
	if err != nil {
		t.Fatal(err)
	}
	if body != "Existing" {
		t.Fatalf("body = %q", body)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("draft still exists: %v", err)
	}
}

func TestCommandsQuotePaths(t *testing.T) {
	commentCommand := comment.CommentCommand("editor", "/tmp/with spaces/it's.md")
	if got, want := strings.Join(commentCommand.Args, "\x00"), "sh\x00-c\x00editor '/tmp/with spaces/it'\"'\"'s.md'"; got != want {
		t.Fatalf("comment command = %q, want %q", got, want)
	}
	sourceCommand := comment.SourceCommand("editor", "/tmp/main.go", 12)
	if got, want := strings.Join(sourceCommand.Args, "\x00"), "sh\x00-c\x00editor +12 '/tmp/main.go'"; got != want {
		t.Fatalf("source command = %q, want %q", got, want)
	}
}
