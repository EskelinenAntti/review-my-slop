package main

import (
	"context"
	"fmt"
	"io"
	"os"

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
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentReview := review.New(ctx, current, patch.Loader{}, store)
	loaded, err := currentReview.Load("")
	if err != nil {
		return err
	}
	pending, err := currentReview.Comments(loaded)
	if err != nil {
		return err
	}
	defaultBranch, err := currentReview.DefaultBranch()
	if err != nil {
		return err
	}
	size := initialTerminalSize()
	model, err := ui.NewWithReview(currentReview, loaded, pending, size)
	if err != nil {
		return err
	}
	model.SetDefaultBranch(defaultBranch)
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
	current, err := os.Getwd()
	if err != nil {
		return err
	}
	return runCommentsAt(ctx, current, output)
}

func runCommentsAt(ctx context.Context, current string, output io.Writer) error {
	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentReview := review.New(ctx, current, patch.Loader{}, store)
	root, err := currentReview.Repository()
	if err != nil {
		return err
	}
	pending, err := currentReview.ExportComments(output, patch.Patch{Repository: root})
	if err != nil {
		return err
	}
	return currentReview.AcknowledgeRepository(root, pending)
}
