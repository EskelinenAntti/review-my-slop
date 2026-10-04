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
	commentStore, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentPatch, err := patch.Get(ctx, patch.Unstaged)
	if err != nil {
		return err
	}
	pending, err := commentStore.List(currentPatch.Root)
	if err != nil {
		return err
	}
	size := initialTerminalSize()
	model, err := ui.NewWithStore(commentStore, currentPatch, pending, size)
	if err != nil {
		return err
	}
	model.SetRefresh(func(kind patch.Kind) (patch.Patch, error) {
		return patch.Get(ctx, kind)
	})
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
	commentStore, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	currentPatch, err := patch.Get(ctx, patch.Unstaged)
	if err != nil {
		return err
	}
	return commentStore.WritePending(output, currentPatch.Root)
}
