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
		current, err := os.Getwd()
		if err != nil {
			return err
		}
		return runCommentsAt(ctx, current, output)
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
	loader := patch.Loader{}
	loaded, err := loader.Load(ctx, current)
	if err != nil {
		return err
	}
	pending, err := store.List(loaded.Repository)
	if err != nil {
		return err
	}
	defaultBranch, err := (&patch.Loader{}).DefaultBranch(ctx, current)
	if err != nil {
		return err
	}
	size := ui.DefaultSize
	for _, fd := range []uintptr{os.Stdin.Fd(), os.Stdout.Fd()} {
		if width, height, err := term.GetSize(fd); err == nil {
			size = ui.Size{Width: width, Height: height}
			break
		}
	}
	model, err := ui.NewWithReview(loader, store, ctx, current, loaded, pending, size, defaultBranch)
	if err != nil {
		return err
	}
	program := tea.NewProgram(model, tea.WithWindowSize(size.Width, size.Height))
	_, err = program.Run()
	return err
}

func runCommentsAt(ctx context.Context, current string, output io.Writer) error {
	store, err := comments.OpenDefault()
	if err != nil {
		return err
	}
	root, err := (&patch.Loader{}).Root(ctx, current)
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
	for index, comment := range pending {
		ids[index] = comment.ID
	}
	return store.Acknowledge(root, ids)
}
