package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := writeCommentDraftFile("existing comment", anchor)
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
	want := "existing comment\n\n```suggestion\nnew()\n```\n"
	if filepath.Ext(path) != ".md" || info.Mode().Perm() != 0o600 ||
		string(body) != want {
		t.Fatalf("path=%q mode=%o body=%q", path, info.Mode().Perm(), body)
	}
}

func TestCommentEditorSuggestionBehaviors(t *testing.T) {
	t.Run("escapes fence", func(t *testing.T) {
		anchor := comments.Anchor{
			QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"},
		}
		draft := formatCommentDraft("explain this", anchor)
		if !strings.Contains(
			draft,
			"`````suggestion",
		) || removeUnchangedSuggestion(
			draft,
			anchor.QuotedLines,
		) != "explain this" {
			t.Fatalf("draft = %q", draft)
		}
	})
	t.Run("only new version", func(t *testing.T) {
		anchor := comments.Anchor{
			QuotedLines: []string{" unchanged()", "-old()", "+new()"},
		}
		want := "comment\n\n```suggestion\nunchanged()\nnew()\n```\n"
		if got := formatCommentDraft("comment", anchor); got != want {
			t.Fatalf("draft = %q", got)
		}
	})
	t.Run("edited suggestion remains", func(t *testing.T) {
		body := "comment\n\n```suggestion\nbetter()\n```\n"
		quoted := []string{"-old()", "+new()"}
		if got := removeUnchangedSuggestion(body, quoted); got != body {
			t.Fatalf("body = %q", got)
		}
	})
}

func TestCommentDraftRoundTrip(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := formatCommentDraft("body", anchor)
	got := removeUnchangedSuggestion(draft, anchor.QuotedLines)
	if got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}
