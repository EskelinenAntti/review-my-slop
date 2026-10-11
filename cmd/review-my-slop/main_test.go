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
)

func TestRunCommentsPrintsAndConsumesCurrentRepositoryFeedback(t *testing.T) {
	repo := initRepository(t)
	nested := filepath.Join(repo, "nested")
	{
		err := os.Mkdir(nested, executableDirMode)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	t.Chdir(nested)
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	store, err := comments.OpenDefault()
	requireEqual(t, err == nil, true, fixtureV, err)
	addPendingFeedback(t, store, repo)

	var output bytes.Buffer
	{
		err := runComments(context.Background(), &output)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	requireEqual(
		t,
		strings.Contains(output.String(), "Check this error."), true,
		"unexpected output:\n%s",
		output.String(),
	)
	requireEqual(
		t,
		strings.HasPrefix(output.String(), "New comments since last run:\n"),
		true,
		"unexpected output heading:\n%s",
		output.String(),
	)
	requireEqual(
		t,
		!strings.Contains(output.String(), "batch"), true,
		"output exposes internal batches:\n%s",
		output.String(),
	)

	var empty bytes.Buffer
	{
		err := runComments(context.Background(), &empty)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
	requireEqual(
		t,
		strings.TrimSpace(empty.String()), "No pending review comments.",
		"second output = %q",
		empty.String(),
	)

	info, err := os.Stat(filepath.Join(data, "review-my-slop", "comments.db"))
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		info.Mode().Perm(), privateFileMode,
		"database mode = %o",
		info.Mode().Perm(),
	)
}

func TestRunCommentsPreservesFeedbackWhenOutputFails(t *testing.T) {
	repo := initRepository(t)
	t.Chdir(repo)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	store, err := comments.OpenDefault()
	requireEqual(t, err == nil, true, fixtureV, err)
	{
		_, err := store.Add(comments.Comment{
			Repository: repo,
			Anchor:     comments.Anchor{FilePath: "main.go", NewStart: one},
			Body:       "Preserve me.",
		})
		requireEqual(t, err == nil, true, fixtureV, err)
	}

	{
		err := runComments(context.Background(), failingWriter{})
		requireEqual(
			t,
			err != nil,
			true,
			fixtureV,
			"output failure was ignored",
		)
	}
	items, err := store.List(repo)
	requireEqual(t, err == nil, true, fixtureV, err)
	requireEqual(
		t,
		len(items), one,
		"pending comments = %d, want 1",
		len(items),
	)
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"unknown"}, &bytes.Buffer{})
	requireEqual(
		t,
		err != nil &&
			strings.Contains(
				err.Error(),
				`unknown subcommand "unknown"`,
			),
		true,
		"error = %v",
		err,
	)
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return zero, os.ErrClosed
}

func initRepository(t *testing.T) string {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	requireEqual(t, err == nil, true, fixtureV, err)
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = repo
	{
		out, err := cmd.CombinedOutput()
		requireEqual(t, err == nil, true, "git init: %v\n%s", err, out)
	}
	return repo
}

func addPendingFeedback(t *testing.T, store comments.Store, repo string) {
	{
		_, err := store.Add(
			comments.Comment{
				Repository: repo,
				Anchor: comments.Anchor{
					FilePath: "main.go",
					NewStart: 3,
					NewEnd:   3,
				},
				Body: "Check this error.",
			},
		)
		requireEqual(t, err == nil, true, fixtureV, err)
	}
}
