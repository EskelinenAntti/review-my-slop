package editor

import (
	"context"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := createCommentFile("existing comment", anchor)
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
	if filepath.Ext(path) != ".md" || info.Mode().Perm() != 0o600 || string(body) != "existing comment\n\n```suggestion\nnew()\n```\n" {
		t.Fatalf("path=%q mode=%o body=%q", path, info.Mode().Perm(), body)
	}
}

func TestCommentEditorSuggestionBehaviors(t *testing.T) {
	t.Run("escapes fence", func(t *testing.T) {
		anchor := comments.Anchor{QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"}}
		draft := commentDraft("explain this", anchor)
		if !strings.Contains(draft, "`````suggestion") || stripUnchangedSuggestion(draft, anchor.QuotedLines) != "explain this" {
			t.Fatalf("draft = %q", draft)
		}
	})
	t.Run("only new version", func(t *testing.T) {
		anchor := comments.Anchor{QuotedLines: []string{" unchanged()", "-old()", "+new()"}}
		if got := commentDraft("comment", anchor); got != "comment\n\n```suggestion\nunchanged()\nnew()\n```\n" {
			t.Fatalf("draft = %q", got)
		}
	})
	t.Run("edited suggestion remains", func(t *testing.T) {
		body := "comment\n\n```suggestion\nbetter()\n```\n"
		if got := stripUnchangedSuggestion(body, []string{"-old()", "+new()"}); got != body {
			t.Fatalf("body = %q", got)
		}
	})
}

func TestExternalEditorCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	if err := commentCommand(context.Background(), "printf 'edited externally' >", path).Run(); err != nil {
		t.Fatal(err)
	}
	body, err := readCommentFile(path, comments.Anchor{})
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestCommentDraftRoundTrip(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := commentDraft("body", anchor)
	if got := stripUnchangedSuggestion(draft, anchor.QuotedLines); got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}
