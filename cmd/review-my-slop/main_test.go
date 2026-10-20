package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
)

func TestRunCodeChecksStorageBeforeLoadingPatch(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runCode(ctx)
	if err == nil ||
		!strings.Contains(err.Error(), "resolve user home directory") {
		t.Fatalf("error = %v, want storage configuration error", err)
	}
}

func TestRunCodePropagatesPatchLoadError(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := runCode(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
}

func TestRunCommentsPrintsAndConsumesCurrentRepositoryFeedback(t *testing.T) {
	repo := initRepository(t)
	nested := filepath.Join(repo, "nested")
	testutil.NoError(t, os.Mkdir(nested, 0o755))

	t.Chdir(nested)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	store, err := comments.OpenDefault()
	testutil.NoError(t, err)

	comment := comments.Comment{
		Repository: repo,
		Anchor: comments.Anchor{
			FilePath: "main.go",
			NewStart: 3,
			NewEnd:   3,
		},
		Body: "Check this error.",
	}
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	testutil.NoError(t, runComments(context.Background(), &output))
	testutil.RequireEqual(t,
		strings.Contains(output.String(), "Check this error."), true,
		"unexpected output:\n%s", output.String(),
	)
	testutil.RequireEqual(t,
		strings.HasPrefix(
			output.String(),
			"New comments since last run:\n",
		), true,
		"unexpected output heading:\n%s", output.String(),
	)
	testutil.RequireEqual(t, !strings.Contains(output.String(), "batch"), true,
		"output exposes internal batches:\n%s", output.String(),
	)

	var empty bytes.Buffer
	testutil.NoError(t, runComments(context.Background(), &empty))
	testutil.RequireEqual(
		t, strings.TrimSpace(empty.String()), "No pending review comments.",
		"second output = %q", empty.String())

	info, err := os.Stat(filepath.Join(data, "review-my-slop", "comments.db"))
	testutil.NoError(t, err)
	testutil.RequireEqual(t, info.Mode().Perm(), 0o600, "database mode = %o",
		info.Mode().Perm(),
	)
}

func TestRunCommentsPreservesFeedbackWhenOutputFails(t *testing.T) {
	repo := initRepository(t)
	t.Chdir(repo)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store, err := comments.OpenDefault()
	testutil.NoError(t, err)

	comment := comments.Comment{
		Repository: repo,
		Anchor:     comments.Anchor{FilePath: "main.go", NewStart: 1},
		Body:       "Preserve me.",
	}
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}

	if err := runComments(context.Background(), failingWriter{}); err == nil {
		t.Fatal("output failure was ignored")
	}
	pending, err := store.List(repo)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(pending), 1, "pending comments = %d, want 1",
		len(pending),
	)
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"unknown"}, &bytes.Buffer{})
	testutil.RequireEqual(t, err != nil, true, "error = %v", err)
	testutil.RequireEqual(t,
		strings.Contains(err.Error(), `unknown subcommand "unknown"`), true,
		"error = %v", err,
	)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}

func initRepository(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	testutil.NoError(t, err)

	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	return repo
}
