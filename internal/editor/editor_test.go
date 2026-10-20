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
		draft := commentDraft("explain this", anchor)
		if !strings.Contains(
			draft,
			"`````suggestion",
		) || stripUnchangedSuggestion(
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
		if got := commentDraft("comment", anchor); got != want {
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
	command := shellCommand(
		context.Background(), "printf 'edited externally' >", path,
	)
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	body, err := readCommentFile(path, comments.Anchor{})
	if err != nil || body != "edited externally" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

func TestCommentDraftRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name       string
		quoted     []string
		suggestion string
	}{
		{name: "no quote"},
		{
			name: "mixed lines", quoted: []string{" old", "-gone", "+new"},
			suggestion: "```suggestion\nold\nnew\n```",
		},
		{
			name: "deletions only", quoted: []string{"-gone"},
			suggestion: "```suggestion\n```",
		},
		{
			name: "empty quoted line", quoted: []string{""},
			suggestion: "```suggestion\n```",
		},
		{
			name: "blank source line", quoted: []string{"+"},
			suggestion: "```suggestion\n\n```",
		},
		{
			name: "backticks", quoted: []string{"+````", "-``````"},
			suggestion: "`````suggestion\n````\n`````",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertCommentDraftRoundTrip(t, tc.quoted, tc.suggestion)
		})
	}
}

func assertCommentDraftRoundTrip(
	t *testing.T,
	quoted []string,
	suggestion string,
) {
	t.Helper()
	anchor := comments.Anchor{QuotedLines: quoted}
	want := "body"
	if suggestion != "" {
		want += "\n\n" + suggestion + "\n"
	}
	draft := commentDraft("body", anchor)
	if draft != want {
		t.Fatalf("draft = %q, want %q", draft, want)
	}
	if got := stripUnchangedSuggestion(draft, quoted); got != "body" {
		t.Fatalf("unchanged suggestion result = %q", got)
	}
}
