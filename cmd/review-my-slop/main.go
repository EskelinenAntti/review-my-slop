package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/eskelinenantti/review-my-slop/internal/comments"
	"github.com/eskelinenantti/review-my-slop/internal/patch"
	"github.com/eskelinenantti/review-my-slop/internal/review"
	"github.com/eskelinenantti/review-my-slop/internal/ui"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "review-my-slop:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return runCode(ctx)
	}
	if len(args) > 1 {
		return fmt.Errorf("usage: review-my-slop [code|comments]")
	}
	switch args[0] {
	case "code":
		return runCode(ctx)
	case "comments":
		return runComments(ctx, output)
	default:
		return fmt.Errorf("unknown subcommand %q; usage: review-my-slop [code|comments]", args[0])
	}
}

func runCode(ctx context.Context) error {
	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentReview := review.New(ctx, patch.Get, store)
	loaded, err := currentReview.Load(patch.Unstaged)
	if err != nil {
		return err
	}
	pending, err := currentReview.Comments(loaded)
	if err != nil {
		return err
	}
	size := initialTerminalSize()
	model, err := ui.NewWithReview(currentReview, loaded, pending, size)
	if err != nil {
		return err
	}
	program := tea.NewProgram(model, tea.WithWindowSize(size.Width, size.Height))
	_, err = program.Run()
	return err
}

func initialTerminalSize() ui.Size {
	if width, height, err := term.GetSize(os.Stdin.Fd()); err == nil {
		return ui.Size{Width: width, Height: height}
	}
	if width, height, err := term.GetSize(os.Stdout.Fd()); err == nil {
		return ui.Size{Width: width, Height: height}
	}
	return ui.DefaultSize
}

func runComments(ctx context.Context, output io.Writer) error {
	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	root, err := repositoryRoot(ctx)
	if err != nil {
		return err
	}
	pending, err := store.List(root)
	if err != nil {
		return err
	}
	if err := comments.WritePrompt(output, pending); err != nil {
		return err
	}
	ids := make([]string, len(pending))
	for i, comment := range pending {
		ids[i] = comment.ID
	}
	return store.Acknowledge(root, ids)
}

func repositoryRoot(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if ctx.Err() != nil {
		return "", fmt.Errorf("resolve repository root: %w", ctx.Err())
	}
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			return "", fmt.Errorf("resolve repository root: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	root, err := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	return root, nil
}
