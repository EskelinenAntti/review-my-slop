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

func TestRunCommentsPrintsAndConsumesCurrentRepositoryFeedback(t *testing.T) {
	repo := initRepository(t)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	store, err := inbox.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(inbox.Message{
		Repository: repo,
		Comment: review.Comment{
			Anchor: review.Anchor{File: "main.go", NewStart: 3, NewEnd: 3},
			Body:   "Check this error.",
		},
	}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	if err := runCommentsAt(context.Background(), repo, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Check this error.") {
		t.Fatalf("unexpected output:\n%s", output.String())
	}
	if !strings.HasPrefix(output.String(), "New comments since last run:\n") {
		t.Fatalf("unexpected output heading:\n%s", output.String())
	}
	if strings.Contains(output.String(), "batch") {
		t.Fatalf("output exposes internal batches:\n%s", output.String())
	}

	var empty bytes.Buffer
	if err := runCommentsAt(context.Background(), repo, &empty); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(empty.String()) != "No pending review comments." {
		t.Fatalf("second output = %q", empty.String())
	}

	info, err := os.Stat(filepath.Join(data, "review-my-slop", "inbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("database mode = %o", info.Mode().Perm())
	}
}

func TestRunCommentsPreservesFeedbackWhenOutputFails(t *testing.T) {
	repo := initRepository(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store, err := inbox.OpenDefault()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(inbox.Message{
		Repository: repo,
		Comment: review.Comment{
			Anchor: review.Anchor{File: "main.go", NewStart: 1},
			Body:   "Preserve me.",
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := runCommentsAt(context.Background(), repo, failingWriter{}); err == nil {
		t.Fatal("output failure was ignored")
	}
	taken, err := store.Peek(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(taken.Messages) != 1 {
		t.Fatalf("pending messages = %d, want 1", len(taken.Messages))
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

func initRepository(t *testing.T) string {
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
