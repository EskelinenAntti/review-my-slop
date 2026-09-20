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
	defaultBranch, err := loader.DefaultBranch(ctx, current)
	if err != nil {
		return err
	}
	size := initialTerminalSize()
	model, err := ui.NewWithActions(ui.Actions{
		SaveComment: func(comment comments.Comment, current patch.Patch) (comments.Comment, error) {
			comment.Repository = current.Repository
			if comment.ID != "" {
				if err := store.Update(comment); err != nil {
					return comments.Comment{}, err
				}
				return comment, nil
			}
			return store.Add(comment)
		},
		DeleteComment: func(comment comments.Comment, current patch.Patch) error {
			return store.Delete(current.Repository, comment.ID)
		},
		LoadComments: func(current patch.Patch) ([]comments.Comment, error) {
			return store.List(current.Repository)
		},
		RefreshPatch: func(branch string) (patch.Patch, error) {
			if branch == "" {
				return loader.Load(ctx, current)
			}
			return loader.LoadBranch(ctx, current, branch)
		},
	}, loaded, pending, size)
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
	root, err := patch.Loader{}.Root(ctx, current)
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
