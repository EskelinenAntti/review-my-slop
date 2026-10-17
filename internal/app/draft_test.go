package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	feedback "github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestEditorCommentOpensMarkdownFileInEditor(t *testing.T) {
	anchor := feedback.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := writeTestCommentDraftFile("existing comment", anchor)
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

func writeTestCommentDraftFile(
	body string,
	anchor feedback.Anchor,
) (string, error) {
	file, err := os.CreateTemp("", "review-my-slop-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("create comment file: %w", err)
	}
	path := file.Name()
	if _, err := file.WriteString(commentDraftText(body, anchor)); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write comment file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("close comment file: %w", err)
	}
	return path, nil
}

func TestEditorCommentEditorSuggestionBehaviors(t *testing.T) {
	t.Run("escapes fence", testEscapesSuggestionFence)
	t.Run("only new version", testSuggestionUsesNewVersion)
	t.Run("edited suggestion remains", testEditedSuggestionRemains)
	t.Run("editor trims final newline", testSuggestionWithoutFinalNewline)
}

func testEscapesSuggestionFence(t *testing.T) {
	t.Helper()
	anchor := feedback.Anchor{
		QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"},
	}
	draft := commentDraftText("explain this", anchor)
	if !strings.Contains(draft, "`````suggestion") ||
		stripCommentSuggestion(draft, anchor) != "explain this" {
		t.Fatalf("draft = %q", draft)
	}
}

func testSuggestionUsesNewVersion(t *testing.T) {
	t.Helper()
	anchor := feedback.Anchor{
		QuotedLines: []string{" unchanged()", "-old()", "+new()"},
	}
	want := "comment\n\n```suggestion\nunchanged()\nnew()\n```\n"
	if got := commentDraftText("comment", anchor); got != want {
		t.Fatalf("draft = %q", got)
	}
}

func testEditedSuggestionRemains(t *testing.T) {
	t.Helper()
	body := "comment\n\n```suggestion\nbetter()\n```\n"
	quoted := []string{"-old()", "+new()"}
	anchor := feedback.Anchor{QuotedLines: quoted}
	got := stripCommentSuggestion(body, anchor)
	if got != body {
		t.Fatalf("body = %q", got)
	}
}

func testSuggestionWithoutFinalNewline(t *testing.T) {
	t.Helper()
	anchor := feedback.Anchor{QuotedLines: []string{"-old()", "+new()"}}
	draft := strings.TrimSuffix(commentDraftText("comment", anchor), "\n")
	if got := stripCommentSuggestion(draft, anchor); got != "comment" {
		t.Fatalf("body = %q", got)
	}
}

func TestEditorExternalEditorCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	err = runCommentEditor(
		context.Background(),
		"printf 'edited externally' >",
		path,
		&editorCommandAdapter{},
	)
	if err != nil {
		t.Fatal(err)
	}
	body, err := readCommentDraft(path, feedback.Anchor{})
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestEditorCommentDraftRoundTrip(t *testing.T) {
	anchor := feedback.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := commentDraftText("body", anchor)
	got := stripCommentSuggestion(draft, anchor)
	if got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}
