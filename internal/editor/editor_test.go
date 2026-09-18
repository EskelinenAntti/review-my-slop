package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
)

func TestCommentDraftAndReadRemoveUntouchedSuggestion(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	anchor := comments.Anchor{QuotedLines: []string{"-old()```x", "+new()"}}
	path, err := CreateCommentFile("existing comment", anchor)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(path) != ".md" || info.Mode().Perm() != 0o600 {
		t.Fatalf("path=%q mode=%o", path, info.Mode().Perm())
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "existing comment\n\n```suggestion\nnew()\n```\n" {
		t.Fatalf("draft = %q", body)
	}
	got, err := ReadCommentFile(path, anchor, nil)
	if err != nil || got != "existing comment" {
		t.Fatalf("body=%q err=%v", got, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("draft was not removed: %v", err)
	}
}

func TestSuggestionBehaviors(t *testing.T) {
	tests := []struct {
		name   string
		anchor comments.Anchor
		body   string
		want   string
	}{
		{name: "new version", anchor: comments.Anchor{QuotedLines: []string{" unchanged()", "-old()", "+new()"}}, body: "comment", want: "comment\n\n```suggestion\nunchanged()\nnew()\n```\n"},
		{name: "no quoted lines", anchor: comments.Anchor{}, body: "comment", want: "comment"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CommentDraft(test.body, test.anchor); got != test.want {
				t.Fatalf("draft=%q want=%q", got, test.want)
			}
		})
	}
	body := "comment\n\n```suggestion\nbetter()\n```\n"
	if got := StripUnchangedSuggestion(body, []string{"-old()", "+new()"}); got != body {
		t.Fatalf("edited suggestion was stripped: %q", got)
	}
}

func TestExternalCommandsQuotePaths(t *testing.T) {
	path := "/tmp/repo with spaces/main.go"
	if got := strings.Join(SourceCommand("printf", path, 2).Args, "\x00"); got != "sh\x00-c\x00printf +2 '/tmp/repo with spaces/main.go'" {
		t.Fatalf("source command=%q", got)
	}
	if got := CommentCommand("printf", path).Args[2]; got != "printf '/tmp/repo with spaces/main.go'" {
		t.Fatalf("comment command=%q", got)
	}
}

func TestReadCommentFileReportsEditorFailureAndCleansDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte("draft"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCommentFile(path, comments.Anchor{}, os.ErrProcessDone); err == nil {
		t.Fatal("editor failure was ignored")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("failed draft was not removed: %v", err)
	}
}
