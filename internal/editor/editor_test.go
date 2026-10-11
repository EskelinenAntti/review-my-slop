package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := createCommentFile("existing comment", anchor)
	testutil.Require(t, err == nil, err)
	t.Cleanup(func() { _ = os.Remove(path) })
	info, err := os.Stat(path)
	testutil.Require(t, err == nil, err)
	body, err := os.ReadFile(path)
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, filepath.Ext(path) == ".md" && info.Mode().Perm() == 0o600 && string(body) == "existing comment\n\n```suggestion\nnew()\n```\n", "path=%q mode=%o body=%q", path, info.Mode().Perm(), body)
}

func TestCommentEditorSuggestionBehaviors(t *testing.T) {
	t.Run("escapes fence", func(t *testing.T) {
		anchor := comments.Anchor{QuotedLines: []string{"+````go", `+fmt.Println("hello")`, "+````"}}
		draft := commentDraft("explain this", anchor)
		testutil.Requiref(t, strings.Contains(draft, "`````suggestion") && stripUnchangedSuggestion(draft, anchor.QuotedLines) == "explain this", "draft = %q", draft)
	})
	t.Run("only new version", func(t *testing.T) {
		anchor := comments.Anchor{QuotedLines: []string{" unchanged()", "-old()", "+new()"}}
		{
			got := commentDraft("comment", anchor)
			testutil.Requiref(t, got == "comment\n\n```suggestion\nunchanged()\nnew()\n```\n", "draft = %q", got)
		}
	})
	t.Run("edited suggestion remains", func(t *testing.T) {
		body := "comment\n\n```suggestion\nbetter()\n```\n"
		{
			got := stripUnchangedSuggestion(body, []string{"-old()", "+new()"})
			testutil.Requiref(t, got == body, "body = %q", got)
		}
	})
}

func TestExternalEditorCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	testutil.Require(t, err == nil, err)
	path := file.Name()

	testutil.NoError(t, file.Close())

	t.Cleanup(func() { _ = os.Remove(path) })

	testutil.NoError(t, commentCommand(context.Background(), "printf 'edited externally' >", path).Run())

	body, err := readCommentFile(path, comments.Anchor{})
	testutil.Requiref(t, err == nil && body == "edited externally", "body = %q, err = %v", body, err)
}

func TestCommentDraftRoundTrip(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := commentDraft("body", anchor)
	{
		got := stripUnchangedSuggestion(draft, anchor.QuotedLines)
		testutil.Requiref(t, got == "body", "unchanged suggestion result = %q", got)
	}
}
