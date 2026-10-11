package editor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestCommentOpensMarkdownFileInEditor(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := createCommentFile("existing comment", anchor)
	requireEqual(t, err == nil, true, fixtureV, err)
	t.Cleanup(func() { _ = os.Remove(path) })
	info, err := os.Stat(path)
	requireEqual(t, err == nil, true, fixtureV, err)
	body, err := os.ReadFile(path)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		filepath.Ext(path) == ".md" && info.Mode().Perm() == 0o600 &&
			string(body) == "existing comment\n\n```suggestion\nnew()\n```\n",
		true,
		"path=%q mode=%o body=%q",
		path,
		info.Mode().Perm(),
		body,
	)
}

func TestCommentEditorSuggestionBehaviors(t *testing.T) {
	t.Run(
		"escapes fence",
		func(t *testing.T) {
			anchor := comments.Anchor{
				QuotedLines: []string{
					"+````go",
					`+fmt.Println("hello")`,
					"+````",
				},
			}
			draft := commentDraft("explain this", anchor)
			requireEqual(
				t,
				strings.Contains(draft, "`````suggestion") &&
					stripUnchangedSuggestion(
						draft,
						anchor.QuotedLines,
					) == "explain this",
				true,
				"draft = %q",
				draft,
			)
		},
	)
	t.Run(
		"only new version",
		func(t *testing.T) {
			anchor := comments.Anchor{
				QuotedLines: []string{" unchanged()", "-old()", "+new()"},
			}
			{
				got := commentDraft("comment", anchor)
				requireEqual(
					t,
					got,
					"comment\n\n```suggestion\nunchanged()\nnew()\n```\n",
					"draft = %q",
					got,
				)
			}
		},
	)
	t.Run(
		"edited suggestion remains",
		func(t *testing.T) {
			body := "comment\n\n```suggestion\nbetter()\n```\n"
			{
				got := stripUnchangedSuggestion(
					body,
					[]string{"-old()", "+new()"},
				)
				requireEqual(t, got, body, "body = %q", got)
			}
		},
	)
}

func TestExternalEditorCommandReadsEditedDraft(t *testing.T) {
	file, err := os.CreateTemp("", "review-my-slop-editor-test-*.md")
	requireEqual(t, err == nil, true, fixtureV, err)
	path := file.Name()
	{
		err := file.Close()
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	{
		err := commentCommand(
			context.Background(),
			"printf 'edited externally' >",
			path,
		).Run()
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	body, err := readCommentFile(path, comments.Anchor{})
	requireEqual(
		t,
		err == nil && body == "edited externally", true,
		"body = %q, err = %v",
		body,
		err,
	)
}

func TestCommentDraftRoundTrip(t *testing.T) {
	anchor := comments.Anchor{QuotedLines: []string{" old", "-gone", "+new"}}
	draft := commentDraft("body", anchor)
	{
		got := stripUnchangedSuggestion(draft, anchor.QuotedLines)
		requireEqual(t, got, "body", "unchanged suggestion result = %q", got)
	}
}
