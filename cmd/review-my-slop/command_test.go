package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainRunCommentsPrintsAndConsumesCurrentRepositoryFeedback(
	t *testing.T,
) {
	repo := createInitializedGitTestRepository(t)
	nested := filepath.Join(repo, "nested")
	requireNoError(t, os.Mkdir(nested, 0o755))

	t.Chdir(nested)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	store, err := commentsStoreFromTestEnvironment()
	requireNoError(t, err)

	comment := commentsComment{
		Repository: repo,
		Anchor:     commentsAnchor{FilePath: "main.go", NewStart: 3, NewEnd: 3},
		Body:       "Check this error.",
	}
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	requireNoError(t, runCommand(
		context.Background(), []string{"comments"}, &output,
	))
	requireEqual(t,
		strings.Contains(output.String(), "Check this error."), true,
		"unexpected output:\n%s", output.String(),
	)
	requireEqual(t,
		strings.HasPrefix(
			output.String(),
			"New comments since last run:\n",
		), true,
		"unexpected output heading:\n%s", output.String(),
	)
	requireEqual(t, !strings.Contains(output.String(), "batch"), true,
		"output exposes internal batches:\n%s", output.String(),
	)

	var empty bytes.Buffer
	requireNoError(t, runCommand(
		context.Background(), []string{"comments"}, &empty,
	))
	requireEqual(
		t, strings.TrimSpace(empty.String()), "No pending review comments.",
		"second output = %q", empty.String())

	info, err := os.Stat(filepath.Join(data, "review-my-slop", "comments.db"))
	requireNoError(t, err)
	requireEqual(t, info.Mode().Perm(), 0o600, "database mode = %o",
		info.Mode().Perm(),
	)
}

func TestMainRunCommentsPreservesFeedbackWhenOutputFails(t *testing.T) {
	repo := createInitializedGitTestRepository(t)
	t.Chdir(repo)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store, err := commentsStoreFromTestEnvironment()
	requireNoError(t, err)

	comment := commentsComment{
		Repository: repo,
		Anchor:     commentsAnchor{FilePath: "main.go", NewStart: 1},
		Body:       "Preserve me.",
	}
	if _, err := store.Add(comment); err != nil {
		t.Fatal(err)
	}

	err = runCommand(
		context.Background(), []string{"comments"}, mainFailingWriter{},
	)
	if err == nil {
		t.Fatal("output failure was ignored")
	}
	pending, err := store.List(repo)
	requireNoError(t, err)
	requireEqual(t, len(pending), 1, "pending comments = %d, want 1",
		len(pending),
	)
}

func TestMainRunRejectsUnknownSubcommand(t *testing.T) {
	err := runCommand(
		context.Background(), []string{"unknown"}, &bytes.Buffer{},
	)
	requireEqual(t, err != nil, true, "error = %v", err)
	requireEqual(t,
		strings.Contains(err.Error(), `unknown subcommand "unknown"`), true,
		"error = %v", err,
	)
}

type mainFailingWriter struct{}

func (mainFailingWriter) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}
