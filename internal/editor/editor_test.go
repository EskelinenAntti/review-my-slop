package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	quoted := []string{"-old()```x", "+new()"}
	path, err := createCommentFile("existing comment", quoted)
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
		quoted := []string{"+````go", `+fmt.Println("hello")`, "+````"}
		draft := commentDraft("explain this", quoted)
		if !strings.Contains(
			draft,
			"`````suggestion",
		) || stripUnchangedSuggestion(
			draft,
			quoted,
		) != "explain this" {
			t.Fatalf("draft = %q", draft)
		}
	})
	t.Run("only new version", func(t *testing.T) {
		quoted := []string{" unchanged()", "-old()", "+new()"}
		want := "comment\n\n```suggestion\nunchanged()\nnew()\n```\n"
		if got := commentDraft("comment", quoted); got != want {
			t.Fatalf("draft = %q", got)
		}
	})
	t.Run("edited suggestion remains", func(t *testing.T) {
		body := "comment\n\n```suggestion\nbetter()\n```\n"
		quoted := []string{"-old()", "+new()"}
		if got := stripUnchangedSuggestion(body, quoted); got != body {
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
	command := commentCommand(
		context.Background(), "printf 'edited externally' >", path,
	)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	body, err := readCommentFile(path, nil)
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestCommentDraftRoundTrip(t *testing.T) {
	quoted := []string{" old", "-gone", "+new"}
	draft := commentDraft("body", quoted)
	got := stripUnchangedSuggestion(draft, quoted)
	if got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}
