package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestCreateCommentFileWritesSecureMarkdownDraft(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	anchor := review.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := CreateCommentFile("existing comment", anchor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".md" || info.Mode().Perm() != 0o600 {
		t.Fatalf("path=%q mode=%o", path, info.Mode().Perm())
	}
	if string(body) != "existing comment\n\n```suggestion\nnew()\n```\n" {
		t.Fatalf("body=%q", body)
	}
}

func TestCommentDraftSuggestionBehaviors(t *testing.T) {
	t.Run("escapes fence", func(t *testing.T) {
		anchor := review.Anchor{QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"}}
		draft := CommentDraft("explain this", anchor)
		if !strings.Contains(draft, "`````suggestion") || StripUnchangedSuggestion(draft, anchor.QuotedLines) != "explain this" {
			t.Fatalf("draft = %q", draft)
		}
	})
	t.Run("only new version", func(t *testing.T) {
		anchor := review.Anchor{QuotedLines: []string{" unchanged()", "-old()", "+new()"}}
		if got := CommentDraft("comment", anchor); got != "comment\n\n```suggestion\nunchanged()\nnew()\n```\n" {
			t.Fatalf("draft = %q", got)
		}
	})
	t.Run("edited suggestion remains", func(t *testing.T) {
		body := "comment\n\n```suggestion\nbetter()\n```\n"
		if got := StripUnchangedSuggestion(body, []string{"-old()", "+new()"}); got != body {
			t.Fatalf("body = %q", got)
		}
	})
}

func TestCommentCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	if err := CommentCommand("printf 'edited externally' >", path).Run(); err != nil {
		t.Fatal(err)
	}
	body, err := ReadCommentFile(path, review.Anchor{})
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestSourceCommandPassesLineAndPathAsArguments(t *testing.T) {
	command := SourceCommand("printf '%s %s'", "/tmp/repo with spaces/main.go", 2)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(output), "+2 /tmp/repo with spaces/main.go"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
