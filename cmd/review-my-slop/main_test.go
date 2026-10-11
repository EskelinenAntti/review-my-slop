package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRunCommentsPrintsAndConsumesCurrentRepositoryFeedback(t *testing.T) {
	repo := initRepository(t)
	nested := filepath.Join(repo, "nested")

	testutil.NoError(t, os.Mkdir(nested, 0o755))

	t.Chdir(nested)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	store, err := comments.OpenDefault()
	testutil.Require(t, err == nil, err)
	{
		_, err := store.Add(comments.Comment{
			Repository: repo,
			Anchor:     comments.Anchor{FilePath: "main.go", NewStart: 3, NewEnd: 3},
			Body:       "Check this error.",
		})
		testutil.Require(t, err == nil, err)
	}

	var output bytes.Buffer

	testutil.NoError(t, runComments(context.Background(), &output))

	testutil.Requiref(t, strings.Contains(output.String(), "Check this error."), "unexpected output:\n%s", output.String())
	testutil.Requiref(t, strings.HasPrefix(output.String(), "New comments since last run:\n"), "unexpected output heading:\n%s", output.String())
	testutil.Requiref(t, !strings.Contains(output.String(), "batch"), "output exposes internal batches:\n%s", output.String())

	var empty bytes.Buffer

	testutil.NoError(t, runComments(context.Background(), &empty))

	testutil.Requiref(t, strings.TrimSpace(empty.String()) == "No pending review comments.", "second output = %q", empty.String())

	info, err := os.Stat(filepath.Join(data, "review-my-slop", "comments.db"))
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, info.Mode().Perm() == 0o600, "database mode = %o", info.Mode().Perm())
}

func TestRunCommentsPreservesFeedbackWhenOutputFails(t *testing.T) {
	repo := initRepository(t)
	t.Chdir(repo)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store, err := comments.OpenDefault()
	testutil.Require(t, err == nil, err)
	{
		_, err := store.Add(comments.Comment{
			Repository: repo,
			Anchor:     comments.Anchor{FilePath: "main.go", NewStart: 1},
			Body:       "Preserve me.",
		})
		testutil.Require(t, err == nil, err)
	}

	{
		err := runComments(context.Background(), failingWriter{})
		testutil.Require(t, err != nil, "output failure was ignored")
	}
	comments, err := store.List(repo)
	testutil.Require(t, err == nil, err)
	testutil.Requiref(t, len(comments) == 1, "pending comments = %d, want 1", len(comments))
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"unknown"}, &bytes.Buffer{})
	testutil.Requiref(t, err != nil && strings.Contains(err.Error(), `unknown subcommand "unknown"`), "error = %v", err)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}

func initRepository(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	testutil.Require(t, err == nil, err)
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	{
		out, err := cmd.CombinedOutput()
		testutil.Requiref(t, err == nil, "git init: %v\n%s", err, out)
	}
	return repo
}
