package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/inbox"
	"github.com/eskelinenantti/review-my-slop/internal/review"
)

func TestRunCommentsPrintsAndAcknowledgesPendingComments(t *testing.T) {
	repo := newEmptyRepository(t)
	newStoreWithComment(t, repo, review.Comment{
		Repository: repo,
		Anchor:     review.Anchor{FilePath: "main.go", NewStart: 3, NewEnd: 3},
		Body:       "Check this error.",
	})

	var output bytes.Buffer
	if err := runCommentsAt(context.Background(), repo, &output); err != nil {
		t.Fatal(err)
	}
	prompt := output.String()
	want := "New comments since last run:\n\n### 1. `main.go` (new line 3)\n\nCheck this error.\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}

	var empty bytes.Buffer
	if err := runCommentsAt(context.Background(), repo, &empty); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(empty.String()) != "No pending review comments." {
		t.Fatalf("second output = %q", empty.String())
	}
}

func TestRunCommentsPreservesFeedbackWhenOutputFails(t *testing.T) {
	repo := newEmptyRepository(t)
	store := newStoreWithComment(t, repo, review.Comment{
		Repository: repo,
		Anchor:     review.Anchor{FilePath: "main.go", NewStart: 1},
		Body:       "Preserve me.",
	})

	if err := runCommentsWithStore(store, repo, failingWriter{}); err == nil {
		t.Fatal("output failure was ignored")
	}
	comments, err := store.List(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("pending comments = %d, want 1", len(comments))
	}
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"unknown"}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), `unknown subcommand "unknown"`) {
		t.Fatalf("error = %v", err)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}

func newStoreWithComment(t *testing.T, repository string, comment review.Comment) inbox.Store {
	t.Helper()
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store, err := inbox.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	comment.Repository = repository
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}
	return store
}

func newEmptyRepository(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return repo
}
