package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/eskelinenantti/review-my-slop/internal/app"
	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/testutil"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCommentsPrintsAndConsumesCurrentRepositoryFeedback(t *testing.T) {
	repo := (func(t *testing.T) string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)

		cmd := exec.Command("git", "init", "-q")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		return repo
	})(t)
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
	testutil.NoError(t, (func(ctx context.Context, output io.Writer) error {
		commentStore, err := comments.OpenDefault()
		if err != nil {
			return err
		}
		currentPatch, err := patch.Get(ctx, patch.Unstaged)
		if err != nil {
			return err
		}
		return commentStore.WritePending(output, currentPatch.Root)
	})(context.Background(), &output))
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
	testutil.NoError(t, (func(ctx context.Context, output io.Writer) error {
		commentStore, err := comments.OpenDefault()
		if err != nil {
			return err
		}
		currentPatch, err := patch.Get(ctx, patch.Unstaged)
		if err != nil {
			return err
		}
		return commentStore.WritePending(output, currentPatch.Root)
	})(context.Background(), &empty))
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
	repo := (func(t *testing.T) string {
		t.Helper()
		repo, err := filepath.EvalSymlinks(t.TempDir())
		testutil.NoError(t, err)

		cmd := exec.Command("git", "init", "-q")
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		return repo
	})(t)
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

	if err := (func(ctx context.Context, output io.Writer) error {
		commentStore, err := comments.OpenDefault()
		if err != nil {
			return err
		}
		currentPatch, err := patch.Get(ctx, patch.Unstaged)
		if err != nil {
			return err
		}
		return commentStore.WritePending(output, currentPatch.Root)
	})(context.Background(), failingWriter{}); err == nil {
		t.Fatal("output failure was ignored")
	}
	pending, err := store.List(repo)
	testutil.NoError(t, err)
	testutil.RequireEqual(t, len(pending), 1, "pending comments = %d, want 1",
		len(pending),
	)
}

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := (func(ctx context.Context, args []string, output io.Writer) error {
		if len(args) == 0 {
			return (func(ctx context.Context) error {
				commentStore, err := comments.OpenDefault()
				if err != nil {
					return err
				}
				currentPatch, err := patch.Get(ctx, patch.Unstaged)
				if err != nil {
					return err
				}
				return app.Run(ctx, currentPatch, commentStore, patch.Get)
			})(ctx)
		}
		if len(args) > 1 {
			return errors.New("usage: review-my-slop [code|comments]")
		}
		switch args[0] {
		case "code":
			return (func(ctx context.Context) error {
				commentStore, err := comments.OpenDefault()
				if err != nil {
					return err
				}
				currentPatch, err := patch.Get(ctx, patch.Unstaged)
				if err != nil {
					return err
				}
				return app.Run(ctx, currentPatch, commentStore, patch.Get)
			})(ctx)
		case "comments":
			return (func(ctx context.Context, output io.Writer) error {
				commentStore, err := comments.OpenDefault()
				if err != nil {
					return err
				}
				currentPatch, err := patch.Get(ctx, patch.Unstaged)
				if err != nil {
					return err
				}
				return commentStore.WritePending(output, currentPatch.Root)
			})(ctx, output)
		default:
			return fmt.Errorf(
				"unknown subcommand %q; usage: review-my-slop [code|comments]",
				args[0],
			)
		}
	})(context.Background(), []string{"unknown"}, &bytes.Buffer{})
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
